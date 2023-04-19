// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"internal/system"

	"github.com/godbus/dbus"
	abrecovery "github.com/linuxdeepin/go-dbus-factory/com.deepin.abrecovery"
	apps "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.apps"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/gettext"
)

type Manager struct {
	service   *dbusutil.Service
	do        sync.Mutex
	updateApi system.System
	config    *Config
	PropsMu   sync.RWMutex
	// dbusutil-gen: equal=nil
	JobList    []dbus.ObjectPath
	jobList    []*Job
	jobManager *JobManager
	updater    *Updater

	// dbusutil-gen: ignore
	SystemArchitectures []system.Architecture

	// dbusutil-gen: equal=nil
	UpgradableApps []string

	SystemOnChanging bool
	AutoClean        bool

	inhibitFd        dbus.UnixFD
	updateSourceOnce bool

	apps       apps.Apps
	sysPower   power.Power
	signalLoop *dbusutil.SignalLoop

	UpdateMode      system.UpdateType `prop:"access:rw"` // 更新设置的内容
	CheckUpdateMode system.UpdateType `prop:"access:rw"` // 检查更新选中的内容
	UpdateStatus    string            // 每一个更新项的状态 json字符串

	HardwareId string

	inhibitAutoQuitCount int32
	autoQuitCountMu      sync.Mutex
	lastoreUnitCacheMu   sync.Mutex

	loginManager  login1.Manager
	sysDBusDaemon ofdbus.DBus
	systemd       systemd1.Manager
	abObj         abrecovery.ABRecovery

	grub           *grubManager
	userAgents     *userAgentMap // 闲时退出时，需要保存数据，启动时需要根据uid,agent sender以及session path完成数据恢复
	statusManager  *UpdateModeStatusManager
	messageManager *messageReportManager
	isDownloading  bool
}

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func NewManager(service *dbusutil.Service, updateApi system.System, c *Config) *Manager {
	archs, err := system.SystemArchitectures()
	if err != nil {
		logger.Errorf("Can't detect system supported architectures %v\n", err)
		return nil
	}

	m := &Manager{
		service:             service,
		config:              c,
		updateApi:           updateApi,
		SystemArchitectures: archs,
		inhibitFd:           -1,
		AutoClean:           c.AutoClean,
		userAgents:          newUserAgentMap(service),
		loginManager:        login1.NewManager(service.Conn()),
		sysDBusDaemon:       ofdbus.NewDBus(service.Conn()),
		signalLoop:          dbusutil.NewSignalLoop(service.Conn(), 10),
		apps:                apps.NewApps(service.Conn()),
		systemd:             systemd1.NewManager(service.Conn()),
		sysPower:            power.NewPower(service.Conn()),
		abObj:               abrecovery.NewABRecovery(service.Conn()),
	}
	m.signalLoop.Start()
	m.grub = newGrubManager(service.Conn(), m.signalLoop)
	m.messageManager = newMessageReportManager(c, m.userAgents)
	m.jobManager = NewJobManager(service, updateApi, m.updateJobList)
	go m.handleOSSignal()
	m.updateJobList()
	m.initStatusManager()
	hardwareId, err := getHardwareId()
	if err != nil {
		logger.Warning("failed to get HardwareId")
	} else {
		m.HardwareId = hardwareId
	}
	m.initDbusSignalListen()
	m.initDSettingsChangedHandle()
	return m
}

func (m *Manager) initDbusSignalListen() {
	m.loginManager.InitSignalExt(m.signalLoop, true)
	m.abObj.InitSignalExt(m.signalLoop, true)
	_, err := m.loginManager.ConnectSessionNew(m.handleSessionNew)
	if err != nil {
		logger.Warning(err)
	}
	_, err = m.loginManager.ConnectSessionRemoved(m.handleSessionRemoved)
	if err != nil {
		logger.Warning(err)
	}
	_, err = m.loginManager.ConnectUserRemoved(m.handleUserRemoved)
	if err != nil {
		logger.Warning(err)
	}
	m.sysDBusDaemon.InitSignalExt(m.signalLoop, true)
	_, err = m.sysDBusDaemon.ConnectNameOwnerChanged(func(name string, oldOwner string, newOwner string) {
		if strings.HasPrefix(name, ":") && oldOwner != "" && newOwner == "" {
			m.userAgents.handleNameLost(name)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	m.sysPower.InitSignalExt(m.signalLoop, true)
}

func (m *Manager) updatePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	caller := mapMethodCaller(execPath, cmdLine)
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()
	isExist, job, err := m.jobManager.CreateJob(jobName, system.UpdateJobType, pkgs, environ, nil)
	if err != nil {
		logger.Warningf("UpdatePackage %q error: %v\n", packages, err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}

	job.caller = caller
	return job, err
}

func (m *Manager) installPackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}

	lang := getUsedLang(environ)
	if lang == "" {
		logger.Warning("failed to get lang")
		return m.installPkg(jobName, packages, environ)
	}

	localePkgs := QueryEnhancedLocalePackages(system.QueryPackageInstallable, lang, pkgs...)
	if len(localePkgs) != 0 {
		logger.Infof("Follow locale packages will be installed:%v\n", localePkgs)
	}

	pkgs = append(pkgs, localePkgs...)
	return m.installPkg(jobName, strings.Join(pkgs, " "), environ)
}

func (m *Manager) installPkg(jobName, packages string, environ map[string]string) (*Job, error) {
	pList := strings.Fields(packages)

	m.do.Lock()
	defer m.do.Unlock()
	isExist, job, err := m.jobManager.CreateJob(jobName, system.InstallJobType, pList, environ, nil)
	if err != nil {
		logger.Warningf("installPackage %q error: %v\n", packages, err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	return job, err
}

func (m *Manager) removePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	if len(pkgs) == 1 {
		desktopFiles := listPackageDesktopFiles(pkgs[0])
		if len(desktopFiles) > 0 {
			err = m.apps.LaunchedRecorder().UninstallHints(0, desktopFiles)
			if err != nil {
				logger.Warningf("call UninstallHints(desktopFiles: %v) error: %v",
					desktopFiles, err)
			}
		}
	}

	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()
	isExist, job, err := m.jobManager.CreateJob(jobName, system.RemoveJobType, pkgs, environ, nil)
	if err != nil {
		logger.Warningf("removePackage %q error: %v\n", packages, err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	job.setHooks(map[string]func(){
		string(system.SucceedStatus): func() {
			msg := gettext.Tr("Removed successfully")
			m.sendNotify(system.GetAppStoreAppName(), 0, "deepin-appstore", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
		},
		string(system.FailedStatus): func() {
			msg := gettext.Tr("Failed to remove the app")
			action := []string{
				"retry",
				gettext.Tr("Retry"),
				"cancel",
				gettext.Tr("Cancel"),
			}
			hints := map[string]dbus.Variant{
				"x-deepin-action-retry":  dbus.MakeVariant(fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.StartJob,string:%s", job.Id)),
				"x-deepin-action-cancel": dbus.MakeVariant(fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.CleanJob,string:%s", job.Id))}
			m.sendNotify(system.GetAppStoreAppName(), 0, "deepin-appstore", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
		},
	})
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (m *Manager) ensureUpdateSourceOnce() {
	m.PropsMu.Lock()
	updateOnce := m.updateSourceOnce
	m.PropsMu.Unlock()

	if updateOnce {
		return
	}

	_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]), false)
	if err != nil {
		logger.Warning(err)
		return
	}
}

func (m *Manager) handleUpdateInfosChanged(sync bool) {
	logger.Info("handle UpdateInfos Changed")
	infosMap, err := SystemUpgradeInfo()
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info(err) // 移除文件时,同样会进入该逻辑,因此在update_infos.json文件不存在时,将日志等级改为info
		} else {
			logger.Error("failed to get upgrade info:", err)
		}
		return
	}
	m.updateUpdatableProp(infosMap)

	// 检查更新时,同步修改canUpgrade状态;检查更新时需要同步操作
	if sync {
		m.statusManager.UpdateModeAllStatusBySize()
		m.statusManager.UpdateCheckCanUpgradeByEachStatus()
	} else {
		go func() {
			m.statusManager.UpdateModeAllStatusBySize()
			m.statusManager.UpdateCheckCanUpgradeByEachStatus()
		}()
	}

	if m.updater.AutoDownloadUpdates && len(m.updater.UpdatablePackages) > 0 && sync && !m.updater.getIdleDownloadEnabled() {
		logger.Info("auto download updates")
		go func() {
			m.inhibitAutoQuitCountAdd()
			_, err := m.PrepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]))
			if err != nil {
				logger.Error("failed to prepare dist-upgrade:", err)
			}
			m.inhibitAutoQuitCountSub()
		}()
	}
}

// 根据解析update_infos.json数据的结果,将数据分别设置到Manager的UpgradableApps和Updater的UpdatablePackages,ClassifiedUpdatablePackages,UpdatableApps
func (m *Manager) updateUpdatableProp(infosMap system.SourceUpgradeInfoMap) {
	m.updater.setClassifiedUpdatablePackages(infosMap)
	m.PropsMu.RLock()
	updateType := m.UpdateMode
	m.PropsMu.RUnlock()
	filterInfos := getFilterInfosMap(infosMap, updateType)
	updatableApps := UpdatableNames(filterInfos)
	m.updatableApps(updatableApps) // Manager的UpgradableApps实际为可更新的包,而非应用;
	m.updater.setUpdatablePackages(updatableApps)
	m.updater.updateUpdatableApps()
}

func prepareUpdateSource() {
	partialFilePaths := []string{
		"/var/lib/apt/lists/partial",
		"/var/lib/lastore/lists/partial",
		"/var/cache/apt/archives/partial",
		"/var/cache/lastore/archives/partial",
	}
	for _, partialFilePath := range partialFilePaths {
		infos, err := ioutil.ReadDir(partialFilePath)
		if err != nil {
			logger.Warning(err)
			continue
		}
		for _, info := range infos {
			err = os.RemoveAll(filepath.Join(partialFilePath, info.Name()))
			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

func (m *Manager) updateSource(sender dbus.Sender, needNotify bool) (*Job, error) {
	m.do.Lock()
	defer m.do.Unlock()
	var err error
	var environ map[string]string
	defer func() {
		if err == nil {
			err1 := m.config.UpdateLastCheckTime()
			if err1 != nil {
				logger.Warning(err1)
			}
			err1 = m.updateAutoCheckSystemUnit()
			if err != nil {
				logger.Warning(err)
			}
		}
	}()
	var jobName string
	if needNotify {
		jobName = "+notify"
	}
	environ, err = makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	prepareUpdateSource()
	m.jobManager.dispatch() // 解决 bug 59351问题（防止CreatJob获取到状态为end但是未被删除的job）
	isExist, job, err := m.jobManager.CreateJob(jobName, system.UpdateSourceJobType, nil, environ, nil)
	if err != nil {
		logger.Warningf("UpdateSource error: %v\n", err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	if job != nil {
		job.setHooks(map[string]func(){
			string(system.RunningStatus): func() {
				m.PropsMu.Lock()
				m.updateSourceOnce = true
				m.PropsMu.Unlock()
				// 检查更新需要重置备份状态,主要是处理备份失败后再检查更新,会直接显示失败的场景
				m.statusManager.SetABStatus(system.NotBackup, system.NoABError)
			},
			string(system.SucceedStatus): func() {
				m.handleUpdateInfosChanged(true)
				if len(m.UpgradableApps) > 0 {
					m.messageManager.reportLog(updateStatus, true, "")
					// 开启自动下载时触发自动下载,发自动下载通知,不发送可更新通知;
					// 关闭自动下载时,发可更新的通知;
					if !m.updater.AutoDownloadUpdates {
						// msg := gettext.Tr("New system edition available")
						msg := gettext.Tr("New version available!")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,update")}
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				} else {
					m.messageManager.reportLog(updateStatus, false, "")
				}
			},
			string(system.FailedStatus): func() {
				// 网络问题检查更新失败和空间不足下载索引失败,需要发通知
				var errorContent = struct {
					ErrType   string
					ErrDetail string
				}{}
				err = json.Unmarshal([]byte(job.Description), &errorContent)
				if err == nil {
					if strings.Contains(errorContent.ErrType, string(system.ErrorFetchFailed)) || strings.Contains(errorContent.ErrType, string(system.ErrorIndexDownloadFailed)) {
						msg := gettext.Tr("Failed to check for updates. Please check your network.")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,network")}
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
					if strings.Contains(errorContent.ErrType, string(system.ErrorInsufficientSpace)) {
						msg := gettext.Tr("Failed to check for updates. Please clean up your disk first.")
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
					}
				}
				m.messageManager.reportLog(updateStatus, false, job.Description)
			},
		})
	}
	if err = m.jobManager.addJob(job); err != nil {
		logger.Warning(err)
		return nil, err
	}
	return job, nil
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

// distUpgrade isClassify true: mode只能是单类型,创建一个单类型的更新job; false: mode类型不限,创建一个全mode类型的更新job
// needAdd true: 返回的job已经被add到jobManager中；false: 返回的job需要被调用者add
func (m *Manager) distUpgrade(sender dbus.Sender, origin system.UpdateType, isClassify bool, needAdd bool) (*Job, error) {
	execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	caller := mapMethodCaller(execPath, cmdLine)
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.updateJobList()
	mode := m.statusManager.GetCanDistUpgradeMode(origin) // 正在安装的状态会包含其中,会在创建job中找到对应job(由于不追加安装,因此直接返回之前的job)
	if mode == 0 {
		return nil, errors.New("don't exit can distUpgrade mode")
	}
	if len(m.updater.getUpdatablePackagesByType(mode)) == 0 {
		return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", mode))
	}

	var job *Job
	var isExist bool
	err = system.CustomSourceWrapper(mode, func(path string, unref func()) error {
		m.do.Lock()
		defer m.do.Unlock()
		if isClassify {
			jobType := GetUpgradeInfoMap()[mode].UpgradeJobId
			isExist, job, err = m.jobManager.CreateJob("", jobType, nil, environ, nil)
		} else {
			isExist, job, err = m.jobManager.CreateJob("", system.DistUpgradeJobType, nil, environ, nil)
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
		// 设置apt命令参数
		info, err := os.Stat(path)
		if err != nil {
			if unref != nil {
				unref()
			}
			return err
		}
		if info.IsDir() {
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
		// 笔记本电池电量监听
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
					msg := gettext.Tr("The battery capacity is lower than 60%. To get successful updates, please plug in.")
					lowPowerNotifyId = m.sendNotify(updateNotifyShow, 0, "notification-battery_low", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
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
		// 设置hook
		job.setHooks(map[string]func(){
			string(system.RunningStatus): func() {
				// 开始更新时修改grub默认入口为rollback
				err := m.grub.changeGrubDefaultEntry(rollbackBootEntry)
				if err != nil {
					logger.Warning(err)
				}
				// 状态更新为running
				err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeRunning, ReasonCode: system.NoError})
				if err != nil {
					logger.Warning(err)
				}
				m.statusManager.SetUpdateStatus(mode, system.Upgrading)
				// mask deepin-desktop-base,该包在系统更新完成后最后安装
				system.HandleDelayPackage(true, []string{
					"deepin-desktop-base",
				})
			},
			string(system.FailedStatus): func() {
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
					err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeFailed, ReasonCode: system.UpgradeReasonType(errorContent.ErrType)})
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
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
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
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
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
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				}

				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.messageManager.postSystemUpgradeMessage(upgradeFailed, job, mode)
				}()
				m.messageManager.reportLog(upgradeStatus, false, job.Description)
				m.statusManager.SetUpdateStatus(mode, system.UpgradeErr)
				// unmask deepin-desktop-base 无需继续安装
				system.HandleDelayPackage(false, []string{
					"deepin-desktop-base",
				})
			},
			string(system.SucceedStatus): func() {
				// 更新成功后修改grub默认入口为当前系统入口
				err := m.grub.changeGrubDefaultEntry(normalBootEntry)
				if err != nil {
					logger.Warning(err)
				}
				// 状态更新为ready
				err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
				if err != nil {
					logger.Warning(err)
				}
				// unmask deepin-desktop-base并安装
				system.HandleDelayPackage(false, []string{
					"deepin-desktop-base",
				})
				// 只处理系统更新
				if mode&system.SystemUpdate != 0 {
					// 系统更新成功后,最后安装deepin-desktop-base包,安装成功后进度更新为100%并变成succeed状态
					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						m.do.Lock()
						defer m.do.Unlock()
						isExist, installJob, err := m.jobManager.CreateJob("install base", system.OnlyInstallJobType, []string{"deepin-desktop-base"}, environ, nil)
						if err != nil {
							wg.Done()
							logger.Warning(err)
							return
						}
						if isExist {
							wg.Done()
							return
						}
						if installJob != nil {
							installJob.option = job.option
							installJob.setHooks(map[string]func(){
								string(system.FailedStatus): func() {
									wg.Done()
								},
								string(system.SucceedStatus): func() {
									wg.Done()
								},
							})
							if err := m.jobManager.addJob(installJob); err != nil {
								logger.Warning(err)
								wg.Done()
								return
							}
						}
					}()
					wg.Wait()
					logger.Info("install deepin-desktop-base done,upgrade succeed.")
				}
				m.statusManager.SetUpdateStatus(mode, system.Upgraded)
				// 等待deepin-desktop-base安装完成后,状态后续切换
				job.setPropProgress(1.00)
				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.messageManager.postSystemUpgradeMessage(upgradeSucceed, job, mode)
				}()

				m.messageManager.reportLog(upgradeStatus, true, "")
			},
			string(system.EndStatus): func() {
				// wrapper的资源释放
				if unref != nil {
					unref()
				}
				m.sysPower.RemoveHandler(proxy.RemovePropertiesChangedHandler)
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
	if err != nil && err != JobExistError { // exist的err通过最后的return返回即可
		logger.Warning(err)
		return nil, err
	}
	cancelErr := m.cancelAllUpdateJob()
	if cancelErr != nil {
		logger.Warning(cancelErr)
	}

	return job, err
}

// prepareDistUpgrade isClassify true: mode只能是单类型,创建一个单类型的下载job; false: mode类型不限,创建一个全mode类型的下载job
func (m *Manager) prepareDistUpgrade(sender dbus.Sender, origin system.UpdateType, isClassify bool) (*Job, error) {
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.ensureUpdateSourceOnce()
	m.updateJobList()
	mode := m.statusManager.GetCanPrepareDistUpgradeMode(origin) // 正在下载的状态会包含其中,会在创建job中找到对应job(由于不追加下载,因此直接返回之前的job) TODO 如果需要追加下载,需要根据前后path的差异,reload该job
	if mode == 0 {
		return nil, errors.New("don't exit can prepareDistUpgrade mode")
	}
	if len(m.updater.getUpdatablePackagesByType(mode)) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}
	var needDownloadSize float64
	if needDownloadSize, _, err = system.QuerySourceDownloadSize(mode); err == nil && needDownloadSize == 0 {
		return nil, system.NotFoundError("no need download")
	}
	// 下载前检查/var分区的磁盘空间是否足够下载,
	isInsufficientSpace := false
	content, err := exec.Command("/bin/sh", []string{
		"-c",
		"df -BK --output='avail' /var|awk 'NR==2'",
	}...).CombinedOutput()
	if err != nil {
		logger.Warning(string(content))
	} else {
		spaceStr := strings.Replace(string(content), "K", "", -1)
		spaceStr = strings.TrimSpace(spaceStr)
		spaceNum, err := strconv.Atoi(spaceStr)
		if err != nil {
			logger.Warning(err)
		} else {
			spaceNum = spaceNum * 1000
			isInsufficientSpace = spaceNum < int(needDownloadSize)
		}
	}
	if isInsufficientSpace {
		dbusError := struct {
			ErrType string
			Detail  string
		}{
			string(system.ErrorInsufficientSpace),
			"You don't have enough free space to download",
		}
		msg := fmt.Sprintf(gettext.Tr("Downloading updates failed. Please free up %n GB disk space first."), needDownloadSize/(1000*1000*1000))
		m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		logger.Warning(dbusError.Detail)
		errStr, _ := json.Marshal(dbusError)
		return nil, dbusutil.ToError(errors.New(string(errStr)))
	}
	var job *Job
	var isExist bool

	// 新的下载处理方式
	m.do.Lock()
	defer m.do.Unlock()
	if isClassify {
		jobType := GetUpgradeInfoMap()[mode].PrepareJobId
		if jobType == "" {
			return nil, fmt.Errorf("invalid args: %v", mode)
		}
		const jobName = "OnlyDownload" // 提供给daemon的lastore模块判断当前下载任务是否还有后续更新任务
		isExist, job, err = m.jobManager.CreateJob(jobName, jobType, nil, environ, nil)
	} else {
		option := map[string]interface{}{
			"UpdateMode":   mode,
			"DownloadSize": m.statusManager.GetAllUpdateModeDownloadSize(),
		}
		isExist, job, err = m.jobManager.CreateJob("", system.PrepareDistUpgradeJobType, nil, environ, option)
	}
	if err != nil {
		logger.Warningf("Prepare DistUpgrade error: %v\n", err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	currentJob := job
	var sendDownloadingOnce sync.Once
	// 遍历job和所有next
	var downloadJobList []*Job
	for currentJob != nil {
		downloadJobList = append(downloadJobList, currentJob)
		currentJob = currentJob.next
	}
	for i, downloadJob := range downloadJobList {
		j := downloadJob
		if m.updater.downloadSpeedLimitConfigObj.DownloadSpeedLimitEnabled {
			j.option[aptLimitKey] = m.updater.downloadSpeedLimitConfigObj.LimitSpeed
		}
		j.setHooks(map[string]func(){
			string(system.ReadyStatus): func() {
				m.PropsMu.Lock()
				m.isDownloading = true
				m.PropsMu.Unlock()
			},
			string(system.RunningStatus): func() {
				m.PropsMu.Lock()
				m.isDownloading = true
				m.PropsMu.Unlock()
				m.statusManager.SetUpdateStatus(mode, system.IsDownloading)
				sendDownloadingOnce.Do(func() {
					msg := gettext.Tr("New version available! Downloading...")
					action := []string{
						"view",
						gettext.Tr("View"),
					}
					hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,update")}
					m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
				})
			},
			string(system.PausedStatus): func() {
				m.statusManager.SetUpdateStatus(mode, system.DownloadPause)
			},
			string(system.FailedStatus): func() {
				m.PropsMu.Lock()
				m.isDownloading = false
				packages := m.UpgradableApps
				m.PropsMu.Unlock()
				m.messageManager.reportLog(downloadStatus, false, j.Description)
				m.statusManager.SetUpdateStatus(mode, system.DownloadErr)
				var errorContent = struct {
					ErrType   string
					ErrDetail string
				}{}
				err = json.Unmarshal([]byte(j.Description), &errorContent)
				if err == nil {
					if strings.Contains(errorContent.ErrType, string(system.ErrorInsufficientSpace)) {
						var msg string
						size, _, err := system.QueryPackageDownloadSize(mode, packages...)
						if err != nil {
							logger.Warning(err)
							size = needDownloadSize
						}
						msg = fmt.Sprintf(gettext.Tr("Downloading updates failed. Please free up %n GB disk space first."), size/(1000*1000*1000))
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
					} else if strings.Contains(errorContent.ErrType, string(system.ErrorDamagePackage)) {
						// 下载更新失败，需要apt-get clean后重新下载
						cleanAllCache()
						msg := gettext.Tr("Updates failed: damaged files. Please update again.")
						action := []string{"retry", gettext.Tr("Try Again")}
						hints := map[string]dbus.Variant{"x-deepin-action-retry": dbus.MakeVariant("dde-control-center,-m,update")}
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					} else if strings.Contains(errorContent.ErrType, string(system.ErrorFetchFailed)) {
						// 网络原因下载更新失败
						msg := gettext.Tr("Downloading updates failed. Please check your network.")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,network")}
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				}
			},
			string(system.EndStatus): func() {
				// pause->end  status更新为未下载.用于适配取消下载的状态
				// if j.Status == system.PausedStatus || m.statusManager.GetUpdateStatus(mode) == system.DownloadPause {
				// 	m.statusManager.SetUpdateStatus(mode, system.NotDownload)
				// 	m.statusManager.UpdateModeAllStatusBySize()
				// }
				// 除了下载完成和下载失败之外,其他的情况都要先把状态设置成未下载.然后通过size进行状态修正
				if m.statusManager.GetUpdateStatus(mode) != system.CanUpgrade && m.statusManager.GetUpdateStatus(mode) != system.DownloadErr {
					m.statusManager.SetUpdateStatus(mode, system.NotDownload)
					m.statusManager.UpdateModeAllStatusBySize()
				}
			},
			// 在reload状态时,修改job的配置
			string(system.ReloadStatus): func() {
				logger.Infof("job reload option:%+v", j.option)
				if m.updater.downloadSpeedLimitConfigObj.DownloadSpeedLimitEnabled {
					if j.option == nil {
						j.option = make(map[string]string)
					}
					j.option[aptLimitKey] = m.updater.downloadSpeedLimitConfigObj.LimitSpeed
				} else {
					if j.option != nil {
						delete(j.option, aptLimitKey)
					}
				}
			},
		})
		// 应该只有最后一个处理
		if i == len(downloadJobList)-1 {
			j.wrapHooks(map[string]func(){
				string(system.SucceedStatus): func() {
					// 有可能一个模块下载完后，其他模块由于有相同的包，状态同样变化
					m.statusManager.UpdateModeAllStatusBySize()
					// 按每个仓库下载，就不会存在该场景：
					// 两个仓库存在相同包但是版本不同，可能存在一个仓库下载好一个仓库未下载的场景，因此
					// m.statusManager.setUpdateStatus(mode, system.CanUpgrade)
					m.statusManager.UpdateCheckCanUpgradeByEachStatus()
					msg := gettext.Tr("Downloading completed. You can install updates when shutdown or reboot.")
					action := []string{
						"updateNow",
						gettext.Tr("Update Now"),
						"ignore",
						gettext.Tr("Dismiss"),
					}
					hints := map[string]dbus.Variant{"x-deepin-action-updateNow": dbus.MakeVariant("dbus-send,--session,--print-reply,--dest=com.deepin.dde.shutdownFront,/com/deepin/dde/shutdownFront,com.deepin.dde.shutdownFront.UpdateAndShutdown")}
					m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					m.messageManager.reportLog(downloadStatus, true, "")
				},
				string(system.EndStatus): func() {
					m.PropsMu.Lock()
					m.isDownloading = false
					m.PropsMu.Unlock()
				},
			})
		}
	}

	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

// 根据更新类型,创建对应的下载或下载+安装的job
func (m *Manager) classifiedUpgrade(sender dbus.Sender, updateType system.UpdateType, isUpgrade bool) ([]dbus.ObjectPath, *dbus.Error) {
	var jobPaths []dbus.ObjectPath
	var err error
	var errList []string
	// 保证任务创建顺序
	for _, t := range system.AllUpdateType() {
		category := updateType & t
		if category != 0 {
			var upgradeJob, prepareJob *Job
			if isUpgrade {
				prepareJob, err = m.prepareDistUpgrade(sender, category, true)
				if err != nil {
					if !strings.Contains(err.Error(), system.NotFoundErrorMsg) {
						errList = append(errList, err.Error())
						logger.Warning(err)
						continue
					} else {
						logger.Info(err)
						// 可能无需下载,因此继续后面安装job的创建
					}
				}
				upgradeJob, err = m.distUpgrade(sender, category, true, false)
				if err != nil && err != JobExistError {
					if !strings.Contains(err.Error(), system.NotFoundErrorMsg) {
						errList = append(errList, err.Error())
						logger.Warning(err)
					} else {
						logger.Info(err)
					}
					continue
				}
				// 如果需要下载job,则绑定下载和安装job.无需下载job,直接将安装job添加进队列即可
				if prepareJob != nil {
					jobPaths = append(jobPaths, prepareJob.getPath())
					prepareJob.next = upgradeJob
				} else {
					if err := m.jobManager.addJob(upgradeJob); err != nil {
						errList = append(errList, err.Error())
						logger.Warning(err)
					}
				}
				jobPaths = append(jobPaths, upgradeJob.getPath())
			} else {
				prepareJob, err = m.prepareDistUpgrade(sender, category, true)
				if err != nil {
					if !strings.Contains(err.Error(), system.NotFoundErrorMsg) {
						errList = append(errList, err.Error())
						logger.Warning(err)
					} else {
						logger.Info(err)
					}
					continue
				}
				jobPaths = append(jobPaths, prepareJob.getPath())
			}
		}
	}
	if len(errList) > 0 {
		return jobPaths, dbusutil.ToError(errors.New(strings.Join(errList, ",")))
	}
	return jobPaths, nil
}

func (m *Manager) cleanArchives(needNotify bool) (*Job, error) {
	var jobName string
	if needNotify {
		jobName = "+notify"
	}

	m.do.Lock()
	defer m.do.Unlock()
	isExist, job, err := m.jobManager.CreateJob(jobName, system.CleanJobType, nil, nil, nil)
	if err != nil {
		logger.Warningf("CleanArchives error: %v", err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	job.setHooks(map[string]func(){
		string(system.EndStatus): func() {
			// 清理完成的通知
			msg := gettext.Tr("Package cache wiped")
			m.sendNotify(updateNotifyShow, 0, "deepin-appstore", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
		},
	})
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	err = m.config.UpdateLastCleanTime()
	if err != nil {
		return nil, err
	}
	err = m.config.UpdateLastCheckCacheSizeTime()
	if err != nil {
		return nil, err
	}

	return job, err
}

func (m *Manager) fixError(sender dbus.Sender, errType string) (*Job, error) {
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}

	switch errType {
	case system.ErrTypeDpkgInterrupted, system.ErrTypeDependenciesBroken:
		// good error type
	default:
		return nil, errors.New("invalid error type")
	}

	m.do.Lock()
	defer m.do.Unlock()
	isExist, job, err := m.jobManager.CreateJob("", system.FixErrorJobType, []string{errType}, environ, nil)
	if err != nil {
		logger.Warningf("fixError error: %v", err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	return job, err
}

func (m *Manager) updateModeWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	writeMode := system.UpdateType(pw.Value.(uint64))
	newMode := m.statusManager.SetUpdateMode(writeMode)
	pw.Value = newMode
	return nil
}

func (m *Manager) checkUpdateModeWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	writeType := system.UpdateType(pw.Value.(uint64))
	newMode := m.statusManager.SetCheckMode(writeType)
	pw.Value = newMode
	return nil
}

func (m *Manager) categorySupportAutoInstall(category system.UpdateType) bool {
	m.updater.PropsMu.RLock()
	autoInstallUpdates := m.updater.AutoInstallUpdates
	autoInstallUpdateType := m.updater.AutoInstallUpdateType
	m.updater.PropsMu.RUnlock()
	return autoInstallUpdates && (autoInstallUpdateType&category != 0)
}

func (m *Manager) handleAutoCheckEvent() error {
	if m.config.AutoCheckUpdates {
		_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]), m.updater.UpdateNotify)
		if err != nil {
			logger.Warning(err)
			return err
		}
	}
	if !m.config.DisableUpdateMetadata {
		startUpdateMetadataInfoService()
	}
	return nil
}

func (m *Manager) handleAutoCleanEvent() error {
	const MaxCacheSize = 500.0 // size MB
	doClean := func() error {
		logger.Debug("call doClean")

		_, err := m.cleanArchives(true)
		if err != nil {
			logger.Warningf("CleanArchives failed: %v", err)
			return err
		}
		return nil
	}

	calcRemainingDuration := func() time.Duration {
		elapsed := time.Since(m.config.LastCleanTime)
		if elapsed < 0 {
			// now time < last clean time : last clean time (from config) is invalid
			return -1
		}
		return m.config.CleanInterval - elapsed
	}

	calcRemainingCleanCacheOverLimitDuration := func() time.Duration {
		elapsed := time.Since(m.config.LastCheckCacheSizeTime)
		if elapsed < 0 {
			// now time < last check cache size time : last check cache size time (from config) is invalid
			return -1
		}
		return m.config.CleanIntervalCacheOverLimit - elapsed
	}

	if m.AutoClean {
		remaining := calcRemainingDuration()
		logger.Debugf("auto clean remaining duration: %v", remaining)
		if remaining < 0 {
			return doClean()
		}
		size, err := getNeedCleanCacheSize()
		if err != nil {
			logger.Warning(err)
			return err
		}
		cacheSize := size / 1000.0
		if cacheSize > MaxCacheSize {
			remainingCleanCacheOverLimitDuration := calcRemainingCleanCacheOverLimitDuration()
			logger.Debugf("clean cache over limit remaining duration: %v", remainingCleanCacheOverLimitDuration)
			if remainingCleanCacheOverLimitDuration < 0 {
				return doClean()
			}
		}
	} else {
		logger.Debug("auto clean disabled")
	}
	return nil
}

const lastoreJobCacheJson = "/tmp/lastoreJobCache.json"

func (m *Manager) canAutoQuit() bool {
	m.PropsMu.RLock()
	jobList := m.jobList
	m.PropsMu.RUnlock()
	haveActiveJob := false
	for _, job := range jobList {
		if (job.Status != system.FailedStatus || job.retry > 0) && job.Status != system.PausedStatus {
			logger.Info(job.Id)
			haveActiveJob = true
		}
	}
	m.autoQuitCountMu.Lock()
	inhibitAutoQuitCount := m.inhibitAutoQuitCount
	m.autoQuitCountMu.Unlock()
	logger.Info("haveActiveJob", haveActiveJob)
	logger.Info("inhibitAutoQuitCount", inhibitAutoQuitCount)
	logger.Info("upgrade status:", m.config.upgradeStatus.Status)
	return !haveActiveJob && inhibitAutoQuitCount == 0 && (m.config.upgradeStatus.Status == system.UpgradeReady)
}

type JobContent struct {
	Id   string
	Name string

	Packages     []string
	CreateTime   int64
	DownloadSize int64

	Type string

	Status system.Status

	Progress    float64
	Description string
	Environ     map[string]string
	// completed bytes per second
	QueueName string
	HaveNext  bool
}

// 读取上一次退出时失败和暂停的job,并导出
func (m *Manager) loadCacheJob() {
	var jobList []*JobContent
	jobContent, err := ioutil.ReadFile(lastoreJobCacheJson)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = json.Unmarshal(jobContent, &jobList)
	if err != nil {
		logger.Warning(err)
		return
	}
	for _, j := range jobList {
		switch j.Status {
		case system.FailedStatus:
			failedJob := NewJob(m.service, j.Id, j.Name, j.Packages, j.Type, j.QueueName, j.Environ)
			failedJob.Description = j.Description
			failedJob.CreateTime = j.CreateTime
			failedJob.DownloadSize = j.DownloadSize
			failedJob.Status = j.Status
			err = m.jobManager.addJob(failedJob)
			if err != nil {
				logger.Warning(err)
				continue
			}
		case system.PausedStatus:
			var updateType system.UpdateType
			var isClassified bool
			switch j.Id {
			case genJobId(system.PrepareSystemUpgradeJobType), genJobId(system.SystemUpgradeJobType):
				updateType = system.SystemUpdate
				isClassified = true
			case genJobId(system.PrepareSecurityUpgradeJobType), genJobId(system.SecurityUpgradeJobType):
				updateType = system.OnlySecurityUpdate
				isClassified = true
			case genJobId(system.PrepareUnknownUpgradeJobType), genJobId(system.UnknownUpgradeJobType):
				updateType = system.UnknownUpdate
				isClassified = true
			case genJobId(system.PrepareDistUpgradeJobType):
				updateType = m.CheckUpdateMode
				isClassified = false
			default: // lastore目前是对控制中心提供功能，任务暂停场景只有三种类型的分类更新（下载）和全量下载
				continue
			}
			if isClassified {
				_, err := m.classifiedUpgrade(dbus.Sender(m.service.Conn().Names()[0]), updateType, j.HaveNext)
				if err != nil {
					logger.Warning(err)
					return
				}
			} else {
				_, err := m.PrepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]))
				if err != nil {
					logger.Warning(err)
					return
				}
			}
			pausedJob := m.jobManager.findJobById(j.Id)
			if pausedJob != nil {
				pausedJob.PropsMu.Lock()
				err := m.jobManager.pauseJob(pausedJob)
				if err != nil {
					logger.Warning(err)
				}
				pausedJob.Progress = j.Progress
				pausedJob.PropsMu.Unlock()
			}

		default:
			continue
		}
	}
}

// 保存失败和暂停的job内容
func (m *Manager) saveCacheJob() {
	m.PropsMu.RLock()
	jobList := m.jobList
	m.PropsMu.RUnlock()

	var needSaveJobs []*JobContent
	for _, job := range jobList {
		if (job.Status == system.FailedStatus && job.retry == 0) || job.Status == system.PausedStatus {
			haveNext := false
			if job.next != nil {
				haveNext = true
			}
			needSaveJob := &JobContent{
				job.Id,
				job.Name,
				job.Packages,
				job.CreateTime,
				job.DownloadSize,
				job.Type,
				job.Status,
				job.Progress,
				job.Description,
				job.environ,
				job.queueName,
				haveNext,
			}
			needSaveJobs = append(needSaveJobs, needSaveJob)
		}
	}
	b, err := json.Marshal(needSaveJobs)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = ioutil.WriteFile(lastoreJobCacheJson, b, 0600)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) inhibitAutoQuitCountSub() {
	m.autoQuitCountMu.Lock()
	m.inhibitAutoQuitCount -= 1
	m.autoQuitCountMu.Unlock()
}

func (m *Manager) inhibitAutoQuitCountAdd() {
	m.autoQuitCountMu.Lock()
	m.inhibitAutoQuitCount += 1
	m.autoQuitCountMu.Unlock()
}

func (m *Manager) loadLastoreCache() {
	m.loadUpdateSourceOnce()
	m.loadCacheJob()
}

func (m *Manager) saveLastoreCache() {
	m.saveUpdateSourceOnce()
	m.saveCacheJob()
	m.userAgents.saveRecordContent(userAgentRecordPath)
}

func (m *Manager) handleOSSignal() {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGSEGV)

	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGSEGV:
			logger.Info("received signal:", sig)
			m.service.Quit()
		}
	}
}

func (m *Manager) watchSession(uid string, session login1.Session) {
	logger.Infof("watching '%s session:%s", uid, session.ServiceName_())
	session.InitSignalExt(m.signalLoop, true)
	err := session.Active().ConnectChanged(func(hasValue bool, active bool) {
		if !hasValue {
			return
		}
		if active {
			m.userAgents.setActiveUid(uid)
			// Active的用户切换后,语言环境切换至对应用户的语言环境,用于发通知
			gettext.SetLocale(gettext.LcAll, m.userAgents.getActiveLastoreAgentLang())
		}
	})

	if err != nil {
		logger.Warning(err)
	}

	active, err := session.Active().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	if active {
		m.userAgents.setActiveUid(uid)
		gettext.SetLocale(gettext.LcAll, m.userAgents.getActiveLastoreAgentLang())
	}
}

func (m *Manager) handleSessionNew(sessionId string, sessionPath dbus.ObjectPath) {
	logger.Info("session added", sessionId, sessionPath)
	sysBus := m.service.Conn()
	session, err := login1.NewSession(sysBus, sessionPath)
	if err != nil {
		logger.Warning(err)
		return
	}

	var userInfo login1.UserInfo
	userInfo, err = session.User().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	uidStr := strconv.Itoa(int(userInfo.UID))
	if !m.userAgents.hasUser(uidStr) {
		// 不关心这个用户的新 session,因为只有注册了agent的用户的user才需要监听
		return
	}

	newlyAdded := m.userAgents.addSession(uidStr, session)
	if newlyAdded {
		m.watchSession(uidStr, session)
	}
}

func (m *Manager) handleSessionRemoved(sessionId string, sessionPath dbus.ObjectPath) {
	logger.Info("session removed", sessionId, sessionPath)
	m.userAgents.removeSession(sessionPath)
}

func (m *Manager) handleUserRemoved(uid uint32, userPath dbus.ObjectPath) {
	uidStr := strconv.Itoa(int(uid))
	m.userAgents.removeUser(uidStr)
}

const (
	updateNotifyShow         = "dde-control-center"          // 无论控制中心状态，都需要发送的通知
	updateNotifyShowOptional = "dde-control-center-optional" // 根据控制中心更新模块焦点状态,选择性的发通知(由dde-session-daemon的lastore agent判断后控制)
)

func (m *Manager) sendNotify(appName string, replacesId uint32, appIcon string, summary string, body string, actions []string, hints map[string]dbus.Variant, expireTimeout int32) uint32 {
	if !m.updater.UpdateNotify {
		return 0
	}
	agent := m.userAgents.getActiveLastoreAgent()
	if agent != nil {
		id, err := agent.SendNotify(0, appName, replacesId, appIcon, summary, body, actions, hints, expireTimeout)
		if err != nil {
			logger.Warning(err)
		} else {
			return id
		}
	}
	return 0
}

func (m *Manager) closeNotify(id uint32) error {
	agent := m.userAgents.getActiveLastoreAgent()
	if agent != nil {
		err := agent.CloseNotification(0, id)
		if err != nil {
			logger.Warning(err)
		}
	}
	return nil
}

// reloadPrepareDistUpgradeJob 标记下载job状态为reload
func (m *Manager) reloadPrepareDistUpgradeJob() {
	// 标记job状态为reload
	prepareUpgradeTypeList := []string{
		system.PrepareDistUpgradeJobType,
		system.PrepareSystemUpgradeJobType,
		system.PrepareUnknownUpgradeJobType,
		system.PrepareSecurityUpgradeJobType,
	}
	for _, jobType := range prepareUpgradeTypeList {
		job := m.jobManager.findJobById(genJobId(jobType))
		if job != nil {
			job.needReload = true
		}
	}
}

func (m *Manager) afterUpdateModeChanged(change *dbusutil.PropertyChanged) {
	m.PropsMu.RLock()
	updateType := m.UpdateMode
	m.PropsMu.RUnlock()
	// UpdateMode修改后,一些对外属性需要同步修改(主要是和UpdateMode有关的数据)
	func() {
		updatableApps := m.updater.getUpdatablePackagesByType(updateType)
		m.updatableApps(updatableApps) // Manager的UpgradableApps实际为可更新的包,而非应用;
		m.updater.setUpdatablePackages(updatableApps)
		m.updater.updateUpdatableApps()
	}()
}

func (m *Manager) initDSettingsChangedHandle() {
	m.config.connectConfigChanged(dSettingsKeyLastoreDaemonStatus, func(bit lastoreDaemonStatus, value interface{}) {
		if bit == disableUpdate {
			_ = m.updateTimerUnit(lastoreOnline)
			_ = m.updateTimerUnit(lastoreAutoCheck)
			_ = m.updateTimerUnit(watchUpdateInfo)
		}
	})
}

func (m *Manager) initStatusManager() {
	logger.Info("start initStatusManager:", time.Now())
	startTime := time.Now()
	m.statusManager = NewStatusManager(m.config, func(newStatus string) {
		m.PropsMu.Lock()
		m.setPropUpdateStatus(newStatus)
		m.PropsMu.Unlock()
	})
	m.statusManager.RegisterChangedHandler(handlerKeyUpdateMode, func(value interface{}) {
		v := value.(system.UpdateType)
		m.PropsMu.Lock()
		m.setPropUpdateMode(v)
		m.PropsMu.Unlock()
	})
	m.statusManager.RegisterChangedHandler(handlerKeyCheckMode, func(value interface{}) {
		v := value.(system.UpdateType)
		m.PropsMu.Lock()
		m.setPropCheckUpdateMode(v)
		m.PropsMu.Unlock()
	})
	m.statusManager.InitModifyData()
	logger.Info("end initStatusManager duration:", time.Now().Sub(startTime))
}
