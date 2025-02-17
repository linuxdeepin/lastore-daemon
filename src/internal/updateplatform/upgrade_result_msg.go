// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/go-lib/utils"
)

// MsgPostStatus 更新结果上报状态
type MsgPostStatus string

const (
	NotReady    MsgPostStatus = "not ready"
	WaitPost    MsgPostStatus = "wait post"
	PostSuccess MsgPostStatus = "post success"
	PostFailure MsgPostStatus = "post failure"
)

var postContentCacheDir = filepath.Join("/var/cache/lastore", "post_msg_cache")

type UpgradePostMsg struct {
	SerialNumber    string        `json:"serialNumber"`
	MachineID       string        `json:"machineId"`
	UpgradeStatus   UpgradeResult `json:"status"`
	UpgradeErrorMsg string        `json:"msg"`
	TimeStamp       int64         `json:"timestamp"`
	SourceUrl       []string      `json:"sourceUrl"`
	Version         string        `json:"version"`

	PreBuild        string `json:"preBuild"`
	NextShowVersion string `json:"nextShowVersion"`
	PreBaseline     string `json:"preBaseline"`
	NextBaseline    string `json:"nextBaseline"`

	UpgradeStartTime int64 `json:"updateStartAt"`
	UpgradeEndTime   int64 `json:"updateFinishAt"`

	Uuid           string
	PostStatus     MsgPostStatus
	RetryCount     uint32
	upgradeLogPath string
}

type UpgradeResult int8

const (
	UpgradeSucceed UpgradeResult = 0
	UpgradeFailed  UpgradeResult = 1
	CheckFailed    UpgradeResult = 2
)

func (u *UpgradePostMsg) save() {
	_ = utils.EnsureDirExist(postContentCacheDir)
	content, err := json.Marshal(u)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = os.WriteFile(filepath.Join(postContentCacheDir, u.Uuid), content, 0600)
	if err != nil {
		logger.Warning(err)
		return
	}
}

func (u *UpgradePostMsg) init(path string) error {
	contentByte, err := os.ReadFile(path)
	if err != nil {
		logger.Warning(err)
		return err
	}
	err = json.Unmarshal(contentByte, u)
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

// 每次post的时候修改状态,无需持久化存储
func (u *UpgradePostMsg) updateTimeStamp() {
	u.TimeStamp = time.Now().Unix()
}

func (u *UpgradePostMsg) updatePostStatus(postStatus MsgPostStatus) {
	logger.Debugf("update %v log message status to %v", u.Uuid, postStatus)
	if postStatus == PostSuccess {
		if logger.GetLogLevel() == log.LevelDebug {
			logger.Debug("debug level, don't need delete upgrade result message")
		} else {
			err := os.RemoveAll(filepath.Join(postContentCacheDir, u.Uuid))
			if err != nil {
				logger.Warning(err)
			}
			return
		}
	}
	u.PostStatus = postStatus
	u.save()
}

func (u *UpgradePostMsg) addFailedCount() {
	u.RetryCount++
	u.save()
}

func getLocalJobPostMsg() (jobPostMsgMap map[string]*UpgradePostMsg) {
	jobPostMsgMap = make(map[string]*UpgradePostMsg)
	infos, err := os.ReadDir(postContentCacheDir)
	if err != nil {
		logger.Warning(err)
		return
	}
	for _, info := range infos {
		content := &UpgradePostMsg{}
		err := content.init(filepath.Join(postContentCacheDir, info.Name()))
		if err != nil {
			logger.Warning(err)
			continue
		}
		if content.PostStatus != PostSuccess {
			jobPostMsgMap[content.Uuid] = content
		}
	}
	return
}
