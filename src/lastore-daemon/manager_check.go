// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/dut"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/godbus/dbus/v5"
)

type checkType uint32

const (
	firstCheck checkType = iota + 1
	secondCheck
	all
)

func (c checkType) JobType() string {
	switch c {
	case firstCheck:
		return "first check"
	case secondCheck:
		return "second check"
	default:
		return "invalid type"
	}
}

// 更新后重启的检查
func (m *Manager) checkUpgrade(sender dbus.Sender, checkMode system.UpdateType, checkOrder checkType) (dbus.ObjectPath, error) {
	m.updateJobList()
	if m.rebootTimeoutTimer != nil {
		m.rebootTimeoutTimer.Stop()
	}
	var inhibitFd dbus.UnixFD = -1
	why := Tr("Checking and installing updates...")
	inhibit := func(enable bool) {
		logger.Infof("handle inhibit:%v fd:%v", enable, inhibitFd)
		if enable {
			if inhibitFd == -1 {
				fd, err := Inhibitor("shutdown:sleep", dbusServiceName, why)
				if err != nil {
					logger.Infof("checkUpgrade:prevent shutdown failed: fd:%v, err:%v\n", fd, err)
				} else {
					logger.Infof("checkUpgrade:prevent shutdown: fd:%v\n", fd)
					inhibitFd = fd
				}
			}
		} else {
			if inhibitFd != -1 {
				err := syscall.Close(int(inhibitFd))
				if err != nil {
					logger.Infof("checkUpgrade:enable shutdown failed: fd:%d, err:%s\n", inhibitFd, err)
				} else {
					logger.Info("checkUpgrade:enable shutdown")
					inhibitFd = -1
				}
			}
		}
	}
	var job *Job
	var isExist bool
	var err error
	isExist, job, err = m.jobManager.CreateJob("", system.CheckSystemJobType, nil, nil, nil)
	if err != nil {
		return "", err
	}
	if isExist {
		return job.getPath(), nil
	}
	if checkOrder == firstCheck {
		job.option[dut.OptionFirstCheck] = "1"
	}

	uuid := getRebootCheckJobUUID()

	// 设置假的进度条，每200ms增长0.1的进度
	var fakeProgress chan bool = make(chan bool, 1)
	setFakeProgress := func(maxValue float64) {
		for job.Progress < maxValue && job.Status != system.FailedStatus {
			job.setPropProgress(job.Progress + 0.1)
			time.Sleep(time.Millisecond * 200)
		}
		fakeProgress <- true
	}

	job.setAfterHooks(map[string]func() error{
		string(system.RunningStatus): func() error {
			<-fakeProgress
			return nil
		},
	})
	job.setPreHooks(map[string]func() error{
		string(system.RunningStatus): func() error {
			// 起个go和job并行
			go setFakeProgress(0.9)
			inhibit(true)
			return nil
		},
		string(system.FailedStatus): func() error {
			m.updatePlatform.PostUpgradeStatus(uuid, updateplatform.UpgradeFailed, job.Description)
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()

				m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
					Type:           "error",
					UpdateType:     checkOrder.JobType(),
					JobDescription: job.Description,
					Detail:         fmt.Sprintf("%v postcheck error: %v", checkOrder.JobType(), job.Description),
				})

				m.reportLog(upgradeStatusReport, false, job.Description)
			}()
			inhibit(false)
			err = m.delRebootCheckOption(all)
			if err != nil {
				logger.Warning(err)
			}
			err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.ErrorUnknown}) // TODO reason应该需要新增
			if err != nil {
				logger.Warning(err)
			}
			return nil
		},
		string(system.SucceedStatus): func() error {
			inhibit(false)
			switch checkOrder {
			case firstCheck:
				// ps: 去掉第一次检查，此时如果重启，那么再次启动时不会再进行该检查
				err = m.delRebootCheckOption(checkOrder)
				if err != nil {
					logger.Warning(err)
				}
			case secondCheck:
				if err = m.immutableManager.osTreeFinalize(); err != nil {
					logger.Warning(err)
				}
				// ps: 登录后检查无异常，去掉第二次检查，上报更新成功，更新baseline信息，还原grub配置
				err = m.delRebootCheckOption(secondCheck)
				if err != nil {
					logger.Warning(err)
				}
				m.handleAfterUpgradeSuccess(checkMode, job.Description, uuid)
			default:
				logger.Warning("invalid check status:", checkOrder)
			}
			return nil
		},
	})

	if err = m.jobManager.addJob(job); err != nil {
		return "", err
	}
	return job.getPath(), nil
}

type fullUpgradeOption struct {
	DoUpgrade         bool
	DoUpgradeMode     system.UpdateType
	IsPowerOff        bool
	PreGreeterCheck   bool
	AfterGreeterCheck bool
	UUID              string
}

const (
	optionFilePath     = "/etc/deepin/deepin_update_option.json" // 和gen_upgrade_check_config.sh脚本中对应
	optionFilePathTemp = "/tmp/deepin_update_option.json"
)

func (m *Manager) setRebootCheckOption(mode system.UpdateType, uuid string) error {
	option := &fullUpgradeOption{
		DoUpgrade:         false,
		DoUpgradeMode:     mode,
		IsPowerOff:        false,
		PreGreeterCheck:   true,
		AfterGreeterCheck: true,
		UUID:              uuid,
	}
	content, err := json.Marshal(option)
	if err != nil {
		return err
	}
	_, _, err = m.systemd.EnableUnitFiles(0, []string{"lastore-after-upgrade-check.service"}, false, true)
	if err != nil {
		logger.Warning(err)
	}
	return os.WriteFile(optionFilePath, content, 0644)
}

func getRebootCheckJobUUID() string {
	content, err := os.ReadFile(optionFilePath)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	var option fullUpgradeOption
	err = json.Unmarshal(content, &option)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return option.UUID
}

func (m *Manager) delRebootCheckOption(order checkType) error {
	switch order {
	case firstCheck:
		option := &fullUpgradeOption{}
		err := decodeJson(optionFilePath, option)
		if err != nil {
			return err
		}
		option.PreGreeterCheck = false
		content, err := json.Marshal(option)
		if err != nil {
			return err
		}
		return os.WriteFile(optionFilePath, content, 0644)
	case secondCheck, all:
		err := os.RemoveAll(optionFilePathTemp)
		if err != nil {
			logger.Warning(err)
		}
		_, err = m.systemd.DisableUnitFiles(0, []string{"lastore-after-upgrade-check.service"}, false)
		if err != nil {
			logger.Warning(err)
		}
		return os.RemoveAll(optionFilePath)
	default:
		return errors.New("invalid type")
	}
}
