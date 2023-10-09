package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"internal/system"
	"strings"
	"sync"
	"syscall"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/gettext"
)

func (m *Manager) distUpgradePartly(sender dbus.Sender, mode system.UpdateType, needBackup bool) (job dbus.ObjectPath, busErr *dbus.Error) {
	// 创建job，但是不添加到任务队列中
	var upgradeJob *Job
	var createJobErr error
	var startJobErr error
	if mode&system.OfflineUpdate != 0 {
		info := m.offline.GetOfflineUpdateInfo()
		if len(info) == 0 {
			return "", dbusutil.ToError(errors.New("don't exist offline upgrade info"))
		}
	} else {
		mode = m.statusManager.GetCanDistUpgradeMode(mode) // 正在安装的状态会包含其中,会在创建job中找到对应job(由于不追加安装,因此直接返回之前的job)
		if mode == 0 {
			return "", dbusutil.ToError(errors.New("don't exist can distUpgrade mode"))
		}
	}
	upgradeJob, createJobErr = m.distUpgrade(sender, mode, false, false, true)
	if createJobErr != nil {
		logger.Warning(createJobErr)
		return "", dbusutil.ToError(createJobErr)
	}
	var inhibitFd dbus.UnixFD = -1
	why := Tr("Backing up and installing updates...")
	inhibit := func(enable bool) {
		logger.Infof("handle inhibit:%v fd:%v", enable, inhibitFd)
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
	var abHandler dbusutil.SignalHandlerId
	var canBackup bool
	var hasBackedUp bool
	var abErr error
	inhibit(true)
	defer func() {
		// 没有开始更新提前结束时，需要处理抑制锁和job
		if abErr != nil || startJobErr != nil {
			inhibit(false)
			err = m.CleanJob(upgradeJob.Id)
			if err != nil {
				logger.Warning(err)
			}
			m.statusManager.SetUpdateStatus(mode, system.CanUpgrade)
		}
	}()
	m.statusManager.SetUpdateStatus(mode, system.WaitRunUpgrade)
	if needBackup {
		// m.statusManager.SetABStatus(mode, system.NotBackup, system.NoABError)
		canBackup, abErr = m.abObj.CanBackup(0)
		if abErr != nil || !canBackup {
			logger.Info("can not backup,", abErr)

			msg := gettext.Tr("Backup failed!")
			action := []string{"continue", gettext.Tr("Proceed to Update")}
			hints := map[string]dbus.Variant{"x-deepin-action-continue": dbus.MakeVariant(
				fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, false))}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)

			m.inhibitAutoQuitCountSub()
			m.statusManager.SetABStatus(mode, system.BackupFailed, system.CanNotBackup)
			abErr = errors.New("can not backup")
			return "", dbusutil.ToError(abErr)
		}
		hasBackedUp, err = m.abObj.HasBackedUp().Get(0)
		if err != nil {
			logger.Warning(err)
		} else if hasBackedUp {
			m.statusManager.SetABStatus(mode, system.HasBackedUp, system.NoABError)
		}
		if !hasBackedUp {
			// 没有备份过，先备份再更新
			abErr = m.abObj.StartBackup(0)
			if abErr != nil {
				logger.Warning(abErr)

				msg := gettext.Tr("Backup failed!")
				action := []string{"backup", gettext.Tr("Back Up Again"), "continue", gettext.Tr("Proceed to Update")}
				hints := map[string]dbus.Variant{
					"x-deepin-action-backup": dbus.MakeVariant(
						fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, true)),
					"x-deepin-action-continue": dbus.MakeVariant(
						fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, false))}
				go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)

				m.inhibitAutoQuitCountSub()
				m.statusManager.SetABStatus(mode, system.BackupFailed, system.OtherError)
				return "", dbusutil.ToError(abErr)
			}
			m.statusManager.SetABStatus(mode, system.BackingUp, system.NoABError)
			abHandler, err = m.abObj.ConnectJobEnd(func(kind string, success bool, errMsg string) {
				if kind == "backup" {
					m.abObj.RemoveHandler(abHandler)
					if success {
						m.statusManager.SetABStatus(mode, system.HasBackedUp, system.NoABError)
						// 开始更新
						startJobErr = startUpgrade()
						if startJobErr != nil {
							logger.Warning(startJobErr)
						}
					} else {
						m.statusManager.SetABStatus(mode, system.BackupFailed, system.OtherError)
						logger.Warning("ab backup failed:", errMsg)
						// 备份失败后,需要清理原来的job,因为是监听信号,所以不能通过上面的defer处理.
						inhibit(false)
						err = m.CleanJob(upgradeJob.Id)
						if err != nil {
							logger.Warning(err)
						}
						m.statusManager.SetUpdateStatus(mode, system.CanUpgrade)
						msg := gettext.Tr("Backup failed!")
						action := []string{"backup", gettext.Tr("Back Up Again"), "continue", gettext.Tr("Proceed to Update")}
						hints := map[string]dbus.Variant{
							"x-deepin-action-backup": dbus.MakeVariant(
								fmt.Sprintf("dbus-send,--system,--print-reply,"+
									"--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, true)),
							"x-deepin-action-continue": dbus.MakeVariant(
								fmt.Sprintf("dbus-send,--system,--print-reply,"+
									"--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, false))}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				}
			})
			if err != nil {
				logger.Warning(err)
			}
		} else {
			// 备份过可以直接更新
			startJobErr = startUpgrade()
		}
	} else {
		// 无需备份,直接更新
		startJobErr = startUpgrade()
	}
	if startJobErr != nil {
		logger.Warning(startJobErr)
		return "", dbusutil.ToError(startJobErr)
	}
	return upgradeJob.getPath(), nil
}

// distUpgrade isClassify true: mode只能是单类型,创建一个单类型的更新job; false: mode类型不限,创建一个全mode类型的更新job
// needAdd true: 返回的job已经被add到jobManager中；false: 返回的job需要被调用者add
// TODO 处理离线更新
func (m *Manager) distUpgrade(sender dbus.Sender, mode system.UpdateType, isClassify bool, needAdd bool, needChangeGrub bool) (*Job, error) {
	if !system.IsAuthorized() {
		return nil, errors.New("not authorized, don't allow to exec upgrade")
	}
	execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	_ = mapMethodCaller(execPath, cmdLine) // TODO 需要对调用者进行鉴权
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.updateJobList()
	packages := m.updater.getUpdatablePackagesByType(mode)
	if len(packages) == 0 {
		return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", mode))
	}
	// TODO 检查系统环境是否满足安装条件

	var isExist bool
	var job *Job
	m.do.Lock()
	defer m.do.Unlock()
	if isClassify {
		// TODO classifiedUpgrade 逻辑
		isExist, job, err = m.jobManager.CreateJob("", GetUpgradeInfoMap()[mode].UpgradeJobId, packages, environ, nil)
	} else {
		isExist, job, err = m.jobManager.CreateJob("", system.DistUpgradeJobType, packages, environ, nil)
	}

	if err != nil {
		logger.Warningf("create DistUpgrade Job error: %v", err)
		return nil, err
	}
	if isExist {
		logger.Info(JobExistError)
		return job, nil
	}
	// 笔记本电池电量监听
	m.handleSysPowerChanged(job)
	// 设置hook
	job.setPreHooks(map[string]func() error{
		string(system.RunningStatus): func() error {
			return m.preRunningHook(needChangeGrub, mode)
		},
		string(system.FailedStatus): func() error {
			return m.preFailedHook(job, mode)
		},
		string(system.SucceedStatus): func() error {
			return m.preSuccessHook(job, needChangeGrub, mode)
		},
		string(system.EndStatus): func() error {
			m.sysPower.RemoveHandler(proxy.RemovePropertiesChangedHandler)
			return nil
		},
	})
	if needAdd { // 分类下载的job需要外部判断是否add
		if err := m.jobManager.addJob(job); err != nil {
			return nil, err
		}
	}

	cancelErr := m.cancelAllUpdateJob()
	if cancelErr != nil {
		logger.Warning(cancelErr)
	}

	return job, nil
}

func (m *Manager) handleSysPowerChanged(job *Job) {
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
			if job.Status != system.RunningStatus {
				return
			}
			if onBatteryGlobal && batteryPercentage < 60.0 && (job.Status == system.RunningStatus || job.Status == system.ReadyStatus) && lowPowerNotifyId == 0 {
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

func (m *Manager) preRunningHook(needChangeGrub bool, mode system.UpdateType) error {
	if needChangeGrub {
		// 开始更新时修改grub默认入口为rollback
		err := m.grub.changeGrubDefaultEntry(rollbackBootEntry)
		if err != nil {
			logger.Warning(err)
		}
	}
	// 状态更新为running
	err := m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeRunning, ReasonCode: system.NoError})
	if err != nil {
		logger.Warning(err)
	}
	m.statusManager.SetUpdateStatus(mode, system.Upgrading)
	// 替换cache文件,防止更新失败后os-version是错误的
	m.updatePlatform.replaceVersionCache()
	return nil
}

func (m *Manager) preFailedHook(job *Job, mode system.UpdateType) error {
	// 状态更新为failed
	var errorContent = struct {
		ErrType   string
		ErrDetail string
	}{}
	err := json.Unmarshal([]byte(job.Description), &errorContent)
	if err != nil {
		logger.Warning(err)
		err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.ErrorUnknown})
		if err != nil {
			logger.Warning(err)
		}
	} else {
		errType := errorContent.ErrType
		if strings.Contains(errType, "JobError::") {
			errType = strings.ReplaceAll(errType, "JobError::", "")
		}
		err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.UpgradeReasonType(errType)})
		if err != nil {
			logger.Warning(err)
		}
		canBackup, abErr := m.abObj.CanBackup(0)
		if abErr != nil {
			canBackup = false
		}
		if strings.Contains(errorContent.ErrType, string(system.ErrorDamagePackage)) {
			// 包损坏，需要下apt-get clean，然后重试更新
			cleanAllCache()
			msg := gettext.Tr("Updates failed: damaged files. Please update again.")
			action := []string{"retry", gettext.Tr("Try Again")}
			hints := map[string]dbus.Variant{"x-deepin-action-retry": dbus.MakeVariant("dde-control-center,-m,update")}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
		} else if strings.Contains(errorContent.ErrType, string(system.ErrorInsufficientSpace)) {
			// 空间不足
			// 已备份
			msg := gettext.Tr("Updates failed: insufficient disk space. Please reboot to avoid the effect on your system.")
			action := []string{"reboot", gettext.Tr("Reboot")}
			hints := map[string]dbus.Variant{"x-deepin-action-reboot": dbus.MakeVariant("dbus-send,--session,--print-reply,--dest=com.deepin.dde.shutdownFront,/com/deepin/dde/shutdownFront,com.deepin.dde.shutdownFront.Restart")}
			// 未备份
			if !canBackup {
				msg = gettext.Tr("Updates failed: insufficient disk space.")
				action = []string{}
				hints = nil
			}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
		} else {
			// 其他原因
			// 已备份
			msg := gettext.Tr("Updates failed. Please reboot to avoid the effect on your system.")
			action := []string{"reboot", gettext.Tr("Reboot")}
			hints := map[string]dbus.Variant{"x-deepin-action-reboot": dbus.MakeVariant("dbus-send,--session,--print-reply,--dest=com.deepin.dde.shutdownFront,/com/deepin/dde/shutdownFront,com.deepin.dde.shutdownFront.Restart")}
			// 未备份
			if !canBackup {
				msg = gettext.Tr("Updates failed.")
				action = []string{}
				hints = nil
			}
			go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
		}
	}

	go func() {
		m.inhibitAutoQuitCountAdd()
		defer m.inhibitAutoQuitCountSub()
		m.updatePlatform.postSystemUpgradeMessage(upgradeFailed, job, mode)
	}()
	m.updatePlatform.reportLog(upgradeStatusReport, false, job.Description)
	m.updatePlatform.PostStatusMessage()
	m.statusManager.SetUpdateStatus(mode, system.UpgradeErr)
	m.updatePlatform.recoverVersionLink()
	return nil
}

func (m *Manager) preSuccessHook(job *Job, needChangeGrub bool, mode system.UpdateType) error {
	// TODO 创建临时grub启动,在检查全部完成后，再修改为normal
	if needChangeGrub {
		// 更新成功后修改grub默认入口为当前系统入口
		err := m.grub.createTempGrubEntry()
		if err != nil {
			logger.Warning(err)
		}
	}
	// TODO 进行安装后检查

	// TODO 设置重启后的检查项
	err := setRebootCheckOption()
	if err != nil {
		logger.Warning(err)
	}
	// 状态更新为ready TODO 需要改成在重启后检查完成时修改为ready，此处应该保持为running状态
	// err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
	// if err != nil {
	// 	logger.Warning(err)
	// }
	m.statusManager.SetUpdateStatus(mode, system.Upgraded)
	job.setPropProgress(1.00)
	go func() {
		// 更新成功的上报需要在重启检查完成后上报
		// m.inhibitAutoQuitCountAdd()
		// defer m.inhibitAutoQuitCountSub()
		// m.updatePlatform.postSystemUpgradeMessage(upgradeSucceed, job, mode)
		// m.updatePlatform.UpdateBaseline()
	}()

	m.updatePlatform.reportLog(upgradeStatusReport, true, "")
	m.updatePlatform.PostStatusMessage()
	return nil
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
