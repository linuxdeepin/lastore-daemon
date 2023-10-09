package main

import (
	"encoding/json"
	"errors"
	"internal/system"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
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

func (m *Manager) checkUpgrade(sender dbus.Sender, checkOrder checkType) (dbus.ObjectPath, *dbus.Error) {
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
	isExist, job, err = m.jobManager.CreateJob("", system.CheckSystemJobType, []string{checkOrder.JobType()}, nil, nil)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	job.setPreHooks(map[string]func() error{
		string(system.RunningStatus): func() error {
			inhibit(true)
			return nil
		},
		string(system.FailedStatus): func() error {
			m.updatePlatform.PostStatusMessage() // TODO 上报检查失败，上报本次更新失败
			inhibit(false)
			err = delRebootCheckOption(all)
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
			if checkOrder == firstCheck {
				// TODO 去掉第一次检查，此时如果重启，那么再次启动时不会再进行该检查
				err = delRebootCheckOption(checkOrder)
				if err != nil {
					logger.Warning(err)
				}
			}
			if checkOrder == secondCheck {
				// TODO 登录后检查无异常，去掉第二次检查，上报更新成功，更新baseline信息，还原grub配置，lastore 状态修改为ready
				err = delRebootCheckOption(secondCheck)
				if err != nil {
					logger.Warning(err)
				}
				err = m.grub.changeGrubDefaultEntry(normalBootEntry)
				if err != nil {
					logger.Warning(err)
				}
				err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
				if err != nil {
					logger.Warning(err)
				}
				// m.updatePlatform.postSystemUpgradeMessage(upgradeSucceed, job, mode)
				m.updatePlatform.UpdateBaseline()
				m.updatePlatform.recoverVersionLink()
			}
			return nil
		},
	})
	if isExist {
		return job.getPath(), nil
	}

	if err = m.jobManager.addJob(job); err != nil {
		return "", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

type checkOption struct {
	PreGreeterCheck   bool
	AfterGreeterCheck bool
}

const optionFilePath = "/etc/deepin/deepin_update_option.json" // 和upgrade_check.sh脚本中对应
func setRebootCheckOption() error {
	option := &checkOption{
		PreGreeterCheck:   true,
		AfterGreeterCheck: true,
	}
	content, err := json.Marshal(option)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(optionFilePath, content, 0644)
}

func delRebootCheckOption(order checkType) error {
	switch order {
	case firstCheck:
		option := &checkOption{}
		err := decodeJson(optionFilePath, option)
		if err != nil {
			return err
		}
		option.PreGreeterCheck = false
		content, err := json.Marshal(option)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(optionFilePath, content, 0644)
	case secondCheck, all:
		return os.RemoveAll(optionFilePath)
	default:
		return errors.New("invalid type")
	}
}
