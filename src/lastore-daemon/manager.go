// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/strv"
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
	updatePlatform *UpdatePlatformManager
	isDownloading  bool

	offline            *OfflineManager
	rebootTimeoutTimer *time.Timer

	allUpgradableInfo map[system.UpdateType]map[string]system.PackageInfo
	allRemovePkgInfo  map[system.UpdateType]map[string]system.PackageInfo
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
	m.jobManager = NewJobManager(service, updateApi, m.updateJobList)
	m.offline = NewOfflineManager()
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
	// running 状态下证明需要进行重启后check
	if c.upgradeStatus.Status == system.UpgradeRunning {
		m.rebootTimeoutTimer = time.AfterFunc(600*time.Second, func() {
			// 启动后600s如果没有触发检查，那么上报更新失败
			m.updatePlatform.postStatusMessage(fmt.Sprintf("the check has not been triggered after reboot for 600 seconds"))
			err = delRebootCheckOption(all)
			if err != nil {
				logger.Warning(err)
			}
		})
	}
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
	logger.Info("initStatusManager cost:", time.Since(startTime))
}

func (m *Manager) initAgent() {
	m.userAgents = newUserAgentMap()
	m.userAgents.recoverLastoreAgents(m.service, m.handleSessionNew)
}

func (m *Manager) initPlatformManager() {
	m.updatePlatform = newUpdatePlatformManager(m.config, m.userAgents)
	m.loadPlatformCache()
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
	job.setPreHooks(map[string]func() error{
		string(system.SucceedStatus): func() error {
			msg := gettext.Tr("Removed successfully")
			go m.sendNotify(system.GetAppStoreAppName(), 0, "deepin-appstore", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
			return nil
		},
		string(system.FailedStatus): func() error {
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
			go m.sendNotify(system.GetAppStoreAppName(), 0, "deepin-appstore", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
			return nil
		},
	})
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
	for _, t := range system.AllCheckUpdateType() {
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
				upgradeJob, err = m.distUpgrade(sender, category, true, false, false)
				if err != nil && !errors.Is(err, JobExistError) {
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
	job.setPreHooks(map[string]func() error{
		string(system.EndStatus): func() error {
			// 清理完成的通知
			msg := gettext.Tr("Package cache wiped")
			go m.sendNotify(updateNotifyShow, 0, "deepin-appstore", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
			return nil
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

	switch system.JobErrorType(errType) {
	case system.ErrorDpkgInterrupted, system.ErrorDependenciesBroken:
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
	logger.Infof("session added %v %v", sessionId, sessionPath)
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
		logger.Infof("no this user %v,don't need add session", uidStr)
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
	logger.Info("user removed", uid, userPath)
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

// ChangePrepareDistUpgradeJobOption 当下载job的配置需要修改,通过该接口触发
func (m *Manager) ChangePrepareDistUpgradeJobOption() {
	logger.Info("start changed download job option by ForceAbortAndRetry")
	prepareUpgradeTypeList := []string{
		system.PrepareDistUpgradeJobType,
		system.PrepareSystemUpgradeJobType,
		system.PrepareUnknownUpgradeJobType,
		system.PrepareSecurityUpgradeJobType,
	}
	for _, jobType := range prepareUpgradeTypeList {
		job := m.jobManager.findJobById(genJobId(jobType))
		if job != nil {
			if job.Status == system.PausedStatus {
				m.handleDownloadLimitChanged(job)
			} else {
				err := m.jobManager.ForceAbortAndRetry(job)
				if err != nil {
					logger.Warning(err)
				}
			}
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

func (m *Manager) handleDownloadLimitChanged(job *Job) {
	limitEnable, limitConfig := m.updater.GetLimitConfig()
	if limitEnable {
		if job.option == nil {
			job.option = make(map[string]string)
		}
		job.option[aptLimitKey] = limitConfig
	} else {
		if job.option != nil {
			delete(job.option, aptLimitKey)
		}
	}
}

func (m *Manager) installSpecialPackageSync(pkgName string, option map[string]string, environ map[string]string) {
	if strv.Strv(m.updater.UpdatablePackages).Contains(pkgName) || system.QueryPackageInstallable(pkgName) {
		// 该包可更新或者该包未安装可以安装
		var wg sync.WaitGroup
		wg.Add(1)

		m.do.Lock()
		defer m.do.Unlock()
		isExist, installJob, err := m.jobManager.CreateJob(fmt.Sprintf("install %v", pkgName), system.OnlyInstallJobType, []string{pkgName}, environ, nil)
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
			installJob.option = option
			installJob.setPreHooks(map[string]func() error{
				string(system.FailedStatus): func() error {
					wg.Done()
					return nil
				},
				string(system.SucceedStatus): func() error {
					wg.Done()
					return nil
				},
			})
			if err := m.jobManager.addJob(installJob); err != nil {
				logger.Warning(err)
				wg.Done()
				return
			}
		}
		wg.Wait()
	}
}

type platformCacheContent struct {
	UpgradableInfo map[system.UpdateType]map[string]system.PackageInfo
	RemovePkgInfo  map[system.UpdateType]map[string]system.PackageInfo
	CoreListPkgs   map[string]system.PackageInfo
	BaselinePkgs   map[string]system.PackageInfo
	SelectPkgs     map[string]system.PackageInfo
	PreCheck       string
	MidCheck       string
	PostCheck      string
}

func (m *Manager) savePlatformCache() {
	cache := platformCacheContent{}
	cache.UpgradableInfo = m.allUpgradableInfo
	cache.RemovePkgInfo = m.allRemovePkgInfo
	cache.CoreListPkgs = m.updatePlatform.targetCorePkgs
	cache.BaselinePkgs = m.updatePlatform.baselinePkgs
	cache.SelectPkgs = m.updatePlatform.selectPkgs
	cache.PreCheck = m.updatePlatform.preCheck
	cache.MidCheck = m.updatePlatform.midCheck
	cache.PostCheck = m.updatePlatform.postCheck
	content, err := json.Marshal(cache)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = m.config.SetOnlineCache(string(content))
	if err != nil {
		logger.Warning(err)
		return
	}
}

func (m *Manager) loadPlatformCache() {
	cache := platformCacheContent{}
	err := json.Unmarshal([]byte(m.config.onlineCache), &cache)
	if err != nil {
		logger.Warning(err)
		return
	}
	m.allUpgradableInfo = cache.UpgradableInfo
	m.allRemovePkgInfo = cache.RemovePkgInfo
	m.updatePlatform.targetCorePkgs = cache.CoreListPkgs
	m.updatePlatform.baselinePkgs = cache.BaselinePkgs
	m.updatePlatform.selectPkgs = cache.SelectPkgs
	m.updatePlatform.preCheck = cache.PreCheck
	m.updatePlatform.midCheck = cache.MidCheck
	m.updatePlatform.postCheck = cache.PostCheck
}
