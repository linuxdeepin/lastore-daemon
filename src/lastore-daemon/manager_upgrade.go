package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"internal/config"
	"internal/system"
	"internal/system/dut"
	"internal/updateplatform"
	"io/ioutil"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/utils"
)

func (m *Manager) distUpgradePartly(sender dbus.Sender, origin system.UpdateType, needBackup bool) (job dbus.ObjectPath, busErr *dbus.Error) {
	// 创建job，但是不添加到任务队列中
	var upgradeJob *Job
	var createJobErr error
	var startJobErr error
	var mode system.UpdateType
	// 非离线安装需要过滤可更新的选项
	if origin&system.OfflineUpdate == 0 {
		mode = m.statusManager.GetCanDistUpgradeMode(origin) // 正在安装的状态会包含其中,会在创建job中找到对应job(由于不追加安装,因此直接返回之前的job)
		if mode == 0 {
			return "", dbusutil.ToError(errors.New("don't exist can distUpgrade mode"))
		}
	} else {
		mode = origin
	}
	if updateplatform.IsForceUpdate(m.updatePlatform.Tp) {
		mode = origin
	}
	upgradeJob, createJobErr = m.distUpgrade(sender, mode, false, false, true)
	if createJobErr != nil {
		logger.Warning(createJobErr)
		return "", dbusutil.ToError(createJobErr)
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
func (m *Manager) distUpgrade(sender dbus.Sender, mode system.UpdateType, isClassify bool, needAdd bool, needChangeGrub bool) (*Job, error) {
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

	// 判断是否有可离线更新内容
	if mode == system.OfflineUpdate {
		if len(m.offline.upgradeAblePackageList) == 0 {
			return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", mode))
		}
	} else {
		packages = m.updater.getUpdatablePackagesByType(mode)
		if len(packages) == 0 {
			return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", mode))
		}
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
		if isClassify {
			jobType := GetUpgradeInfoMap()[mode].UpgradeJobId
			isExist, job, err = m.jobManager.CreateJob("", jobType, nil, environ, nil)
		} else {
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

		if mode == system.OfflineUpdate {
			job.option["Dir::State::lists"] = system.OfflineListPath
		}

		if mode == system.UnknownUpdate {
			job.option["DPkg::Options::"] = "--script-ignore-error"
		}

		m.handleSysPowerChanged()
		uuid, err = m.prepareAptCheck(mode)
		if err != nil {
			logger.Warning(err)
			m.updatePlatform.PostStatusMessage(fmt.Sprintf("%v gen dut meta failed, detail is: %v", mode, err.Error()))
			if unref != nil {
				unref()
			}
			return err
		}
		logger.Info(uuid)

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
				systemErr := dut.CheckSystem(dut.PreCheck, mode == system.OfflineUpdate, nil) // 只是为了执行precheck的hook脚本
				if systemErr != nil {
					logger.Info(systemErr)
					m.updatePlatform.PostStatusMessage(fmt.Sprintf("%v CheckSystem failed, detail is: %v", mode, systemErr.Error()))
					return systemErr
				}
				if !system.CheckInstallAddSize(mode) {
					return &system.JobError{
						Type:   system.ErrorInsufficientSpace,
						Detail: "There is not enough space on the disk to upgrade",
					}
				}
				m.preRunningHook(needChangeGrub, mode)
				return nil
			},
			string(system.FailedStatus): func() error {
				_ = m.preFailedHook(job, mode)
				return nil
			},
		})

		endJob.setPreHooks(map[string]func() error{
			string(system.SucceedStatus): func() error {
				systemErr := dut.CheckSystem(dut.MidCheck, mode == system.OfflineUpdate, nil)
				if systemErr != nil {
					logger.Info(systemErr)
					m.updatePlatform.PostStatusMessage(fmt.Sprintf("%v CheckSystem failed, detail is: %v", mode, systemErr.Error()))
					return systemErr
				}

				if mode&system.SystemUpdate != 0 {
					recordUpgradeLog(uuid, system.SystemUpdate, m.updatePlatform.SystemUpdateLogs, upgradeRecordPath)
				}

				if mode&system.SecurityUpdate != 0 {
					recordUpgradeLog(uuid, system.SecurityUpdate, m.updatePlatform.GetCVEUpdateLogs(m.updater.getUpdatablePackagesByType(system.SecurityUpdate)), upgradeRecordPath)
				}
				_ = m.preSuccessHook(job, needChangeGrub, mode)
				return nil
			},
			string(system.FailedStatus): func() error {
				_ = m.preFailedHook(job, mode)
				return nil
			},
		})

		endJob.setAfterHooks(map[string]func() error{
			string(system.SucceedStatus): func() error {
				m.config.SetInstallUpdateTime("")
				return m.afterSuccessHook()
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

	return job, nil
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
	if needChangeGrub {
		// 开始更新时修改grub默认入口为rollback
		// TODO 备份完成后调用grub2模块接口修改grub，由于grub2模块没有实时获取grub数据，可能需要kill grub2(或者grub2可以更新数据)在调用接口
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
	if mode&system.SystemUpdate != 0 {
		m.updatePlatform.ReplaceVersionCache()
	}
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
		err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.JobErrorType(errType)})
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
	// 安装失败删除配置文件
	err = m.delRebootCheckOption(all)
	if err != nil {
		logger.Warning(err)
	}
	go func() {
		m.inhibitAutoQuitCountAdd()
		defer m.inhibitAutoQuitCountSub()
		m.updatePlatform.PostSystemUpgradeMessage(updateplatform.UpgradeFailed, job.Description, mode)
		m.reportLog(upgradeStatusReport, false, job.Description)
		var allErrMsg []string
		for _, logPath := range job.errLogPath {
			content, err := ioutil.ReadFile(logPath)
			if err != nil {
				logger.Warning(err)
			}
			allErrMsg = append(allErrMsg, string(content))
		}
		msg, err := ioutil.ReadFile("/var/log/apt/term.log")
		if err != nil {
			logger.Warning("failed to get upgrade failed lod:", err)
		} else {
			allErrMsg = append(allErrMsg, string(msg))
		}
		m.updatePlatform.PostStatusMessage(fmt.Sprintf("%v upgrade failed, detail is: %v;all error message is %v", mode.JobType(), job.Description, strings.Join(allErrMsg, "\n")))
	}()
	m.statusManager.SetUpdateStatus(mode, system.UpgradeErr)
	// 如果安装失败，那么需要将version文件一直缓存，防止下次检查更新时version版本变高
	// m.updatePlatform.recoverVersionLink()
	return nil
}

func (m *Manager) preSuccessHook(job *Job, needChangeGrub bool, mode system.UpdateType) error {
	if (m.config.PlatformDisabled & config.DisabledRebootCheck) == 0 {
		if needChangeGrub {
			// 更新成功后修改grub默认入口为当前系统入口
			err := m.grub.createTempGrubEntry()
			if err != nil {
				logger.Warning(err)
			}
		}
		// 设置重启后的检查项
		err := m.setRebootCheckOption(mode)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		err := m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
		if err != nil {
			logger.Warning(err)
		}
		err = m.grub.changeGrubDefaultEntry(normalBootEntry)
		if err != nil {
			logger.Warning(err)
		}
		go func() {
			m.inhibitAutoQuitCountAdd()
			defer m.inhibitAutoQuitCountSub()
			// m.updatePlatform.postStatusMessage(fmt.Sprintf("%v postcheck error: %v", checkOrder, job.Description))
			m.updatePlatform.PostSystemUpgradeMessage(updateplatform.UpgradeSucceed, job.Description, mode)
			m.reportLog(upgradeStatusReport, true, "")
		}()
		// 只要系统更新，需要更新baseline文件
		if mode&system.SystemUpdate != 0 {
			m.updatePlatform.UpdateBaseline()
			m.updatePlatform.RecoverVersionLink()
		}
	}
	m.statusManager.SetUpdateStatus(mode, system.Upgraded)
	job.setPropProgress(1.00)
	m.updatePlatform.PostStatusMessage(fmt.Sprintf("%v install package success，need reboot and check", mode))
	return nil
}

func (m *Manager) afterSuccessHook() error {
	// 防止后台更新后注销再次进入桌面，导致错误发出通知。因此更新成功后设置为ready。由于保持running是为了处理更新异常中断场景，因此更新安装成功后，无需保持running状态。
	err := m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
	if err != nil {
		logger.Warning(err)
	}
	summary := gettext.Tr("Updates successful")
	msg := gettext.Tr("Restart the computer to use the system and applications properly.")
	action := []string{"reboot", gettext.Tr("Reboot Now"), "cancel", gettext.Tr("Reboot Later")}
	hints := map[string]dbus.Variant{
		"x-deepin-action-reboot": dbus.MakeVariant("dbus-send,--session,--print-reply,--dest=com.deepin.dde.shutdownFront,/com/deepin/dde/shutdownFront,com.deepin.dde.shutdownFront.Restart")}
	go m.sendNotify(updateNotifyShow, 0, "system-updated", summary, msg, action, hints, system.NotifyExpireTimeoutNoHide)

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

func onlyDownloadOfflinePackage(pkgsMap map[string]system.PackageInfo) error {
	var packages []string
	for _, info := range pkgsMap {
		packages = append(packages, fmt.Sprintf("%v=%v", info.Name, info.Version))
	}
	cmdStr := fmt.Sprintf("apt-get download %v -c /var/lib/lastore/apt_v2_common.conf --allow-change-held-packages -o Dir::State::lists=/var/lib/lastore/offline_list -o Dir::Etc::SourceParts=/dev/null -o Dir::Etc::SourceList=/var/lib/lastore/offline.list", strings.Join(packages, " "))
	cmd := exec.Command("/bin/sh", "-c", cmdStr)
	cmd.Dir = system.LocalCachePath // 该路径和/var/lib/lastore/apt_v2_common.conf保持一致
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		logger.Info(err)
		logger.Info(errBuf.String())
		return err
	}
	return nil
}

// 生成meta.json和uuid 暂时不使用dut，使用apt
func (m *Manager) prepareDutUpgrade(job *Job, mode system.UpdateType) (string, error) {
	// 使用dut更新前的准备
	var uuid string
	var err error
	if mode == system.OfflineUpdate {
		// 转移包路径
		err = onlyDownloadOfflinePackage(m.offline.upgradeAblePackages)
		if err != nil {
			return "", err
		}
		job.option["--meta-cfg"] = system.DutOfflineMetaConfPath
		uuid, err = dut.GenDutMetaFile(system.DutOfflineMetaConfPath,
			system.LocalCachePath,
			m.offline.upgradeAblePackages,
			m.offline.upgradeAblePackages, nil, m.offline.upgradeAblePackages, m.offline.removePackages, nil, genRepoInfo(mode, system.OfflineListPath))
		if err != nil {
			logger.Warning(err)
			return "", err
		}
	} else {
		job.option["--meta-cfg"] = system.DutOnlineMetaConfPath
		var pkgMap map[string]system.PackageInfo
		var removeMap map[string]system.PackageInfo
		mode &= system.AllInstallUpdate
		if mode == 0 {
			return "", errors.New("invalid mode")
		}

		// m.allUpgradableInfoMu.Lock()
		// pkgMap = m.mergePackagesByMode(mode, m.allUpgradableInfo)
		// m.allUpgradableInfoMu.Unlock()
		// m.allRemovePkgInfoMu.Lock()
		// removeMap = m.mergePackagesByMode(mode, m.allRemovePkgInfo)
		// m.allRemovePkgInfoMu.Unlock()

		coreListMap := make(map[string]system.PackageInfo)
		if m.coreList != nil && len(m.coreList) > 0 {
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
				data, err := ioutil.ReadFile(coreListVarPath)
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
		uuid, err = dut.GenDutMetaFile(system.DutOnlineMetaConfPath,
			system.LocalCachePath,
			pkgMap,
			coreListMap, nil, nil, removeMap,
			m.updatePlatform.GetRules(), genRepoInfo(mode, system.OnlineListPath))

		if err != nil {
			logger.Warning(err)
			return "", err
		}
	}
	return uuid, nil
}

// 生成meta.json和uuid
func (m *Manager) prepareAptCheck(mode system.UpdateType) (string, error) {
	var uuid string
	var err error
	// pkgMap repo和system.LocalCachePath都是占位用，没有起到真正的作用
	pkgMap := make(map[string]system.PackageInfo)
	pkgMap["lastore-daemon"] = system.PackageInfo{
		Name:    "lastore-daemon",
		Version: "1.0",
		Need:    "exist",
	}
	var repo []dut.RepoInfo
	if mode == system.OfflineUpdate {
		repo = genRepoInfo(system.OfflineUpdate, system.OfflineListPath)
	} else {
		repo = genRepoInfo(mode, system.OnlineListPath)
	}

	// coreList 生成
	coreListMap := make(map[string]system.PackageInfo)
	if m.coreList != nil && len(m.coreList) > 0 {
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
			data, err := ioutil.ReadFile(coreListVarPath)
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
	if mode == system.OfflineUpdate {
		uuid, err = dut.GenDutMetaFile(system.DutOfflineMetaConfPath,
			system.LocalCachePath,
			pkgMap,
			coreListMap, nil, nil, nil, nil, repo)
		if err != nil {
			logger.Warning(err)
			return "", err
		}
	} else {
		mode &= system.AllInstallUpdate
		if mode == 0 {
			return "", errors.New("invalid mode")
		}
		uuid, err = dut.GenDutMetaFile(system.DutOnlineMetaConfPath,
			system.LocalCachePath,
			pkgMap,
			coreListMap, nil, nil, nil,
			m.updatePlatform.GetRules(), repo)
		if err != nil {
			logger.Warning(err)
			return "", err
		}
	}
	return uuid, err
}

func (m *Manager) mergePackagesByMode(mode system.UpdateType, needMergePkgMap map[system.UpdateType]map[string]system.PackageInfo) map[string]system.PackageInfo {
	typeArray := system.UpdateTypeBitToArray(mode)
	var res map[string]system.PackageInfo
	if len(typeArray) == 0 {
		return nil
	}
	jsonStr, err := json.Marshal(needMergePkgMap[typeArray[0]])
	if err != nil {
		logger.Warning(err)
		return nil
	}
	err = json.Unmarshal(jsonStr, &res)
	if err != nil {
		logger.Warning(err)
		return nil
	}

	if len(typeArray) == 1 {
		return res
	}

	typeArray = typeArray[1:]
	for _, typ := range typeArray {
		for name, newInfo := range needMergePkgMap[typ] {
			originInfo, ok := res[name]
			if ok {
				// 当两个仓库存在相同包，留版本高的包
				if compareVersionsGe(newInfo.Version, originInfo.Version) {
					res[name] = newInfo
				}
			} else {
				// 不存在时直接添加
				res[name] = newInfo
			}
		}
	}
	return res
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
		data, err := ioutil.ReadFile(file)
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
	if typ&system.OfflineUpdate != 0 {
		urls = append(urls, getUpgradeUrls(system.GetCategorySourceMap()[system.OfflineUpdate])...)
	}
	if typ&system.UnknownUpdate != 0 {
		urls = append(urls, getUpgradeUrls(system.GetCategorySourceMap()[system.UnknownUpdate])...)
	}
	for _, url := range urls {
		prefixMap[strings.ReplaceAll(utils.URIToPath(url), "/", "_")] = struct{}{}
	}
	var prefixs []string
	for k := range prefixMap {
		prefixs = append(prefixs, k)
	}
	infos, err := ioutil.ReadDir(listPath)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	for _, info := range infos {
		if strings.HasSuffix(info.Name(), "Packages") {
			for _, prefix := range prefixs {
				unquotedStr, _ := url.QueryUnescape(info.Name())
				if strings.HasPrefix(unquotedStr, prefix) {
					res = append(res, filepath.Join(listPath, info.Name()))
				}
			}
		}
	}
	return res
}
