// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
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
	grub2 "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.grub2"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/strv"
)

const (
	appStoreDaemonPath    = "/usr/bin/deepin-app-store-daemon"
	oldAppStoreDaemonPath = "/usr/bin/deepin-appstore-daemon"
	printerPath           = "/usr/bin/dde-printer"
	printerHelperPath     = "/usr/bin/dde-printer-helper"
	sessionDaemonPath     = "/usr/lib/deepin-daemon/dde-session-daemon"
	langSelectorPath      = "/usr/lib/deepin-daemon/langselector"
	controlCenterPath     = "/usr/bin/dde-control-center"
	controlCenterCmdLine  = "/usr/share/applications/dde-control-center.deskto" // 缺个 p 是因为 deepin-turbo 修改命令的时候 buffer 不够用, 所以截断了.
)

var (
	allowInstallPackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		printerPath,
		printerHelperPath,
		langSelectorPath,
		controlCenterPath,
	}
	allowRemovePackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		sessionDaemonPath,
		langSelectorPath,
		controlCenterPath,
	}
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

	UpdateMode system.UpdateType `prop:"access:rw"`
	HardwareId string

	isUpdateSucceed bool

	inhibitAutoQuitCount int32
	autoQuitCountMu      sync.Mutex
	lastoreUnitCacheMu   sync.Mutex

	userAgents    *userAgentMap // 闲时退出时，需要保存数据，启动时需要根据uid,agent sender以及session path完成数据恢复
	loginManager  login1.Manager
	sysDBusDaemon ofdbus.DBus
	systemd       systemd1.Manager
	grub          grub2.Grub2
	abObj         abrecovery.ABRecovery
	isDownloading bool
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
		UpdateMode:          c.UpdateMode,
		userAgents:          newUserAgentMap(service),
		loginManager:        login1.NewManager(service.Conn()),
		sysDBusDaemon:       ofdbus.NewDBus(service.Conn()),
		signalLoop:          dbusutil.NewSignalLoop(service.Conn(), 10),
		apps:                apps.NewApps(service.Conn()),
		systemd:             systemd1.NewManager(service.Conn()),
		grub:                grub2.NewGrub2(service.Conn()),
		sysPower:            power.NewPower(service.Conn()),
		abObj:               abrecovery.NewABRecovery(service.Conn()),
	}
	m.signalLoop.Start()
	m.jobManager = NewJobManager(service, updateApi, m.updateJobList)
	go m.handleOSSignal()
	m.updateJobList()
	m.modifyUpdateMode()
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
	m.grub.InitSignalExt(m.signalLoop, true)
	m.sysPower.InitSignalExt(m.signalLoop, true)
}

// execPath和cmdLine可以有一个为空,其中一个存在即可作为判断调用者的依据
func (m *Manager) getExecutablePathAndCmdline(sender dbus.Sender) (string, string, error) {
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		return "", "", err
	}

	proc := procfs.Process(pid)

	execPath, err := proc.Exe()
	if err != nil {
		// 当调用者在使用过程中发生了更新,则在获取该进程的exe时,会出现lstat xxx (deleted)此类的error,如果发生的是覆盖,则该路径依旧存在,因此增加以下判断
		pErr, ok := err.(*os.PathError)
		if ok {
			if os.IsNotExist(pErr.Err) {
				errExecPath := strings.Replace(pErr.Path, "(deleted)", "", -1)
				oldExecPath := strings.TrimSpace(errExecPath)
				if system.NormalFileExists(oldExecPath) {
					execPath = oldExecPath
					err = nil
				}
			}
		}
	}

	cmdLine, err1 := proc.Cmdline()
	if err != nil && err1 != nil {
		return "", "", errors.New(strings.Join([]string{
			err.Error(),
			err1.Error(),
		}, ";"))
	}
	return execPath, strings.Join(cmdLine, " "), nil
}

func (m *Manager) updatePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	execPath, cmdLine, err := m.getExecutablePathAndCmdline(sender)
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
	isExist, job, err := m.jobManager.CreateJob(jobName, system.UpdateJobType, pkgs, environ)
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
	isExist, job, err := m.jobManager.CreateJob(jobName, system.InstallJobType, pList, environ)
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

func (m *Manager) needPostSystemUpgradeMessage() bool {
	return strv.Strv(m.config.AllowPostSystemUpgradeMessageVersion).Contains(getEditionName())
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
	isExist, job, err := m.jobManager.CreateJob(jobName, system.RemoveJobType, pkgs, environ)
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
	return job, err
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
	err = m.config.UpdateLastCheckTime()
	if err != nil {
		logger.Warning(err)
		return
	}
	err = m.updateAutoCheckSystemUnit()
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleUpdateInfosChanged() {
	logger.Info("handleUpdateInfosChanged")
	infosMap, err := m.SystemUpgradeInfo()
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info(err) // 移除文件时,同样会进入该逻辑,因此在update_infos.json文件不存在时,将日志等级改为info
		} else {
			logger.Error("failed to get upgrade info:", err)
		}
		return
	}
	m.updateUpdatableProp(infosMap)
	// 检查更新时,同步修改待下载数据大小
	go func() {
		m.PropsMu.RLock()
		packages := m.UpgradableApps
		m.PropsMu.RUnlock()
		size, err := system.QueryPackageDownloadSize(false, packages...)
		if err != nil {
			logger.Warning(err)
		}
		err = m.config.UpdateLastoreDaemonStatus(canUpgrade, size == 0 && len(packages) != 0)
		if err != nil {
			logger.Warning(err)
		}
	}()
	m.PropsMu.Lock()
	isUpdateSucceed := m.isUpdateSucceed
	m.PropsMu.Unlock()
	m.updater.PropsMu.RLock()
	enable := m.updater.idleDownloadConfigObj.IdleDownloadEnabled
	m.updater.PropsMu.RUnlock()
	if m.updater.AutoDownloadUpdates && len(m.updater.UpdatablePackages) > 0 && isUpdateSucceed && !enable {
		logger.Info("auto download updates")
		go func() {
			m.inhibitAutoQuitCountAdd()
			_, err = m.PrepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0])) // 自动下载使用控制中心的配置
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
	filterInfos := m.getFilterInfosMap(infosMap)
	updatableApps := UpdatableNames(filterInfos)
	m.updatableApps(updatableApps) // Manager的UpgradableApps实际为可更新的包,而非应用;
	m.updater.setUpdatablePackages(updatableApps)
	m.updater.updateUpdatableApps()
}

// ClassifiedUpdatablePackages属性保存所有数据,UpdatablePackages属性保存过滤后的数据
func (m *Manager) getFilterInfosMap(infosMap system.SourceUpgradeInfoMap) system.SourceUpgradeInfoMap {
	r := make(system.SourceUpgradeInfoMap)
	m.PropsMu.RLock()
	updateType := m.UpdateMode
	m.PropsMu.RUnlock()
	for _, t := range system.AllUpdateType() {
		category := updateType & t
		if category != 0 {
			info, ok := infosMap[t.JobType()]
			if ok {
				r[t.JobType()] = info
			}
		}
	}
	return r
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
	var jobName string
	if needNotify {
		jobName = "+notify"
	}
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	prepareUpdateSource()
	m.jobManager.dispatch() // 解决 bug 59351问题（防止CreatJob获取到状态为end但是未被删除的job）
	isExist, job, err := m.jobManager.CreateJob(jobName, system.UpdateSourceJobType, nil, environ)
	if err != nil {
		logger.Warningf("UpdateSource error: %v\n", err)
		return nil, err
	}
	if isExist {
		return job, nil
	}
	if err := m.jobManager.addJob(job); err != nil {
		return nil, err
	}
	if job != nil {
		m.PropsMu.Lock()
		m.updateSourceOnce = true
		m.isUpdateSucceed = false
		m.PropsMu.Unlock()
		job.setHooks(map[string]func(){
			string(system.SucceedStatus): func() {
				m.PropsMu.Lock()
				m.isUpdateSucceed = true
				m.PropsMu.Unlock()
				m.handleUpdateInfosChanged()
				if len(m.UpgradableApps) > 0 {
					m.reportLog(updateStatus, true, "")
					// 开启自动下载时触发自动下载,发自动下载通知
					// 关闭自动下载且为自动触发时,发可更新的通知
					if !m.updater.AutoDownloadUpdates && sender == dbus.Sender(m.service.Conn().Names()[0]) {
						msg := gettext.Tr("Updates Available")
						action := []string{"update", gettext.Tr("Update Now")}
						hints := map[string]dbus.Variant{"x-deepin-action-update": dbus.MakeVariant("dde-control-center,-m,update")}
						m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				} else {
					m.reportLog(updateStatus, false, "")
				}
			},
			string(system.FailedStatus): func() {
				m.handleUpdateInfosChanged()
				m.reportLog(updateStatus, false, job.Description)
			},
		})
	}
	return job, err
}

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
func (m *Manager) distUpgrade(sender dbus.Sender, mode system.UpdateType, isClassify bool) (*Job, error) {
	execPath, cmdLine, err := m.getExecutablePathAndCmdline(sender)
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

	if isClassify {
		m.updater.PropsMu.RLock()
		classifiedUpdatablePackagesMap := m.updater.ClassifiedUpdatablePackages
		m.updater.PropsMu.RUnlock()
		if len(classifiedUpdatablePackagesMap[mode.JobType()]) == 0 {
			return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", mode.JobType()))
		}
	} else {
		m.PropsMu.RLock()
		upgradableApps := m.UpgradableApps
		m.PropsMu.RUnlock()
		if len(upgradableApps) == 0 {
			return nil, system.NotFoundError("empty UpgradableApps")
		}
	}

	var job *Job
	var isExist bool
	err = system.CustomSourceWrapper(mode, func(path string, unref func()) error {
		m.do.Lock()
		defer m.do.Unlock()
		if isClassify {
			jobType := GetUpgradeInfoMap()[mode].UpgradeJobId
			isExist, job, err = m.jobManager.CreateJob("", jobType, nil, environ)
		} else {
			isExist, job, err = m.jobManager.CreateJob("", system.DistUpgradeJobType, nil, environ)
		}
		if err != nil {
			logger.Warningf("DistUpgrade error: %v\n", err)
			if unref != nil {
				unref()
			}
			return err
		}
		if isExist {
			return nil
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
		if isLaptop && err == nil {
			var hasSendNotify bool
			var batteryPercentageNotify sync.Once
			var lowPowerNotifyId uint32 = 0
			var needConnectNotifyId uint32 = 0
			onBatteryGlobal, _ := m.sysPower.OnBattery().Get(0)
			batteryPercentage, _ := m.sysPower.BatteryPercentage().Get(0)
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
			handleSysPowerBatteryEvent := func() {
				if onBatteryGlobal && batteryPercentage <= 60.0 && (job.Status == system.RunningStatus || job.Status == system.ReadyStatus) {
					batteryPercentageNotify.Do(func() {
						msg := gettext.Tr("The battery level is lower than 60%, please charge it")
						lowPowerNotifyId = m.sendNotify("dde-control-center", 0, "notification-battery_low", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
						hasSendNotify = true
					})
				}

				if onBatteryGlobal && !hasSendNotify {
					// TODO 翻译
					msg := gettext.Tr("需要在1分钟内连接电源")
					needConnectNotifyId = m.sendNotify("dde-control-center", 0, "TBD-battery_low", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
				}
				// 用户连上电源时,需要关闭通知,并重置Once
				if !onBatteryGlobal {
					if hasSendNotify {
						err = m.closeNotify(lowPowerNotifyId)
						if err != nil {
							logger.Warning(err)
						}
						hasSendNotify = false
						batteryPercentageNotify = sync.Once{}
					}
					if needConnectNotifyId != 0 {
						err = m.closeNotify(needConnectNotifyId)
						if err != nil {
							logger.Warning(err)
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
				handleSysPowerBatteryEvent()
			})
			_ = m.sysPower.OnBattery().ConnectChanged(func(hasValue bool, onBattery bool) {
				if !hasValue {
					return
				}
				onBatteryGlobal = onBattery
				handleSysPowerBatteryEvent()
			})
		}
		// 设置hook
		job.setHooks(map[string]func(){
			string(system.RunningStatus): func() {
				// 开始更新时修改grub默认入口为rollback
				err := m.changeGrubDefaultEntry(rollbackBootEntry)
				if err != nil {
					logger.Warning(err)
				}
				// 状态更新为running
				err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeRunning, ReasonCode: system.NoError})
				if err != nil {
					logger.Warning(err)
				}
				// mask deepin-desktop-base,该包在系统更新完成后最后安装
				system.HandleDelayPackage(true, []string{
					"deepin-desktop-base",
				})
			},
			string(system.FailedStatus): func() {
				msg := gettext.Tr("Update failed")
				m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
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
				}

				// 上报更新失败的信息(如果需要)
				if m.needPostSystemUpgradeMessage() && ((mode & system.SystemUpdate) != 0) {
					go m.postSystemUpgradeMessage(upgradeFailed, job, system.SystemUpdate)
				}
				m.reportLog(upgradeStatus, false, job.Description)
				// unmask deepin-desktop-base 无需继续安装
				system.HandleDelayPackage(false, []string{
					"deepin-desktop-base",
				})
			},
			string(system.SucceedStatus): func() {
				// 更新成功后修改grub默认入口为当前系统入口
				err := m.changeGrubDefaultEntry(normalBootEntry)
				if err != nil {
					logger.Warning(err)
				}
				// 状态更新为ready
				err = m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{Status: system.UpgradeReady, ReasonCode: system.NoError})
				if err != nil {
					logger.Warning(err)
				}
				// 系统更新成功后,最后安装deepin-desktop-base包,安装成功后进度更新为100%并变成succeed状态
				ch := make(chan int, 1)
				// unmask deepin-desktop-base并安装
				system.HandleDelayPackage(false, []string{
					"deepin-desktop-base",
				})
				go func() {
					m.do.Lock()
					defer m.do.Unlock()
					isExist, installJob, err := m.jobManager.CreateJob("install base", system.OnlyInstallJobType, []string{"deepin-desktop-base"}, environ)
					if err != nil {
						ch <- 1
						logger.Warning(err)
					}
					if isExist {
						ch <- 1
						return
					}
					if installJob != nil {
						installJob.option = job.option
						installJob.setHooks(map[string]func(){
							string(system.FailedStatus): func() {
								ch <- 1
							},
							string(system.SucceedStatus): func() {
								ch <- 1
							},
						})
						if err := m.jobManager.addJob(installJob); err != nil {
							logger.Warning(err)
							ch <- 1
							return
						}
					}
				}()
				select {
				case <-ch:
					logger.Info("install deepin-desktop-base done,upgrade succeed.")
				}
				err = m.config.UpdateLastoreDaemonStatus(canUpgrade, false)
				if err != nil {
					logger.Warning(err)
				}
				// 等待deepin-desktop-base安装完成后,状态后续切换
				job.setPropProgress(1.00)
				// 上报更新成功的信息(如果需要)
				if m.needPostSystemUpgradeMessage() && ((mode & system.SystemUpdate) != 0) {
					go m.postSystemUpgradeMessage(upgradeSucceed, job.next, system.SystemUpdate)
				}
				m.reportLog(upgradeStatus, true, "")
			},
			string(system.EndStatus): func() {
				// wrapper的资源释放
				if unref != nil {
					unref()
				}
				m.sysPower.RemoveHandler(proxy.RemovePropertiesChangedHandler)
			},
		})
		if !isClassify { // 分类下载的job需要外部判断是否add
			if err := m.jobManager.addJob(job); err != nil {
				if unref != nil {
					unref()
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
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
func (m *Manager) prepareDistUpgrade(sender dbus.Sender, mode system.UpdateType, isClassify bool) (*Job, error) {
	environ, err := makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.ensureUpdateSourceOnce()
	m.updateJobList()

	if isClassify {
		m.updater.PropsMu.RLock()
		classifiedUpdatablePackagesMap := m.updater.ClassifiedUpdatablePackages
		m.updater.PropsMu.RUnlock()
		if len(classifiedUpdatablePackagesMap[mode.JobType()]) == 0 {
			return nil, system.NotFoundError("empty UpgradableApps")
		}
	} else {
		m.PropsMu.RLock()
		upgradableApps := m.UpgradableApps
		m.PropsMu.RUnlock()
		if len(upgradableApps) == 0 {
			return nil, system.NotFoundError("empty UpgradableApps")
		}
	}
	var needDownloadSize float64
	if needDownloadSize, err = system.QuerySourceDownloadSize(mode, false); err == nil && needDownloadSize == 0 {
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
			spaceNum = spaceNum * 1024
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
		if string(sender) == m.service.Conn().Names()[0] {
			msg := gettext.Tr("You don't have enough free space to download")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		}
		logger.Warning(dbusError.Detail)
		errStr, _ := json.Marshal(dbusError)
		return nil, dbusutil.ToError(errors.New(string(errStr)))
	}
	var job *Job
	var isExist bool
	err = system.CustomSourceWrapper(mode, func(path string, unref func()) error {
		m.do.Lock()
		defer m.do.Unlock()
		if isClassify {
			jobType := GetUpgradeInfoMap()[mode].PrepareJobId
			const jobName = "OnlyDownload" // 提供给daemon的lastore模块判断当前下载任务是否还有后续更新任务
			isExist, job, err = m.jobManager.CreateJob(jobName, jobType, nil, environ)
		} else {
			isExist, job, err = m.jobManager.CreateJob("", system.PrepareDistUpgradeJobType, nil, environ)
		}
		if err != nil {
			logger.Warningf("Prepare DistUpgrade error: %v\n", err)
			if unref != nil {
				unref()
			}
			return err
		}
		if isExist {
			return nil
		}
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
		if m.updater.downloadSpeedLimitConfigObj.DownloadSpeedLimitEnabled {
			job.option["Acquire::http::Dl-Limit"] = m.updater.downloadSpeedLimitConfigObj.LimitSpeed
		}
		var sendAutoDownloadOnce sync.Once
		job.setHooks(map[string]func(){
			string(system.ReadyStatus): func() {
				m.PropsMu.Lock()
				m.isDownloading = true
				m.PropsMu.Unlock()
			},
			string(system.RunningStatus): func() {
				m.PropsMu.Lock()
				m.isDownloading = true
				m.PropsMu.Unlock()
				// 如果sender为自己,则为触发自动下载
				if sender == dbus.Sender(m.service.Conn().Names()[0]) {
					sendAutoDownloadOnce.Do(func() {
						msg := gettext.Tr("auto downloading")
						m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
					})
				}
			},
			string(system.SucceedStatus): func() {
				err = m.config.UpdateLastoreDaemonStatus(canUpgrade, true)
				if err != nil {
					logger.Warning(err)
				}
				if sender == dbus.Sender(m.service.Conn().Names()[0]) {
					msg := gettext.Tr("Newer version updates were downloaded successfully. You can install them when shut down or reboot the system")
					action := []string{
						"updateNow",
						gettext.Tr("update now"),
						"ignore",
						gettext.Tr("ignore"),
					}
					hints := map[string]dbus.Variant{"x-deepin-action-updateNow": dbus.MakeVariant("dbus-send,--session,--print-reply,--dest=com.deepin.dde.shutdownFront,/com/deepin/dde/shutdownFront,com.deepin.dde.shutdownFront.UpdateAndShutdown")}
					m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
				}
				m.reportLog(downloadStatus, true, "")
			},
			string(system.FailedStatus): func() {
				m.PropsMu.Lock()
				m.isDownloading = false
				packages := m.UpgradableApps
				m.PropsMu.Unlock()
				m.reportLog(downloadStatus, false, job.Description)
				var errorContent = struct {
					ErrType   string
					ErrDetail string
				}{}
				err = json.Unmarshal([]byte(job.Description), &errorContent)
				if err == nil {
					if errorContent.ErrType == string(system.ErrorInsufficientSpace) {
						var msg string
						size, err := system.QueryPackageDownloadSize(false, packages...)
						if err != nil {
							logger.Warning(err)
							msg = gettext.Tr("更新包下载失败，请为下载目录释放空间")
						} else {
							msg = fmt.Sprintf(gettext.Tr("更新包下载失败，请为下载目录释放%v空间"), size/1000*1000*1000)
						}
						m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
					}
				}
			},
			string(system.EndStatus): func() {
				if unref != nil {
					unref()
				}
				m.PropsMu.Lock()
				m.isDownloading = false
				m.PropsMu.Unlock()
			},
			// 在reload状态时,修改job的配置
			string(system.ReloadStatus): func() {
				if m.updater.downloadSpeedLimitConfigObj.DownloadSpeedLimitEnabled {
					if job.option == nil {
						job.option = make(map[string]string)
					}
					job.option["Acquire::http::Dl-Limit"] = m.updater.downloadSpeedLimitConfigObj.LimitSpeed
				} else {
					if job.option != nil {
						delete(job.option, "Acquire::http::Dl-Limit")
					}
				}
			},
		})
		if err := m.jobManager.addJob(job); err != nil {
			if unref != nil {
				unref()
			}
			return err
		}
		return nil
	})
	if err != nil {
		logger.Warningf("PrepareDistUpgrade error: %v\n", err)
		return nil, err
	}
	return job, err
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
				upgradeJob, err = m.distUpgrade(sender, category, true)
				if err != nil {
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
	isExist, job, err := m.jobManager.CreateJob(jobName, system.CleanJobType, nil, nil)
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
			m.sendNotify("dde-control-center", 0, "deepin-appstore", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
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
	isExist, job, err := m.jobManager.CreateJob("", system.FixErrorJobType, []string{errType}, environ)
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
	writeType := system.UpdateType(pw.Value.(uint64))

	if writeType&system.OnlySecurityUpdate != 0 { // 如果更新类别包含仅安全更新，关闭其它更新项
		writeType = system.OnlySecurityUpdate
	}
	pw.Value = writeType
	err := m.config.SetUpdateMode(writeType)
	if err != nil {
		logger.Warning(err)
	}
	err = updateSecurityConfigFile(writeType == system.OnlySecurityUpdate)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (m *Manager) modifyUpdateMode() {
	m.PropsMu.RLock()
	mode := m.UpdateMode
	m.PropsMu.RUnlock()
	if mode&system.OnlySecurityUpdate != 0 { // 如果更新类别包含仅安全更新，关闭其它更新项
		mode = system.OnlySecurityUpdate
	}
	m.setPropUpdateMode(mode)
	err := m.config.SetUpdateMode(mode)
	if err != nil {
		logger.Warning(err)
	}
	err = updateSecurityConfigFile(mode == system.OnlySecurityUpdate)
	if err != nil {
		logger.Warning(err)
	}
}

// SystemUpgradeInfo 将update_infos.json数据解析成map
func (m *Manager) SystemUpgradeInfo() (map[string][]system.UpgradeInfo, error) {
	r := make(system.SourceUpgradeInfoMap)

	filename := path.Join(system.VarLibDir, "update_infos.json")
	var updateInfosList []system.UpgradeInfo
	err := system.DecodeJson(filename, &updateInfosList)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}

		var updateInfoErr system.UpdateInfoError
		err2 := system.DecodeJson(filename, &updateInfoErr)
		if err2 == nil {
			return nil, &updateInfoErr
		}
		return nil, fmt.Errorf("Invalid update_infos: %v\n", err)
	}
	for _, info := range updateInfosList {
		r[info.Category] = append(r[info.Category], info)
	}
	return r, nil
}

func (m *Manager) categorySupportAutoInstall(category system.UpdateType) bool {
	m.updater.PropsMu.RLock()
	autoInstallUpdates := m.updater.AutoInstallUpdates
	autoInstallUpdateType := m.updater.AutoInstallUpdateType
	m.updater.PropsMu.RUnlock()
	return autoInstallUpdates && (autoInstallUpdateType&category != 0)
}

func (m *Manager) handleAutoCheckEvent() error {
	var checkNeedUpdateSource = func() bool {
		upgradeTypeList := []string{
			system.PrepareDistUpgradeJobType,
			system.PrepareSystemUpgradeJobType,
			system.PrepareAppStoreUpgradeJobType,
			system.PrepareUnknownUpgradeJobType,
			system.PrepareSecurityUpgradeJobType,
			system.DistUpgradeJobType,
			system.SystemUpgradeJobType,
			system.AppStoreUpgradeJobType,
			system.SecurityUpgradeJobType,
			system.UnknownUpgradeJobType,
			system.OnlyInstallJobType,
		}
		for _, jobType := range upgradeTypeList {
			job := m.jobManager.findJobById(genJobId(jobType))
			if job != nil && job.Status == system.RunningStatus {
				return false
			}
		}
		return true
	}
	if m.config.AutoCheckUpdates {
		if !checkNeedUpdateSource() {
			logger.Info("lastore is running prepare upgrade or upgrade job, not need check update")
			return nil
		}
		_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]), m.updater.UpdateNotify)
		if err != nil {
			logger.Warning(err)
			return err
		}
		err = m.config.UpdateLastCheckTime()
		if err != nil {
			logger.Warning(err)
			return err
		}
		err = m.updateAutoCheckSystemUnit()
		if err != nil {
			logger.Warning(err)
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
		cacheSize := size / 1024.0
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

type upgradePostContent struct {
	SerialNumber    string   `json:"serialNumber"`
	MachineID       string   `json:"machineId"`
	UpgradeStatus   int      `json:"status"`
	UpgradeErrorMsg string   `json:"msg"`
	TimeStamp       int64    `json:"timestamp"`
	SourceUrl       []string `json:"sourceUrl"`
	Version         string   `json:"version"`
}

const (
	upgradeSucceed = 0
	upgradeFailed  = 1
)

// 发送系统更新成功或失败的状态
func (m *Manager) postSystemUpgradeMessage(upgradeStatus int, j *Job, updateType system.UpdateType) {
	m.inhibitAutoQuitCountAdd()
	defer m.inhibitAutoQuitCountSub()
	var upgradeErrorMsg string
	var version string
	if upgradeStatus == upgradeFailed && j != nil {
		upgradeErrorMsg = j.Description
	}
	infoMap, err := getOSVersionInfo()
	if err != nil {
		logger.Warning(err)
	} else {
		version = strings.Join(
			[]string{infoMap["MajorVersion"], infoMap["MinorVersion"], infoMap["OsBuild"]}, ".")
	}

	sn, err := getSN()
	if err != nil {
		logger.Warning(err)
	}
	hardwareId, err := getHardwareId()
	if err != nil {
		logger.Warning(err)
	}

	sourceFilePath := system.GetCategorySourceMap()[updateType]
	postContent := &upgradePostContent{
		SerialNumber:    sn,
		MachineID:       hardwareId,
		UpgradeStatus:   upgradeStatus,
		UpgradeErrorMsg: upgradeErrorMsg,
		TimeStamp:       time.Now().Unix(),
		SourceUrl:       getUpgradeUrls(sourceFilePath),
		Version:         version,
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
	const url = "https://update-platform.uniontech.com/api/v1/update/status"
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

const (
	lastoreUnitCache    = "/tmp/lastoreUnitCache"
	lastoreJobCacheJson = "/tmp/lastoreJobCache.json"
	run                 = "systemd-run"
	lastoreDBusCmd      = "dbus-send --system --print-reply --dest=com.deepin.lastore /com/deepin/lastore com.deepin.lastore.Manager.HandleSystemEvent"
)

type systemdEventType string

const (
	AutoCheck          systemdEventType = "AutoCheck"
	AutoClean          systemdEventType = "AutoClean"
	UpdateInfosChanged systemdEventType = "UpdateInfosChanged"
	OsVersionChanged   systemdEventType = "OsVersionChanged"
	AutoDownload       systemdEventType = "AutoDownload"
	AbortAutoDownload  systemdEventType = "AbortAutoDownload"
)

func (m *Manager) getNextUpdateDelay() time.Duration {
	elapsed := time.Since(m.config.LastCheckTime)
	remained := m.config.CheckInterval - elapsed
	if remained < 0 {
		return _minDelayTime
	}
	// ensure delay at least have 10 seconds
	return remained + _minDelayTime

}

type UnitName string

const (
	lastoreOnline            UnitName = "lastoreOnline"
	lastoreAutoClean         UnitName = "lastoreAutoClean"
	lastoreAutoCheck         UnitName = "lastoreAutoCheck"
	lastoreAutoUpdateToken   UnitName = "lastoreAutoUpdateToken"
	watchOsVersion           UnitName = "watchOsVersion"
	watchUpdateInfo          UnitName = "watchUpdateInfo"
	lastoreAutoDownload      UnitName = "lastoreAutoDownload"
	lastoreAbortAutoDownload UnitName = "lastoreAbortAutoDownload"
)

type lastoreUnitMap map[UnitName][]string

// 定时任务和文件监听
func (m *Manager) getLastoreSystemUnitMap() lastoreUnitMap {
	unitMap := make(lastoreUnitMap)
	if (m.config.getLastoreDaemonStatus() & disableUpdate) == 0 { // 更新禁用未开启时
		unitMap[lastoreOnline] = []string{
			// 随机数范围0-21600，时间为0-6小时
			fmt.Sprintf("--on-active=%d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(21600)),
			"/bin/bash",
			"-c",
			fmt.Sprintf("/usr/bin/nm-online -t 3600 && %s string:%s", lastoreDBusCmd, AutoCheck), // 等待网络联通后检查更新
		}
		unitMap[lastoreAutoCheck] = []string{
			// 随机数范围0-21600，时间为0-6小时
			fmt.Sprintf("--on-active=%d", int(m.getNextUpdateDelay()/time.Second)+rand.New(rand.NewSource(time.Now().UnixNano())).Intn(21600)),
			"/bin/bash",
			"-c",
			fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, AutoCheck), // 根据上次检查时间,设置下一次自动检查时间
		}
		unitMap[watchUpdateInfo] = []string{
			"--path-property=PathModified=/var/lib/lastore/update_infos.json",
			"--property=StartLimitBurst=0",
			"/bin/bash",
			"-c",
			fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, UpdateInfosChanged), // 监听update_infos.json文件
		}
	}
	unitMap[lastoreAutoClean] = []string{
		"--on-active=600",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, AutoClean), // 10分钟后自动检查是否需要清理
	}
	unitMap[lastoreAutoUpdateToken] = []string{
		"--on-active=60",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, OsVersionChanged), // 60s后更新token文件
	}
	unitMap[watchOsVersion] = []string{
		"--path-property=PathModified=/etc/os-version",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, "OsVersionChanged"), // 监听os-version文件，更新token
	}
	m.updater.PropsMu.RLock()
	enable := m.updater.idleDownloadConfigObj.IdleDownloadEnabled
	m.updater.PropsMu.RUnlock()
	if enable {
		unitMap[lastoreAutoDownload] = []string{
			fmt.Sprintf("--on-active=%d", m.getNextAutoDownloadDelay()/time.Second),
			"/bin/bash",
			"-c",
			fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, AutoDownload), // 根据用户设置的自动下载的时间段，设置自动下载开始的时间
		}
		unitMap[lastoreAbortAutoDownload] = []string{
			fmt.Sprintf("--on-active=%d", m.getAbortNextAutoDownloadDelay()/time.Second),
			"/bin/bash",
			"-c",
			fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, AbortAutoDownload), // 根据用户设置的自动下载的时间段，终止自动下载
		}
	}
	return unitMap
}

// 开启定时任务和文件监听(通过systemd-run实现)
func (m *Manager) startSystemdUnit() {
	m.lastoreUnitCacheMu.Lock()
	defer m.lastoreUnitCacheMu.Unlock()

	if system.NormalFileExists(lastoreUnitCache) {
		return
	}
	kf := keyfile.NewKeyFile()
	for name, cmdArgs := range m.getLastoreSystemUnitMap() {
		var args []string
		args = append(args, fmt.Sprintf("--unit=%s", name))
		args = append(args, cmdArgs...)
		cmd := exec.Command(run, args...)
		logger.Info(cmd.String())
		var errBuffer bytes.Buffer
		cmd.Stderr = &errBuffer
		err := cmd.Run()
		if err != nil {
			logger.Warning(err)
			logger.Warning(errBuffer.String())
			continue
		}
		kf.SetString("UnitName", string(name), fmt.Sprintf("%s.unit", name))
	}

	err := kf.SaveToFile(lastoreUnitCache)
	if err != nil {
		logger.Warning(err)
	}
}

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

// 保存检查过更新的状态
func (m *Manager) saveUpdateSourceOnce() {
	m.lastoreUnitCacheMu.Lock()
	defer m.lastoreUnitCacheMu.Unlock()

	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(lastoreUnitCache)
	if err != nil {
		logger.Warning(err)
		return
	}
	kf.SetBool("RecordData", "UpdateSourceOnce", true)
	err = kf.SaveToFile(lastoreUnitCache)
	if err != nil {
		logger.Warning(err)
	}
}

// 读取检查过更新的状态
func (m *Manager) loadUpdateSourceOnce() {
	m.lastoreUnitCacheMu.Lock()
	defer m.lastoreUnitCacheMu.Unlock()

	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(lastoreUnitCache)
	if err != nil {
		logger.Warning(err)
		return
	}
	updateSourceOnce, err := kf.GetBool("RecordData", "UpdateSourceOnce")
	if err == nil {
		m.PropsMu.Lock()
		m.updateSourceOnce = updateSourceOnce
		m.PropsMu.Unlock()
	} else {
		logger.Warning(err)
	}

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
				updateType = m.UpdateMode
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
				err := m.jobManager.pauseJob(pausedJob)
				if err != nil {
					logger.Warning(err)
				}
				pausedJob.Progress = j.Progress
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

// 下载中断或者修改下载时间段后,需要更新timer   用户手动中断下载时，需要再第二天的设置实际重新下载   开机时间在自动下载时间段内时，
func (m *Manager) updateAutoDownloadTimer() error {
	err := m.updateTimerUnit(lastoreAbortAutoDownload)
	if err != nil {
		return err
	}
	err = m.updateTimerUnit(lastoreAutoDownload)
	if err != nil {
		return err
	}
	m.updater.PropsMu.RLock()
	enable := m.updater.idleDownloadConfigObj.IdleDownloadEnabled
	m.updater.PropsMu.RUnlock()
	// 如果关闭闲时更新，需要终止下载job
	if !enable {
		m.handleAbortAutoDownload()
	}
	return nil
}

// systemd计时服务需要根据上一次更新时间而变化
func (m *Manager) updateAutoCheckSystemUnit() error {
	err := m.stopTimerUnit(lastoreOnline)
	if err != nil {
		logger.Warning(err)
	}
	return m.updateTimerUnit(lastoreAutoCheck)
}

func (m *Manager) stopTimerUnit(unitName UnitName) error {
	timerName := fmt.Sprintf("%s.%s", unitName, "timer")
	_, err := m.systemd.GetUnit(0, timerName)
	if err == nil {
		_, err = m.systemd.StopUnit(0, timerName, "replace")
		if err != nil {
			logger.Warning(err)
			return err
		}
	} else {
		return err
	}
	return nil
}

// 重新启动systemd unit,先GetUnit，如果能获取到，就调用StopUnit(replace).如果获取不到,证明已经处理完成,直接重新创建对应unit执行
func (m *Manager) updateTimerUnit(unitName UnitName) error {
	timerName := fmt.Sprintf("%s.%s", unitName, "timer")
	_, err := m.systemd.GetUnit(0, timerName)
	if err == nil {
		_, err = m.systemd.StopUnit(0, timerName, "replace")
		if err != nil {
			logger.Warning(err)
		}
	}
	var args []string
	args = append(args, fmt.Sprintf("--unit=%s", unitName))
	autoCheckArgs, ok := m.getLastoreSystemUnitMap()[unitName]
	if ok {
		args = append(args, autoCheckArgs...)
		cmd := exec.Command(run, args...)
		var errBuffer bytes.Buffer
		cmd.Stderr = &errBuffer
		err = cmd.Run()
		if err != nil {
			logger.Warning(err)
			logger.Warning(errBuffer.String())
			return errors.New(errBuffer.String())
		}
		logger.Debug(cmd.String())
	}
	return nil
}

// getNextAutoDownloadDelay 用配置时间减去当前时间，得到延迟下载任务开始时间.
func (m *Manager) getNextAutoDownloadDelay() time.Duration {
	m.updater.PropsMu.RLock()
	defer m.updater.PropsMu.RUnlock()
	beginDur := getCustomTimeDuration(m.updater.idleDownloadConfigObj.BeginTime)
	endDur := getCustomTimeDuration(m.updater.idleDownloadConfigObj.EndTime)
	defer func() {
		logger.Debug("auto download begin time duration:", beginDur)
	}()
	// 如果下载或者自动下载已经开始,下一次的开始时间应该为第二天时间
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()
	if m.isDownloading {
		return beginDur
	}
	// 如果用户开机时间在自动下载时间段内，则返回默认最小时间(立即开始)
	if beginDur > endDur {
		beginDur = _minDelayTime
	}
	return beginDur
}

// getAbortNextAutoDownloadDelay 用配置时间减去当前时间，得到终止延迟下载任务的时间.
func (m *Manager) getAbortNextAutoDownloadDelay() time.Duration {
	m.updater.PropsMu.RLock()
	defer m.updater.PropsMu.RUnlock()
	beginDur := getCustomTimeDuration(m.updater.idleDownloadConfigObj.BeginTime)
	endDur := getCustomTimeDuration(m.updater.idleDownloadConfigObj.EndTime)
	defer func() {
		logger.Debug("auto download end time duration:", endDur)
	}()
	// 如果用户开机时间在自动下载时间段内,且立即开始的时间(_minDelayTime)大于结束时间,则结束时间为两倍最小时间
	if beginDur > endDur && endDur <= _minDelayTime {
		endDur = _minDelayTime * 2
	}
	return endDur
}

func (m *Manager) handleAutoDownload() {
	_, err := m.PrepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]))
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleAbortAutoDownload() {
	err := m.CleanJob(system.PrepareDistUpgradeJobType)
	if err != nil {
		logger.Warning(err)
	}
}

const (
	grubScriptFile       = "/boot/grub/grub.cfg"
	normalBootEntryTitle = "UnionTech OS Desktop 20 Pro GNU/Linux"
)

type bootEntry uint

const (
	normalBootEntry bootEntry = iota
	rollbackBootEntry
)

// changeGrubDefaultEntry 设置grub默认入口(社区版可能不需要进行grub设置)
func (m *Manager) changeGrubDefaultEntry(to bootEntry) error {
	var title string
	var err error
	ch := make(chan bool, 1)
	switch to {
	case rollbackBootEntry:
		title, err = system.GetGrubRollbackTitle(grubScriptFile)
		if err != nil {
			return err
		}
	case normalBootEntry:
		title = normalBootEntryTitle
	}
	logger.Debug("change grub default entry to:", title)
	defaultEntry, err := m.grub.DefaultEntry().Get(0)
	if err != nil {
		return err
	}
	if defaultEntry == title {
		return nil
	}
	entrys, err := m.grub.GetSimpleEntryTitles(0)
	if err != nil {
		return err
	}
	if !strv.Strv(entrys).Contains(title) {
		return fmt.Errorf("grub no %s entry", title)
	}
	err = m.grub.SetDefaultEntry(0, title)
	if err != nil {
		return err
	}
	logger.Info("updating grub default entry to ", title)
	_ = m.grub.Updating().ConnectChanged(func(hasValue bool, updating bool) {
		if !hasValue {
			return
		}
		if !updating {
			ch <- updating
		}
	})
	defer func() {
		m.grub.RemoveHandler(proxy.RemovePropertiesChangedHandler)
	}()
	select {
	case <-ch:
		return nil
	case <-time.After(30 * time.Second):
		return nil
	}
}

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

func (m *Manager) handleFailedNotify() {
	status := m.config.upgradeStatus
	// 更新中断的通知(断电,强制关机等)
	switch status.Status {
	case system.UpgradeRunning:
		// config失效时,无法回滚,提示重新更新
		valid, err := m.abObj.ConfigValid().Get(0)
		if err != nil || !valid {
			msg := gettext.Tr("upgrade abort,need try to upgrade again")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)

		} else {
			msg := gettext.Tr("upgrade abort,need rollback")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		}
	case system.UpgradeFailed:
		switch status.ReasonCode {
		case system.ErrorDpkgError:
			msg := gettext.Tr("upgrade failed because an error occurred during dpkg installation, need rollback")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		case system.ErrorPkgNotFound:
			msg := gettext.Tr("upgrade failed because unable to locate package, need rollback")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		case system.ErrorNoInstallationCandidate:
			msg := gettext.Tr("upgrade failed because has no installation candidate")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		case system.ErrorUnknown:
			msg := gettext.Tr("upgrade failed, need rollback")
			m.sendNotify("dde-control-center", 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutNoHide)
		}
	}
	err := m.config.SetUpgradeStatusAndReason(system.UpgradeStatusAndReason{
		Status:     system.UpgradeReady,
		ReasonCode: system.NoError,
	})
	if err != nil {
		logger.Warning(err)
	}
}

type reportCategory uint32

const (
	updateStatus reportCategory = iota
	downloadStatus
	upgradeStatus
)

type reportLogInfo struct {
	Tid    int
	Result bool
	Reason string
}

func (m *Manager) reportLog(category reportCategory, status bool, description string) {
	go func() {
		agent := m.userAgents.getActiveLastoreAgent()
		if agent != nil {
			logInfo := reportLogInfo{
				Result: status,
				Reason: description,
			}
			switch category {
			case updateStatus:
				logInfo.Tid = 1000600002
			case downloadStatus:
				logInfo.Tid = 1000600003
			case upgradeStatus:
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
	// UpdateMode修改后，需要先清零，然后再根据实际状态判断是否可以安装更新
	err := m.config.UpdateLastoreDaemonStatus(canUpgrade, false)
	if err != nil {
		logger.Warning(err)
	}
	// UpdateMode修改后,一些对外属性需要同步修改
	infosMap, err := m.SystemUpgradeInfo()
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info(err) // 移除文件时,同样会进入该逻辑,因此在update_infos.json文件不存在时,将日志等级改为info
		} else {
			logger.Error("failed to get upgrade info:", err)
		}
	} else {
		m.updateUpdatableProp(infosMap)
	}
	go func() {
		m.PropsMu.RLock()
		packages := m.UpgradableApps
		m.PropsMu.RUnlock()
		size, err := system.QueryPackageDownloadSize(false, packages...)
		if err != nil {
			logger.Warning(err)
			err = m.config.UpdateLastoreDaemonStatus(canUpgrade, false)
			if err != nil {
				logger.Warning(err)
			}
		} else {
			err = m.config.UpdateLastoreDaemonStatus(canUpgrade, size == 0 && len(packages) != 0)
			if err != nil {
				logger.Warning(err)
			}
		}
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
