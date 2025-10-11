// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/dut"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/utils"
)

func (m *Manager) retryDistUpgrade(upgradeJob *Job, needBackup bool) error {
	if upgradeJob == nil {
		return errors.New("upgrade job is nil")
	}
	m.do.Lock()
	defer m.do.Unlock()

	backupJob := m.jobManager.findJobByType(system.BackupJobType, nil)
	// Handle existing backup job
	if backupJob != nil {
		if !backupJob.HasStatus(system.FailedStatus) {
			return errors.New("backup job status is not failed")
		}

		if needBackup {
			// Retry backup job
			if err := m.jobManager.MarkStart(backupJob.Id); err != nil {
				logger.Warningf("retryDistUpgrade: failed to mark start backup job: %v", err)
				return fmt.Errorf("failed to start backup job: %w", err)
			}
			return nil
		}

		// Clean backup job when backup is not needed
		if err := m.jobManager.CleanJob(backupJob.Id); err != nil {
			logger.Warningf("retryDistUpgrade: failed to clean backup job: %v", err)
			return fmt.Errorf("failed to clean backup job: %w", err)
		}
	} else {
		logger.Debug("backup job not found")
	}

	// Start upgrade job
	if err := m.jobManager.MarkStart(upgradeJob.Id); err != nil {
		logger.Warningf("retryDistUpgrade: failed to mark start upgrade job: %v", err)
		return fmt.Errorf("failed to start upgrade job: %w", err)
	}
	return nil
}

func (m *Manager) distUpgradePartly(sender dbus.Sender, origin system.UpdateType, needBackup bool) (job dbus.ObjectPath, busErr *dbus.Error) {
	// 创建job，但是不添加到任务队列中
	var upgradeJob *Job
	var createJobErr error
	var startJobErr error
	var mode system.UpdateType
	// 非离线安装需要过滤可更新的选项
	mode = m.statusManager.GetCanDistUpgradeMode(origin) // 正在安装的状态会包含其中,会在创建job中找到对应job(由于不追加安装,因此直接返回之前的job)
	if mode == 0 {
		return "", dbusutil.ToError(errors.New("don't exist can distUpgrade mode"))
	}

	if updateplatform.IsForceUpdate(m.updatePlatform.Tp) {
		mode = origin
	}
	upgradeJob, createJobErr = m.distUpgrade(sender, mode, false, false)
	if createJobErr != nil {
		if errors.Is(createJobErr, JobExistError) {
			// job exist, retry dist upgrade
			err := m.retryDistUpgrade(upgradeJob, needBackup)
			if err != nil {
				logger.Warningf("DistUpgradePartly: retry upgrade job failed: %v", err)
				return "/", dbusutil.ToError(err)
			}
			return upgradeJob.getPath(), nil
		}
		return "/", dbusutil.ToError(createJobErr)
	}
	var inhibitFd dbus.UnixFD = -1
	why := Tr("Backing up and installing updates...")
	inhibit := func(enable bool) {
		logger.Infof("DistUpgradePartly:handle inhibit:%v fd:%v", enable, inhibitFd)
		if enable {
			if inhibitFd == -1 {
				fd, err := Inhibitor("shutdown:sleep", dbusServiceName, why)
				if err != nil {
					logger.Infof("DistUpgradePartly:prevent shutdown failed: fd:%v, err:%v\n", fd, err)
				} else {
					logger.Infof("DistUpgradePartly:prevent shutdown: fd:%v\n", fd)
					inhibitFd = fd
				}
			}
		} else {
			if inhibitFd != -1 {
				err := syscall.Close(int(inhibitFd))
				if err != nil {
					logger.Infof("DistUpgradePartly:enable shutdown failed: fd:%d, err:%s\n", inhibitFd, err)
				} else {
					logger.Info("DistUpgradePartly:enable shutdown")
					inhibitFd = -1
				}
			}
		}
	}
	// 开始更新job
	startUpgrade := func() error {
		m.inhibitAutoQuitCountSub()
		m.do.Lock()
		defer m.do.Unlock()
		return m.jobManager.MarkStart(upgradeJob.Id)
	}

	// 对hook进行包装:增加关机阻塞的控制
	upgradeJob.wrapPreHooks(map[string]func() error{
		string(system.EndStatus): func() error {
			logger.Info("DistUpgradePartly:run wrap end hook")
			return nil
		},
		string(system.SucceedStatus): func() error {
			logger.Info("DistUpgradePartly:run wrap success hook")
			inhibit(false)
			return nil
		},
		string(system.FailedStatus): func() error {
			logger.Info("DistUpgradePartly:run wrap failed hook")
			inhibit(false)
			return nil
		},
		string(system.RunningStatus): func() error {
			logger.Info("DistUpgradePartly:run wrap running hook")
			inhibit(true)
			return nil
		},
	})
	m.updateJobList()
	// 将job的状态修改为pause,并添加到队列中,但是不开始
	upgradeJob.Status = system.PausedStatus
	err := m.jobManager.addJob(upgradeJob)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	m.inhibitAutoQuitCountAdd() // 开始备份前add，结束备份后sub(无论是否成功)
	var isExist bool
	var backupJob *Job
	if needBackup && system.NormalFileExists(system.DeepinImmutableCtlPath) {
		isExist, backupJob, err = m.jobManager.CreateJob("", system.BackupJobType, nil, nil, nil)
		if isExist {
			return "", dbusutil.ToError(JobExistError)
		}
		backupJob.next = upgradeJob
		backupJob.setPreHooks(map[string]func() error{
			string(system.RunningStatus): func() error {
				// 每次开始更新前，清理之前记录的日志文件
				_ = os.RemoveAll(logTmpPath)
				_ = m.logTmpFile.Close()
				m.logTmpFile = nil
				m.logTmpFile, err = os.OpenFile(logTmpPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err != nil {
					return fmt.Errorf("failed to open file %s: %v", logTmpPath, err)
				}
				inhibit(true)
				m.statusManager.SetABStatus(mode, system.BackingUp, system.NoABError)
				// 设置UpdateStatus为WaitRunUpgrade，隐藏更新并关机/重启按钮
				m.statusManager.SetUpdateStatus(mode, system.WaitRunUpgrade)
				return nil
			},
			string(system.SucceedStatus): func() error {
				m.statusManager.SetABStatus(mode, system.HasBackedUp, system.NoABError)
				inhibit(false)
				return nil
			},
			string(system.FailedStatus): func() error {
				m.statusManager.SetABStatus(mode, system.BackupFailed, system.OtherError)
				// 备份失败时重置UpdateStatus为CanUpgrade，让用户可以重新操作
				m.statusManager.SetUpdateStatus(mode, system.CanUpgrade)
				inhibit(false)
				msg := gettext.Tr("Backup failed!")
				action := []string{"backup", gettext.Tr("Back Up Again"), "continue", gettext.Tr("Proceed to Update")}
				hints := map[string]dbus.Variant{
					// Backup and continue dist upgrade
					"x-deepin-action-backup": dbus.MakeVariant(
						buildDistUpgradePartlyCommand(mode, true)),
					// Continue dist upgrade without backup
					"x-deepin-action-continue": dbus.MakeVariant(
						buildDistUpgradePartlyCommand(mode, false))}
				go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
				return nil
			},
		})
		backupJob.setAfterHooks(map[string]func() error{
			string(system.SucceedStatus): func() error {
				startJobErr = startUpgrade()
				if startJobErr != nil {
					logger.Warning(err)
				}
				return nil
			},
		})
		if err = m.jobManager.addJob(backupJob); err != nil {
			logger.Warning(err)
			return "", dbusutil.ToError(err)
		}
		return backupJob.getPath(), nil
	}
	defer func() {
		// 没有开始更新提前结束时，需要处理抑制锁和job
		if startJobErr != nil {
			err = m.CleanJob(upgradeJob.Id)
			if err != nil {
				logger.Warning(err)
			}
			m.statusManager.SetUpdateStatus(mode, system.CanUpgrade)
		}
	}()
	m.statusManager.SetUpdateStatus(mode, system.WaitRunUpgrade)
	startJobErr = startUpgrade()
	if startJobErr != nil {
		logger.Warning(startJobErr)
		return "", dbusutil.ToError(startJobErr)
	}
	return upgradeJob.getPath(), nil
}

// buildDistUpgradePartlyCommand builds a dbus-send command to invoke the DistUpgradePartly method
func buildDistUpgradePartlyCommand(mode system.UpdateType, needBackup bool) string {
	const methodName = "DistUpgradePartly"

	// Build dbus-send command with proper arguments
	args := []string{
		"dbus-send",
		"--system",
		"--print-reply",
		fmt.Sprintf("--dest=%s", dbusServiceName),
		dbusObjectPath,
		fmt.Sprintf("%s.%s", dbusInterfaceManager, methodName),
		fmt.Sprintf("uint64:%v", mode),
		fmt.Sprintf("boolean:%v", needBackup),
	}

	return strings.Join(args, ",")
}

// distUpgrade needAdd true: 返回的job已经被add到jobManager中；false: 返回的job需要被调用者add
func (m *Manager) distUpgrade(sender dbus.Sender, mode system.UpdateType, needAdd bool, needChangeGrub bool) (*Job, error) {
	m.checkDpkgCapabilityOnce.Do(func() {
		m.supportDpkgScriptIgnore = checkSupportDpkgScriptIgnore()
	})
	if !system.IsAuthorized() {
		return nil, errors.New("not authorized, don't allow to exec upgrade")
	}
	execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	caller := mapMethodCaller(execPath, cmdLine) // TODO 需要对调用者进行鉴权
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.updateJobList()
	var packages []string

	packages = m.updater.getUpdatablePackagesByType(mode)
	if len(packages) == 0 {
		return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", mode))
	}

	var isExist bool
	var job *Job
	var uuid string
	mergeMode := mode
	if mode != system.UnknownUpdate {
		mergeMode = mode & (^system.UnknownUpdate)
	}
	/*
		1. 仅有第三方仓库或没有第三方仓库时,mergeMode为当期更新内容,和原有逻辑一致;
		2. 多仓库包含第三方时,先融合非第三方仓库内容生成path,CreateJob中,分别创建排除第三方的更新job和第三方更新job,源配置分别在CreateJob后和CreateJob时设置;
		TODO: 该处逻辑和下载逻辑代码将业务和机制耦合太死,需要根据现有需求规划对该部分做新的设计;
	*/
	err = system.CustomSourceWrapper(mergeMode, func(path string, unref func()) error {
		m.do.Lock()
		defer m.do.Unlock()
		{
			option := map[string]interface{}{
				"UpdateMode":              mode, // 原始mode
				"SupportDpkgScriptIgnore": m.supportDpkgScriptIgnore,
			}
			isExist, job, err = m.jobManager.CreateJob("", system.DistUpgradeJobType, m.coreList, environ, option)
		}
		if err != nil {
			logger.Warningf("DistUpgrade error: %v\n", err)
			if unref != nil {
				unref()
			}
			return err
		}
		if isExist {
			logger.Infof("%v is exist", system.DistUpgradeJobType)
			return JobExistError
		}
		job.caller = caller

		if utils.IsDir(path) {
			job.option = map[string]string{
				"Dir::Etc::SourceList":  "/dev/null",
				"Dir::Etc::SourceParts": path,
			}
		} else {
			job.option = map[string]string{
				"Dir::Etc::SourceList":  path,
				"Dir::Etc::SourceParts": "/dev/null",
			}
		}

		if m.supportDpkgScriptIgnore && mode == system.UnknownUpdate {
			job.option["DPkg::Options::"] = "--script-ignore-error"
		}

		m.handleSysPowerChanged()

		// 设置hook
		// TODO 目前最多两个job关联,先这样写,后续规划抽象每个更新类型做处理.
		var endJob *Job
		startJob := job
		if job.next != nil {
			endJob = job.next
		} else {
			endJob = job
		}
		startJob.setPreHooks(map[string]func() error{
			string(system.RunningStatus): func() error {
				// 防止还在检查更新的时候，就生成了meta文件，此时meta文件可能不准
				var err error
				uuid, err = m.prepareAptCheck(mode)
				if err != nil {
					logger.Warning(err)

					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:       "error",
						UpdateType: mode.JobType(),
						Detail:     fmt.Sprintf("%v gen dut meta failed, detail is: %v", mode.JobType(), err.Error()),
					})

					if unref != nil {
						unref()
					}
					return err
				}
				logger.Info("update UUID:", uuid)
				m.updatePlatform.CreateJobPostMsgInfo(uuid, job.updateTyp)
				systemErr := dut.CheckSystem(dut.PreCheck, nil) // 只是为了执行precheck的hook脚本
				if systemErr != nil {
					logger.Info(systemErr)

					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:       "error",
						UpdateType: mode.JobType(),
						Detail:     fmt.Sprintf("CheckSystem failed, detail is: %v", systemErr.Error()),
					})
					return systemErr
				}
				if !system.CheckInstallAddSize(mode) {
					return &system.JobError{
						ErrType:      system.ErrorInsufficientSpace,
						ErrDetail:    "There is not enough space on the disk to upgrade",
						IsCheckError: true,
					}
				}
				m.preRunningHook(needChangeGrub, mode)
				return nil
			},
			string(system.FailedStatus): func() error {
				_ = m.preFailedHook(job, mode, uuid)
				return nil
			},
		})

		endJob.setPreHooks(map[string]func() error{
			string(system.SucceedStatus): func() error {
				systemErr := dut.CheckSystem(dut.MidCheck, nil)
				if systemErr != nil {
					logger.Info(systemErr)

					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:       "error",
						UpdateType: mode.JobType(),
						Detail:     fmt.Sprintf("%v CheckSystem failed, detail is: %v", mode.JobType(), systemErr.Error()),
					})
					return systemErr
				}
				if m.statusManager.abStatus == system.HasBackedUp {
					if err := m.immutableManager.osTreeRefresh(); err != nil {
						logger.Warning("ostree deploy refresh failed,", err)
					}
				}

				if mode&system.SystemUpdate != 0 {
					recordUpgradeLog(uuid, system.SystemUpdate, m.updatePlatform.SystemUpdateLogs, upgradeRecordPath)
				}

				if mode&system.SecurityUpdate != 0 {
					recordUpgradeLog(uuid, system.SecurityUpdate, m.updatePlatform.GetCVEUpdateLogs(m.updater.getUpdatablePackagesByType(system.SecurityUpdate)), upgradeRecordPath)
				}
				_ = m.preUpgradeCmdSuccessHook(job, needChangeGrub, mode, uuid)
				return nil
			},
			string(system.FailedStatus): func() error {
				_ = m.preFailedHook(job, mode, uuid)
				return nil
			},
		})

		endJob.setAfterHooks(map[string]func() error{
			string(system.SucceedStatus): func() error {
				err := m.config.SetInstallUpdateTime("")
				if err != nil {
					logger.Warning(err)
				}
				return m.afterUpgradeCmdSuccessHook()
			},
			string(system.EndStatus): func() error {
				m.sysPower.RemoveHandler(proxy.RemovePropertiesChangedHandler)
				if unref != nil {
					unref()
				}
				return nil
			},
		})
		if needAdd { // 分类下载的job需要外部判断是否add
			if err := m.jobManager.addJob(job); err != nil {
				if unref != nil {
					unref()
				}
				return err
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, JobExistError) { // exist的err通过最后的return返回即可
		logger.Warning(err)
		return nil, err
	}
	cancelErr := m.cancelAllUpdateJob()
	if cancelErr != nil {
		logger.Warning(cancelErr)
	}
	return job, err
}

func (m *Manager) handleSysPowerChanged() {
	isLaptop, err := m.sysPower.HasBattery().Get(0)
	if err == nil && isLaptop {
		var lowPowerNotifyId uint32 = 0
		var handleSysPowerBatteryEventMu sync.Mutex
		onBatteryGlobal, _ := m.sysPower.OnBattery().Get(0)
		batteryPercentage, _ := m.sysPower.BatteryPercentage().Get(0)
		/* 是否可以开始更新目前由前端管控
		if onBatteryGlobal && batteryPercentage <= 60.0 && (job.Status == system.RunningStatus || job.Status == system.ReadyStatus) {
			msg := gettext.Tr("请插入电源后再开始更新")
			_ = m.sendNotify("dde-control-center", 0, "notification-battery_low", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
			powerError := errors.New("inhibit dist-upgrade because low power")
			logger.Warningf("DistUpgrade error: %v\n", powerError)
			if unref != nil {
				unref()
			}
			return powerError
		}
		*/
		handleSysPowerBatteryEvent := func() {
			handleSysPowerBatteryEventMu.Lock()
			defer handleSysPowerBatteryEventMu.Unlock()
			if onBatteryGlobal && batteryPercentage < 60.0 && m.statusManager.isUpgrading() && lowPowerNotifyId == 0 {
				go func() {
					msg := gettext.Tr("The battery capacity is lower than 60%. To get successful updates, please plug in.")
					lowPowerNotifyId = m.sendNotify(updateNotifyShow, 0, "notification-battery_low", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
				}()
			}
			// 用户连上电源时,需要关闭通知
			if !onBatteryGlobal {
				if lowPowerNotifyId != 0 {
					err = m.closeNotify(lowPowerNotifyId)
					if err != nil {
						logger.Warning(err)
					} else {
						lowPowerNotifyId = 0
					}
				}
			}
		}
		// 更新过程中,如果笔记本使用电池,并且电量低于60%时,发送通知,提醒用户有风险
		handleSysPowerBatteryEvent()
		_ = m.sysPower.BatteryPercentage().ConnectChanged(func(hasValue bool, value float64) {
			if !hasValue {
				return
			}
			batteryPercentage = value
			go handleSysPowerBatteryEvent()
		})
		_ = m.sysPower.OnBattery().ConnectChanged(func(hasValue bool, onBattery bool) {
			if !hasValue {
				return
			}
			onBatteryGlobal = onBattery
			go handleSysPowerBatteryEvent()
		})
	}
}

func (m *Manager) preRunningHook(needChangeGrub bool, mode system.UpdateType) {
	// 状态更新为running
	err := m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeRunning, ReasonCode: system.NoError})
	if err != nil {
		logger.Warning(err)
	}
	m.statusManager.SetUpdateStatus(mode, system.Upgrading)
	// 替换cache文件,防止更新失败后os-version是错误的
	if mode&system.SystemUpdate != 0 {
		m.updatePlatform.ReplaceVersionCache()
	}
}

func (m *Manager) preFailedHook(job *Job, mode system.UpdateType, uuid string) error {
	// 状态更新为failed
	var errorContent system.JobError
	err := json.Unmarshal([]byte(job.Description), &errorContent)
	if err != nil {
		logger.Warning(err)
		err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.ErrorUnknown})
		if err != nil {
			logger.Warning(err)
		}
	} else {
		errType := errorContent.ErrType.String()
		err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.JobErrorType(errType)})
		if err != nil {
			logger.Warning(err)
		}
		if strings.Contains(errType, system.ErrorDamagePackage.String()) {
			// 包损坏，需要下apt-get clean，然后重试更新
			cleanAllCache()
			msg := gettext.Tr("Updates failed: damaged files. Please update again.")
			action := []string{"retry", gettext.Tr("Try Again")}
			hints := map[string]dbus.Variant{"x-deepin-action-retry": dbus.MakeVariant("dde-control-center,-m,update")}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
		} else if strings.Contains(errType, system.ErrorInsufficientSpace.String()) {
			msg := gettext.Tr("Updates failed: insufficient disk space.")
			action := []string{}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, nil, system.NotifyExpireTimeoutDefault)
		} else {
			msg := gettext.Tr("Updates failed.")
			action := []string{}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, nil, system.NotifyExpireTimeoutDefault)
		}
	}
	if errorContent.IsCheckError {
		m.updatePlatform.PostUpgradeStatus(uuid, updateplatform.CheckFailed, job.Description)
	} else {
		errorContent.ErrDetail = ""
		content, _ := json.Marshal(errorContent)
		// 安装失败不需要detail，PostStatusMessage会把term.log上报
		m.updatePlatform.PostUpgradeStatus(uuid, updateplatform.UpgradeFailed, string(content))
	}

	go func() {
		m.inhibitAutoQuitCountAdd()
		defer m.inhibitAutoQuitCountSub()
		m.reportLog(upgradeStatusReport, false, job.Description)
		var allErrMsg []string
		for _, logPath := range job.errLogPath {
			content, err := os.ReadFile(logPath)
			if err != nil {
				logger.Warning(err)
			}
			allErrMsg = append(allErrMsg, string(content))
		}
		if !errorContent.IsCheckError {
			msg, err := os.ReadFile("/var/log/apt/term.log")
			if err != nil {
				logger.Warning("failed to get upgrade failed lod:", err)
			} else {
				allErrMsg = append(allErrMsg, fmt.Sprintf("upgrade failed time is %v \n", time.Now()))
				allErrMsg = append(allErrMsg, string(msg))
			}
			m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
				Type:       "error",
				UpdateType: mode.JobType(),
				Detail:     fmt.Sprintf("upgrade failed, detail is: %v; all error message is %v", job.Description, strings.Join(allErrMsg, "\n")),
			})
		}
	}()
	m.statusManager.SetUpdateStatus(mode, system.UpgradeErr)
	// 如果安装失败，那么需要将version文件一直缓存，防止下次检查更新时version版本变高
	// m.updatePlatform.recoverVersionLink()
	return nil
}

func (m *Manager) preUpgradeCmdSuccessHook(job *Job, needChangeGrub bool, mode system.UpdateType, uuid string) error {
	if !m.config.GetPlatformStatusDisable(config.DisabledRebootCheck) {
		// 设置重启后的检查项;重启后需要检查时,需要将本次job的uuid记录到检查配置中,无需检查时只要将uuid直接记录到 upgradeJobMetaInfo 中即可
		err := m.setRebootCheckOption(mode, uuid)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		m.handleAfterUpgradeSuccess(mode, job.Description, uuid)
	}
	m.statusManager.SetUpdateStatus(mode, system.Upgraded)
	job.setPropProgress(1.00)

	m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
		Type:   "info",
		Detail: fmt.Sprintf("%v install package success, need reboot and check", mode.JobType()),
	})
	return nil
}

func (m *Manager) afterUpgradeCmdSuccessHook() error {
	// 防止后台更新后注销再次进入桌面，导致错误发出通知。因此更新成功后设置为ready。由于保持running是为了处理更新异常中断场景，因此更新安装成功后，无需保持running状态。
	err := m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
	if err != nil {
		logger.Warning(err)
	}
	summary := gettext.Tr("Updates successful")
	msg := gettext.Tr("Restart the computer to use the system and applications properly.")
	action := []string{"reboot", gettext.Tr("Reboot Now"), "cancel", gettext.Tr("Reboot Later")}
	hints := map[string]dbus.Variant{
		"x-deepin-action-reboot":      dbus.MakeVariant("dbus-send,--session,--print-reply,--dest=org.deepin.dde.ShutdownFront1,/org/deepin/dde/ShutdownFront1,org.deepin.dde.ShutdownFront1.Restart"),
		"x-deepin-NoAnimationActions": dbus.MakeVariant("reboot")}
	go m.sendNotify(updateNotifyShow, 0, "system-updated", summary, msg, action, hints, system.NotifyExpireTimeoutNoHide)
	return nil
}

// 整个更新流程走完后(安装、检测(如果需要)全部成功)
func (m *Manager) handleAfterUpgradeSuccess(mode system.UpdateType, des string, uuid string) {
	m.updatePlatform.PostUpgradeStatus(uuid, updateplatform.UpgradeSucceed, des)
	go func() {
		m.inhibitAutoQuitCountAdd()
		defer m.inhibitAutoQuitCountSub()
		m.reportLog(upgradeStatusReport, true, "")
	}()
	// 只要系统更新，需要更新baseline文件
	if mode&system.SystemUpdate != 0 {
		m.updatePlatform.UpdateBaseline()
		m.updatePlatform.RecoverVersionLink()
	}
}

// 安装更新时,需要退出所有不在运行的检查更新job
func (m *Manager) cancelAllUpdateJob() error {
	var updateJobIds []string
	for _, job := range m.jobManager.List() {
		if job.Type == system.UpdateJobType && job.Status != system.RunningStatus {
			updateJobIds = append(updateJobIds, job.Id)
		}
	}

	for _, jobId := range updateJobIds {
		err := m.jobManager.CleanJob(jobId)
		if err != nil {
			logger.Warningf("CleanJob %q error: %v\n", jobId, err)
		}
	}
	return nil
}

// 生成meta.json和uuid
func (m *Manager) prepareAptCheck(mode system.UpdateType) (string, error) {
	var uuid string
	var err error
	// repo和system.LocalCachePath都是占位用，没有起到真正的作用
	var repo []dut.RepoInfo

	repo = genRepoInfo(mode, system.OnlineListPath)

	// coreList 生成
	coreListMap := make(map[string]system.PackageInfo)
	if len(m.coreList) > 0 {
		for _, pkgName := range m.coreList {
			coreListMap[pkgName] = system.PackageInfo{
				Name:    pkgName,
				Version: "",
				Need:    "skipversion",
			}
		}
	} else {
		loadCoreList := func() map[string]system.PackageInfo {
			coreListMap := make(map[string]system.PackageInfo)
			data, err := os.ReadFile(coreListVarPath)
			if err != nil {
				return nil
			}
			var pkgList PackageList
			err = json.Unmarshal(data, &pkgList)
			if err != nil {
				return nil
			}
			for _, pkg := range pkgList.PkgList {
				coreListMap[pkg.PkgName] = system.PackageInfo{
					Name:    pkg.PkgName,
					Version: "",
					Need:    "skipversion",
				}
			}
			return nil
		}
		coreListMap = loadCoreList()
	}
	// 使用dut检查前的准备
	{
		mode &= system.AllInstallUpdate
		if mode == 0 {
			return "", errors.New("invalid mode")
		}
		m.updatePlatform.PrepareCheckScripts()
		uuid, err = dut.GenDutMetaFile(system.DutOnlineMetaConfPath,
			system.LocalCachePath, coreListMap, repo)
		if err != nil {
			logger.Warning(err)
			return "", err
		}
	}
	return uuid, err
}

// 生成repo信息
func genRepoInfo(typ system.UpdateType, listPath string) []dut.RepoInfo {
	var repoInfos []dut.RepoInfo
	for _, file := range getPackagesPathList(typ, listPath) {
		info := dut.RepoInfo{
			Name:       filepath.Base(file),
			FilePath:   file,
			HashSha256: "",
		}
		data, err := os.ReadFile(file)
		if err != nil {
			logger.Warning(err)
			continue
		}
		hash := sha256.New()
		hash.Write(data)
		info.HashSha256 = hex.EncodeToString(hash.Sum(nil))
		repoInfos = append(repoInfos, info)
	}
	return repoInfos
}

func getPackagesPathList(typ system.UpdateType, listPath string) []string {
	var res []string
	var urls []string
	prefixMap := make(map[string]struct{})
	if typ&system.SystemUpdate != 0 {
		urls = append(urls, getUpgradeUrls(system.GetCategorySourceMap()[system.SystemUpdate])...)
	}
	if typ&system.SecurityUpdate != 0 {
		urls = append(urls, getUpgradeUrls(system.GetCategorySourceMap()[system.SecurityUpdate])...)
	}
	if typ&system.UnknownUpdate != 0 {
		urls = append(urls, getUpgradeUrls(system.GetCategorySourceMap()[system.UnknownUpdate])...)
	}
	for _, repoUrl := range urls {
		prefixMap[strings.ReplaceAll(utils.URIToPath(repoUrl), "/", "_")] = struct{}{}
	}
	var prefixArray []string
	for k := range prefixMap {
		prefixArray = append(prefixArray, k)
	}
	infos, err := os.ReadDir(listPath)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	for _, info := range infos {
		if strings.HasSuffix(info.Name(), "Packages") {
			for _, prefix := range prefixArray {
				unquotedStr, _ := url.QueryUnescape(info.Name())
				if strings.HasPrefix(unquotedStr, prefix) {
					res = append(res, filepath.Join(listPath, info.Name()))
				}
			}
		}
	}
	return res
}
