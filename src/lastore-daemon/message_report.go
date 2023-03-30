// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/base64"
	"encoding/json"
	"internal/system"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/linuxdeepin/go-lib/strv"
)

type messageReportManager struct {
	config                            *Config
	userAgents                        *userAgentMap
	allowPostSystemUpgradeMessageType system.UpdateType
}

func newMessageReportManager(c *Config, agents *userAgentMap) *messageReportManager {
	return &messageReportManager{
		config:                            c,
		userAgents:                        agents,
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
	}
}

type upgradePostContent struct {
	SerialNumber    string   `json:"serialNumber"`
	MachineID       string   `json:"machineId"`
	UpgradeStatus   int      `json:"status"`
	UpgradeErrorMsg string   `json:"msg"`
	TimeStamp       int64    `json:"timestamp"`
	SourceUrl       []string `json:"sourceUrl"`
	Version         string   `json:"version"`
}

const (
	upgradeSucceed = 0
	upgradeFailed  = 1
)

func (m *messageReportManager) needPostSystemUpgradeMessage(mode system.UpdateType) bool {
	return strv.Strv(m.config.AllowPostSystemUpgradeMessageVersion).Contains(getEditionName()) && ((mode & m.allowPostSystemUpgradeMessageType) != 0)
}

// 发送系统更新成功或失败的状态
func (m *messageReportManager) postSystemUpgradeMessage(upgradeStatus int, j *Job, updateType system.UpdateType) {
	if !m.needPostSystemUpgradeMessage(updateType) {
		return
	}
	updateType &= m.allowPostSystemUpgradeMessageType
	var upgradeErrorMsg string
	var version string
	if upgradeStatus == upgradeFailed && j != nil {
		upgradeErrorMsg = j.Description
	}
	infoMap, err := getOSVersionInfo()
	if err != nil {
		logger.Warning(err)
	} else {
		version = strings.Join(
			[]string{infoMap["MajorVersion"], infoMap["MinorVersion"], infoMap["OsBuild"]}, ".")
	}

	sn, err := getSN()
	if err != nil {
		logger.Warning(err)
	}
	hardwareId, err := getHardwareId()
	if err != nil {
		logger.Warning(err)
	}

	sourceFilePath := system.GetCategorySourceMap()[updateType]
	postContent := &upgradePostContent{
		SerialNumber:    sn,
		MachineID:       hardwareId,
		UpgradeStatus:   upgradeStatus,
		UpgradeErrorMsg: upgradeErrorMsg,
		TimeStamp:       time.Now().Unix(),
		SourceUrl:       getUpgradeUrls(sourceFilePath),
		Version:         version,
	}
	content, err := json.Marshal(postContent)
	if err != nil {
		logger.Warning(err)
		return
	}
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	logger.Debug(postContent)
	encryptMsg, err := EncryptMsg(content)
	if err != nil {
		logger.Warning(err)
		return
	}
	base64EncodeString := base64.StdEncoding.EncodeToString(encryptMsg)
	const url = "https://update-platform.uniontech.com/api/v1/update/status"
	request, err := http.NewRequest("POST", url, strings.NewReader(base64EncodeString))
	if err != nil {
		logger.Warning(err)
		return
	}
	response, err := client.Do(request)
	if err == nil {
		defer func() {
			_ = response.Body.Close()
		}()
		body, _ := ioutil.ReadAll(response.Body)
		logger.Info(string(body))
	} else {
		logger.Warning(err)
	}
}

type reportCategory uint32

const (
	updateStatus reportCategory = iota
	downloadStatus
	upgradeStatus
)

type reportLogInfo struct {
	Tid    int
	Result bool
	Reason string
}

func (m *messageReportManager) reportLog(category reportCategory, status bool, description string) {
	go func() {
		agent := m.userAgents.getActiveLastoreAgent()
		if agent != nil {
			logInfo := reportLogInfo{
				Result: status,
				Reason: description,
			}
			switch category {
			case updateStatus:
				logInfo.Tid = 1000600002
			case downloadStatus:
				logInfo.Tid = 1000600003
			case upgradeStatus:
				logInfo.Tid = 1000600004
			}
			infoContent, err := json.Marshal(logInfo)
			if err != nil {
				logger.Warning(err)
			}
			err = agent.ReportLog(0, string(infoContent))
			if err != nil {
				logger.Warning(err)
			}
		}
	}()
}
