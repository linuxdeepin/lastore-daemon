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
	"os"
	"strings"
	"time"

	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
)

type messageReportManager struct {
	config                            *Config
	userAgents                        *userAgentMap
	allowPostSystemUpgradeMessageType system.UpdateType
	preBuild                          string // 进行系统更新前的版本号，如20.1060.11018.100.100
	targetVersion                     string // 更新到的目标版本号，如1062
	preBaseline                       string // 更新前的基线号，从baseline获取
	targetBaseline                    string // 更新到的目标基线号,从baseline获取
	checkTime                         string // 基线检查时间
	systemTypeFromPlatform            string // 从更新平台获取的系统类型
	requestUrl                        string
}

func newMessageReportManager(c *Config, agents *userAgentMap) *messageReportManager {
	var preBuild string
	infoMap, err := getOSVersionInfo()
	if err != nil {
		logger.Warning(err)
	} else {
		preBuild = strings.Join(
			[]string{infoMap["MajorVersion"], infoMap["MinorVersion"], infoMap["OsBuild"]}, ".")
	}
	url := os.Getenv("UPDATE_PLATFORM_URL")
	if len(url) == 0 {
		url = "https://update-platform.uniontech.com"
	}
	return &messageReportManager{
		config:                            c,
		userAgents:                        agents,
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
		preBuild:                          preBuild,
		preBaseline:                       getBaseline(),
		requestUrl:                        url,
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

	PreBuild        string `json:"preBuild"`
	NextShowVersion string `json:"nextShowVersion"`
	PreBaseline     string `json:"preBaseline"`
	NextBaseline    string `json:"nextBaseline"`
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
	// updateType &= m.allowPostSystemUpgradeMessageType
	var upgradeErrorMsg string
	if upgradeStatus == upgradeFailed && j != nil {
		upgradeErrorMsg = j.Description
	}
	hardwareId, err := getHardwareId()
	if err != nil {
		logger.Warning(err)
	}

	postContent := &upgradePostContent{
		MachineID:       hardwareId,
		UpgradeStatus:   upgradeStatus,
		UpgradeErrorMsg: upgradeErrorMsg,
		TimeStamp:       time.Now().Unix(),
		PreBuild:        m.preBuild,
		NextShowVersion: m.targetVersion,
		PreBaseline:     m.preBaseline,
		NextBaseline:    m.targetBaseline,
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
	url := m.requestUrl + "/api/v1/update/status"
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

type version struct {
	Version  string `json:"version"`
	Baseline string `json:"baseline"`
}

type policy struct {
	Tp int `json:"tp"`

	Data struct {
	} `json:"data"`
}

type updateMessage struct {
	SystemType string  `json:"systemType"`
	Version    version `json:"version"`
	Policy     policy  `json:"policy"`
}

type tokenMessage struct {
	Result bool          `json:"result"`
	Code   int           `json:"code"`
	Data   updateMessage `json:"data"`
}
type tokenErrorMessage struct {
	Result bool   `json:"result"`
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

// 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *messageReportManager) genUpdatePolicyByToken() bool {
	policyUrl := m.requestUrl + "/api/v1/version"
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	if logger.GetLogLevel() == log.LevelDebug {
		logger.Info("debug mode,default target is 1062")
		time.Sleep(4 * time.Second)
		m.targetBaseline = "106x_debug"
		m.targetVersion = "1062"
		m.systemTypeFromPlatform = "professional"
		m.checkTime = time.Now().String()
		return true
	}
	request, err := http.NewRequest("GET", policyUrl, nil)
	if err != nil {
		logger.Warning(err)
		return false
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateTokenConfigFile())))
	response, err := client.Do(request)
	if err == nil {
		defer func() {
			_ = response.Body.Close()
		}()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			logger.Warning(err)
			return false
		}
		if response.StatusCode == 200 {
			logger.Debug(string(body))
			msg := &tokenMessage{}
			err = json.Unmarshal(body, msg)
			if err != nil {
				logger.Warning(err)
				return false
			}
			if !msg.Result {
				errorMsg := &tokenErrorMessage{}
				err = json.Unmarshal(body, errorMsg)
				if err != nil {
					logger.Warning(err)
					return false
				}
				logger.Warning(errorMsg.Msg)
				return false
			}
			m.targetBaseline = msg.Data.Version.Baseline
			m.targetVersion = msg.Data.Version.Version
			m.systemTypeFromPlatform = msg.Data.SystemType
			m.checkTime = time.Now().String()
			return true
		}
		logger.Warning(string(body))
		return false
	} else {
		logger.Warning(err)
		return false
	}
}

type updateTarget struct {
	TargetVersion string
	CheckTime     string
}

func (m *messageReportManager) getUpdateTarget() string {
	target := &updateTarget{
		TargetVersion: m.targetBaseline,
		CheckTime:     m.checkTime,
	}
	content, err := json.Marshal(target)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return string(content)
}

type reportCategory uint32

const (
	updateStatusReport reportCategory = iota
	downloadStatusReport
	upgradeStatusReport
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
			case updateStatusReport:
				logInfo.Tid = 1000600002
			case downloadStatusReport:
				logInfo.Tid = 1000600003
			case upgradeStatusReport:
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

const baselinePath = "/etc/os-baseline"

/*
[General]
Baseline=""
SystemType=""
*/
func getBaseline() string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(baselinePath)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	content, err := kf.GetString("General", "Baseline")
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return content
}

func getSystemType() string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(baselinePath)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	content, err := kf.GetString("General", "SystemType")
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return content
}

func (m *messageReportManager) UpdateBaselineFile() {
	updateBaseline(baselinePath, m.targetBaseline)
	updateSystemType(baselinePath, m.systemTypeFromPlatform)
}

func updateBaseline(path, content string) bool {
	kf := keyfile.NewKeyFile()
	if system.NormalFileExists(path) {
		err := kf.LoadFromFile(path)
		if err != nil {
			logger.Warning(err)
			return false
		}
	}
	kf.SetString("General", "Baseline", content)
	err := kf.SaveToFile(path)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}

func updateSystemType(path, content string) bool {
	kf := keyfile.NewKeyFile()
	if system.NormalFileExists(path) {
		err := kf.LoadFromFile(path)
		if err != nil {
			logger.Warning(err)
			return false
		}
	}
	kf.SetString("General", "SystemType", content)
	err := kf.SaveToFile(path)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}
