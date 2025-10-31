// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/polkit"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	abrecovery "github.com/linuxdeepin/go-dbus-factory/system/com.deepin.abrecovery"
	accounts "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.accounts1"
	power "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/utils"
)

const (
	UserExperServiceName = "com.deepin.userexperience.Daemon"
	UserExperPath        = "/com/deepin/userexperience/Daemon"

	UserExperInstallApp   = "installapp"
	UserExperUninstallApp = "uninstallapp"
)

const MaxCacheSize = 500.0 //size MB

type Manager struct {
	service   *dbusutil.Service
	do        sync.Mutex
	updateApi system.System
	config    *config.Config
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

	sysPower   power.Power
	signalLoop *dbusutil.SignalLoop

	UpdateMode      system.UpdateType `prop:"access:rw"` // 更新设置的内容
	CheckUpdateMode system.UpdateType `prop:"access:rw"` // 检查更新选中的内容
	UpdateStatus    string            // 每一个更新项的状态 json字符串

	// 无忧还原是否开启的状态
	ImmutableAutoRecovery bool `prop:"access:r"`

	HardwareId string

	systemSourceConfig   UpdateSourceConfig
	securitySourceConfig UpdateSourceConfig

	inhibitAutoQuitCount int32
	autoQuitCountMu      sync.Mutex
	lastoreUnitCacheMu   sync.Mutex

	loginManager  login1.Manager
	sysDBusDaemon ofdbus.DBus
	systemd       systemd1.Manager
	abObj         abrecovery.ABRecovery

	grub             *grubManager
	userAgents       *userAgentMap // 闲时退出时，需要保存数据，启动时需要根据uid,agent sender以及session path完成数据恢复
	statusManager    *UpdateModeStatusManager
	updatePlatform   *updateplatform.UpdatePlatformManager
	immutableManager *immutableManager

	rebootTimeoutTimer *time.Timer

	coreList   []string
	updateTime string // 定时时间，记录定时更新通知，防止重复发通知

	checkDpkgCapabilityOnce sync.Once
	supportDpkgScriptIgnore bool

	logFds     []*os.File
	logFdsMu   sync.Mutex
	logTmpFile *os.File
}

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func NewManager(service *dbusutil.Service, updateApi system.System, c *config.Config) *Manager {
	archs, err := system.SystemArchitectures()
	if err != nil {
		logger.Errorf("Can't detect system supported architectures %v\n", err)
		return nil
	}

	m := &Manager{
		service:              service,
		config:               c,
		updateApi:            updateApi,
		SystemArchitectures:  archs,
		inhibitFd:            -1,
		AutoClean:            c.AutoClean,
		loginManager:         login1.NewManager(service.Conn()),
		sysDBusDaemon:        ofdbus.NewDBus(service.Conn()),
		signalLoop:           dbusutil.NewSignalLoop(service.Conn(), 10),
		systemd:              systemd1.NewManager(service.Conn()),
		sysPower:             power.NewPower(service.Conn()),
		abObj:                abrecovery.NewABRecovery(service.Conn()),
		securitySourceConfig: make(UpdateSourceConfig),
		systemSourceConfig:   make(UpdateSourceConfig),
	}
	m.reloadOemConfig(true)
	m.signalLoop.Start()
	m.grub = newGrubManager(service.Conn(), m.signalLoop)
	m.jobManager = NewJobManager(service, updateApi, m.updateJobList, m.processLogFds)
	m.immutableManager = newImmutableManager(m.jobManager.handleJobProgressInfo)
	go m.handleOSSignal()
	m.updateJobList()
	m.initStatusManager()
	m.HardwareId = updateplatform.GetHardwareId(m.config.IncludeDiskInfo)

	m.initDbusSignalListen()
	m.initDSettingsChangedHandle()
	m.syncThirdPartyDconfig()
	m.updateAutoRecoveryStatus()
	// running 状态下证明需要进行重启后check
	if c.UpgradeStatus.Status == system.UpgradeRunning {
		m.rebootTimeoutTimer = time.AfterFunc(600*time.Second, func() {
			// 启动后600s如果没有触发检查，那么上报更新失败

			m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
				Type:   "error",
				Detail: "the check has not been triggered after reboot for 600 seconds",
			})

			err = m.delRebootCheckOption(all)
			if err != nil {
				logger.Warning(err)
			}
		})
	}
	return m
}

func (m *Manager) initDbusSignalListen() {
	m.loginManager.InitSignalExt(m.signalLoop, true)
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
			// 当lastore-daemon启动时还没初始化完成，刚好收到NameOwnerChanged，导致崩溃
			if m.userAgents != nil {
				m.userAgents.handleNameLost(name)
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	m.sysPower.InitSignalExt(m.signalLoop, true)
}

func (m *Manager) initDSettingsChangedHandle() {
	m.config.ConnectConfigChanged(config.DSettingsKeyLastoreDaemonStatus, func(bit config.LastoreDaemonStatus, value interface{}) {
		if bit == config.DisableUpdate {
			_ = m.updateTimerUnit(lastoreAutoCheck)
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
	sessions, err := m.loginManager.ListSessions(0)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, session := range sessions {
			m.handleSessionNew("", session.Path)
		}
	}
}

func (m *Manager) initPlatformManager() {
	m.updatePlatform = updateplatform.NewUpdatePlatformManager(m.config, false)
	if isFirstBoot() {
		// 不能阻塞初始化流程,防止dbus服务激活超时
		go m.updatePlatform.RetryPostHistory() // 此处调用还没有export以及dispatch job,因此可以判断是否需要check.
	}
}

func (m *Manager) delUpdatePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
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

func (m *Manager) delInstallPackageFromRepo(sender dbus.Sender, jobName string, sourceListPath string,
	repoListPath string, cachePath string, packageName []string) (*Job, error) {
	if !utils.IsDir(repoListPath) {
		return nil, fmt.Errorf("illegal repoListPath: %v", repoListPath)
	}
	if !utils.IsDir(cachePath) {
		return nil, fmt.Errorf("illegal cachePath: %v", cachePath)
	}

	var job *Job
	var isExist bool
	var err error

	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, fmt.Errorf("make environ failed: %v", err)
	}

	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		return nil, fmt.Errorf("get conn uid failed: %v", err)
	}

	if uid != 0 {
		err := polkit.CheckAuth(polkitActionChangeOwnData, string(sender), nil)
		if err != nil {
			return nil, fmt.Errorf("check authorization failed: %v", err)
		}
	}

	var canNotInstallError = errors.New("unable to install packages now")
	_, isLock := system.CheckLock("/var/lib/dpkg/lock")
	if isLock {
		return nil, canNotInstallError
	}
	_, isLock = system.CheckLock("/var/lib/dpkg/lock-frontend")
	if isLock {
		return nil, canNotInstallError
	}

	m.do.Lock()
	defer m.do.Unlock()
	isExist, job, err = m.jobManager.CreateJob(jobName, system.OnlyInstallJobType, packageName, environ, nil)
	if err != nil {
		return nil, fmt.Errorf("create job failed: %v, jobname: %v", err, jobName)
	}
	if isExist {
		return job, nil
	}

	if utils.IsDir(sourceListPath) {
		job.option = map[string]string{
			"Dir::Etc::SourceList":  "/dev/null",
			"Dir::Etc::SourceParts": sourceListPath,
		}
	} else {
		job.option = map[string]string{
			"Dir::Etc::SourceList":  sourceListPath,
			"Dir::Etc::SourceParts": "/dev/null",
		}
	}
	job.option["Dir::State::lists"] = repoListPath
	job.option["Dir::Cache::archives"] = cachePath

	if err = m.jobManager.addJob(job); err != nil {
		return nil, fmt.Errorf("add job failed: %v", err)
	}

	return job, nil
}

func (m *Manager) installPkg(jobName, packages string, environ map[string]string) (*Job, error) {
	pList := strings.Fields(packages)
	var job *Job
	var isExist bool
	var err error
	err = system.CustomSourceWrapper(system.AllCheckUpdate, func(path string, unref func()) error {
		m.do.Lock()
		defer m.do.Unlock()
		isExist, job, err = m.jobManager.CreateJob(jobName, system.InstallJobType, pList, environ, nil)
		if err != nil {
			logger.Warningf("installPackage %q error: %v\n", packages, err)
			if unref != nil {
				unref()
			}
			return err
		}
		if isExist {
			if unref != nil {
				unref()
			}
			logger.Info(JobExistError)
			return JobExistError
		}
		// 设置apt命令参数

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
		if job.next != nil {
			job.next.option = job.option
			job.next.setPreHooks(map[string]func() error{
				string(system.EndStatus): func() error {
					// wrapper的资源释放
					if unref != nil {
						unref()
					}
					return nil
				},
			})
		}

		if err = m.jobManager.addJob(job); err != nil {
			logger.Warning(err)
			if unref != nil {
				unref()
			}
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, JobExistError) { // exist的err无需返回
		logger.Warning(err)
		return nil, err
	}
	return job, nil
}

func (m *Manager) removePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
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
				"x-deepin-action-retry":  dbus.MakeVariant(fmt.Sprintf("dbus-send,--system,--print-reply,--dest=org.deepin.dde.Lastore1,/org/deepin/dde/Lastore1,org.deepin.dde.Lastore1.Manager.StartJob,string:%s", job.Id)),
				"x-deepin-action-cancel": dbus.MakeVariant(fmt.Sprintf("dbus-send,--system,--print-reply,--dest=org.deepin.dde.Lastore1,/org/deepin/dde/Lastore1,org.deepin.dde.Lastore1.Manager.CleanJob,string:%s", job.Id))}
			go m.sendNotify(system.GetAppStoreAppName(), 0, "deepin-appstore", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
			return nil
		},
	})
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	return job, nil
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
			go m.sendNotify(updateNotifyShow, 0, "", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
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

func (m *Manager) delFixError(sender dbus.Sender, errType string) (*Job, error) {
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
	// 调用者判断
	err := checkInvokePermission(m.service, pw.Sender)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	writeMode := system.UpdateType(pw.Value.(uint64))
	newMode := m.statusManager.SetUpdateMode(writeMode)
	pw.Value = newMode
	return nil
}

func (m *Manager) syncThirdPartyDconfig() {
	const (
		dccDsettingsId         = "org.deepin.dde.control-center"
		dccUpdateDsettingsName = "org.deepin.dde.control-center.update"
		dccKeyThirdPartySource = "updateThirdPartySource"
	)
	ds := ConfigManager.NewConfigManager(m.service.Conn())
	dsPath, err := ds.AcquireManager(0, dccDsettingsId, dccUpdateDsettingsName, "")
	if err != nil {
		logger.Warning(err)
		return
	}
	dsDCCManager, err := ConfigManager.NewManager(m.service.Conn(), dsPath)
	if err != nil {
		logger.Warning(err)
		return
	}
	systemSigLoop := dbusutil.NewSignalLoop(m.service.Conn(), 10)
	systemSigLoop.Start()
	dsDCCManager.InitSignalExt(systemSigLoop, true)
	v, err := dsDCCManager.Value(0, dccKeyThirdPartySource)
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Info("updateThirdPartySource is ", v.Value().(string))

	syncUpdateMode := func(enable string) {
		if enable == "Hidden" {
			newMode := m.UpdateMode & (^system.UnknownUpdate)
			m.statusManager.SetUpdateMode(newMode)
		}
	}
	syncUpdateMode(v.Value().(string))
	_, err = dsDCCManager.ConnectValueChanged(func(key string) {
		switch key {
		case "updateThirdPartySource":
			v, err := dsDCCManager.Value(0, dccKeyThirdPartySource)
			if err != nil {
				logger.Warning(err)
				return
			}
			syncUpdateMode(v.Value().(string))
		}
	})
}

func (m *Manager) checkUpdateModeWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	// 调用者判断
	err := checkInvokePermission(m.service, pw.Sender)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

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
	if m.config.AutoCheckUpdates && !m.ImmutableAutoRecovery {
		_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]))
		if err != nil {
			logger.Warning(err)
			return err
		}
	}
	if !m.config.DisableUpdateMetadata && !m.ImmutableAutoRecovery {
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
			lang := m.userAgents.getActiveLastoreAgentLang()
			if len(lang) != 0 {
				// Active的用户切换后,语言环境切换至对应用户的语言环境,用于发通知
				logger.Info("SetLocale", lang)
				gettext.SetLocale(gettext.LcAll, lang)
			} else {
				m.updateLocaleByUser(uid)
			}
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
		lang := m.userAgents.getActiveLastoreAgentLang()
		if len(lang) != 0 {
			logger.Info("SetLocale", lang)
			gettext.SetLocale(gettext.LcAll, lang)
		} else {
			m.updateLocaleByUser(uid)
		}
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
	sessionType, err := session.Type().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	if sessionType == "tty" {
		logger.Infof("%v session is tty", sessionPath)
		return
	}

	var userInfo login1.UserInfo
	userInfo, err = session.User().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	uidStr := strconv.Itoa(int(userInfo.UID))

	newlyAdded := m.userAgents.addSession(uidStr, session)
	if newlyAdded {
		m.watchSession(uidStr, session)
	}
}

func (m *Manager) handleSessionRemoved(sessionId string, sessionPath dbus.ObjectPath) {
	logger.Info("session removed", sessionId, sessionPath)
	m.userAgents.removeSession(sessionPath)
}

func (m *Manager) updateLocaleByUser(uid string) {
	logger.Info("update locale by user", uid)
	obj := accounts.NewAccounts(m.service.Conn())
	path, err := obj.FindUserById(0, uid)
	if err != nil {
		logger.Warning(err)
		return
	}
	user, err := accounts.NewUser(m.service.Conn(), dbus.ObjectPath(path))
	if err != nil {
		logger.Warning(err)
		return
	}
	locale, err := user.Locale().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Info("SetLocale", locale)
	gettext.SetLocale(gettext.LcAll, locale)
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
	} else {
		users, err := m.loginManager.ListUsers(0)
		if err != nil {
			logger.Warning(err)
		}
		app := appName
		if app == updateNotifyShowOptional {
			app = updateNotifyShow
		}
		actionsArg := "["
		if len(actions) != 0 {
			actionsArg = actionsArg + `"` + strings.Join(actions, `","`) + `"`
		}
		actionsArg = actionsArg + "]"

		var hintsList []string
		for key, value := range hints {
			hintsList = append(hintsList, `"`+key+`":<"`+value.Value().(string)+`">`)
		}
		hintsArg := "{" + strings.Join(hintsList, `,`) + "}"
		timeout := expireTimeout
		if timeout < 0 {
			timeout = 5000 // -1: default 5s
		}
		args := []string{
			"gdbus", "call", "--session", "--dest=org.freedesktop.Notifications", "--object-path=/org/freedesktop/Notifications", "--method=org.freedesktop.Notifications.Notify",
			`'` + app + `'`, strconv.FormatUint(uint64(replacesId), 10), `'` + appIcon + `'`, `'` + summary + `'`, `'` + body + `'`, actionsArg, hintsArg, strconv.FormatInt(int64(timeout), 10),
		}
		var id uint32 = 0
		for _, user := range users {
			username := user.Name
			uid := user.UID
			if m.userAgents.activeUid != strconv.Itoa(int(uid)) {
				continue
			}
			cmdArgs := []string{
				"-u", username, "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + strconv.Itoa(int(uid)) + "/bus",
			}
			// 安全地添加参数，避免命令注入
			cmdArgs = append(cmdArgs, args...)
			cmd := exec.Command("sudo", cmdArgs...)
			logger.Info(cmd.String())
			var outBuffer bytes.Buffer
			var errBuffer bytes.Buffer
			cmd.Stderr = &errBuffer
			cmd.Stdout = &outBuffer
			err = cmd.Run()
			if err != nil {
				logger.Warning(err)
				logger.Warning(errBuffer.String())
			} else {
				str := outBuffer.String()
				if len(str) >= 12 {
					// 确保解析的字符串是有效的数字
					numStr := str[8 : len(str)-3]
					num, err := strconv.ParseUint(numStr, 10, 32)
					if err != nil {
						logger.Warning(err)
					} else {
						id = uint32(num)
					}
				}
			}
			break
		}
		return id
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
	} else {
		users, err := m.loginManager.ListUsers(0)
		if err != nil {
			logger.Warning(err)
		}
		args := []string{
			"gdbus", "call", "--session", "--dest=org.freedesktop.Notifications", "--object-path=/org/freedesktop/Notifications", "--method=org.freedesktop.Notifications.CloseNotification",
			strconv.FormatUint(uint64(id), 10),
		}
		for _, user := range users {
			username := user.Name
			uid := user.UID
			if m.userAgents.activeUid != strconv.Itoa(int(uid)) {
				continue
			}
			cmdArgs := []string{
				"-u", username, "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + strconv.Itoa(int(uid)) + "/bus",
			}
			// 安全地添加参数，避免命令注入
			cmdArgs = append(cmdArgs, args...)
			cmd := exec.Command("sudo", cmdArgs...)
			logger.Info(cmd.String())
			var errBuffer bytes.Buffer
			cmd.Stderr = &errBuffer
			err = cmd.Run()
			if err != nil {
				logger.Warning(err)
				logger.Warning(errBuffer.String())
			}
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

// 只有初始化和检查更新的时候，才能更新系统和安全仓库的Dir，目的是保证检查、下载、安装过程中的一致性，不受配置修改的影响
func (m *Manager) reloadOemConfig(reloadSourceDir bool) {
	// 更新仓库Dir
	if reloadSourceDir {
		m.config.ReloadSourcesDir()
	}

	// 更新 dbus 属性
	InitConfig(m.systemSourceConfig, m.config.SystemOemSourceConfig, m.config.SystemCustomSource)
	InitConfig(m.securitySourceConfig, m.config.SecurityOemSourceConfig, m.config.SecurityCustomSource)
	SetUsingRepoType(m.systemSourceConfig, m.config.SystemRepoType)
	SetUsingRepoType(m.securitySourceConfig, m.config.SecurityRepoType)
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

// 数据埋点接口
func (m *Manager) reportLog(category reportCategory, status bool, description string) {
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
}

// 检查无忧还原状态，只需要启动时检查一次
func (m *Manager) updateAutoRecoveryStatus() {
	bootedFile := "/run/deepin-immutable-writable/booted"
	bootedContent, err := os.ReadFile(bootedFile)
	if err != nil {
		logger.Warning(err)
		return
	}

	isEnabled := string(bootedContent) == "overlay"
	if isEnabled {
		logger.Info("immutable auto recovery is enabled")
		// 在无忧还原模式下，主动停止相关定时器
		_ = m.stopTimerUnit(lastoreAutoCheck)
	} else {
		logger.Info("immutable auto recovery is disabled")
	}

	m.PropsMu.Lock()
	m.setPropImmutableAutoRecovery(isEnabled)
	m.PropsMu.Unlock()
}

const (
	logTmpPath = "/run/lastore/lastore_update_detail.log"
)

// processLogFds 遍历logFds，检查fd有效性，写入消息，移除无效的fd
func (m *Manager) processLogFds(message string) {
	m.logFdsMu.Lock()
	defer m.logFdsMu.Unlock()
	// 将 message 所有内容汇总不断的追加到 logTmpPath 文件中
	err := func() error {
		if m.logTmpFile != nil {
			_, err := m.logTmpFile.WriteString(message)
			if err != nil {
				return fmt.Errorf("failed to write to file %s: %v", logTmpPath, err)
			}
		}
		return nil
	}()
	if err != nil {
		logger.Warning(err)
	}

	validFds := make([]*os.File, 0, len(m.logFds))
	for _, file := range m.logFds {
		if file == nil {
			continue
		}

		// 尝试写入字符串，如果成功说明fd有效
		_, err := file.WriteString(message)
		if err != nil {
			// 写入失败，fd无效，关闭文件并跳过
			file.Close()
			logger.Debugf("Removed invalid log fd due to write error: %v", err)
			continue
		}

		// 写入成功，保留这个有效的fd
		validFds = append(validFds, file)
	}

	// 更新logFds，只保留有效的fd
	m.logFds = validFds
}
