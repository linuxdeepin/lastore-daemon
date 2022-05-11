/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"internal/system"
	"internal/utils"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/godbus/dbus"
	abrecovery "github.com/linuxdeepin/go-dbus-factory/com.deepin.abrecovery"
	apps "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.apps"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.systemd1"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/strv"
)

const (
	UserExperServiceName = "com.deepin.userexperience.Daemon"
	UserExperPath        = "/com/deepin/userexperience/Daemon"

	UserExperInstallApp   = "installapp"
	UserExperUninstallApp = "uninstallapp"

	uosReleaseNotePkgName = "uos-release-note"

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
	}
	allowRemovePackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		sessionDaemonPath,
		langSelectorPath,
	}
)

const MaxCacheSize = 500.0 //size MB

type Manager struct {
	service *dbusutil.Service
	do      sync.Mutex
	b       system.System
	config  *Config

	PropsMu sync.RWMutex
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

	apps                     apps.Apps
	sysPower                 power.Power
	abRecovery               abrecovery.ABRecovery
	signalLoop               *dbusutil.SignalLoop
	shouldHandleBackupJobEnd bool
	autoInstallType          system.UpdateType // 保存需要自动安装的类别

	UpdateMode system.UpdateType `prop:"access:rw"`

	isUpdateSucceed bool
	canRestore      bool

	inhibitAutoQuitCount int32
	autoQuitCountMu      sync.Mutex
	lastoreUnitCacheMu   sync.Mutex
}

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func NewManager(service *dbusutil.Service, b system.System, c *Config) *Manager {
	archs, err := system.SystemArchitectures()
	if err != nil {
		logger.Errorf("Can't detect system supported architectures %v\n", err)
		return nil
	}

	m := &Manager{
		service:             service,
		config:              c,
		b:                   b,
		SystemArchitectures: archs,
		inhibitFd:           -1,
		AutoClean:           c.AutoClean,
		UpdateMode:          c.UpdateMode,
	}
	sysBus := service.Conn()
	m.apps = apps.NewApps(sysBus)
	m.initAutoInstall(sysBus)
	m.jobManager = NewJobManager(service, b, m.updateJobList)
	go m.handleOSSignal()
	m.updateJobList()
	m.modifyUpdateMode()
	return m
}

var pkgNameRegexp = regexp.MustCompile(`^[a-z0-9]`)

func NormalizePackageNames(s string) ([]string, error) {
	pkgNames := strings.Fields(s)
	for _, pkgName := range pkgNames {
		if !pkgNameRegexp.MatchString(pkgName) {
			return nil, fmt.Errorf("invalid package name %q", pkgName)
		}
	}

	if s == "" || len(pkgNames) == 0 {
		return nil, fmt.Errorf("Empty value")
	}
	return pkgNames, nil
}

func makeEnvironWithSender(service *dbusutil.Service, sender dbus.Sender) (map[string]string, error) {
	environ := make(map[string]string)

	pid, err := service.GetConnPID(string(sender))
	if err != nil {
		return nil, err
	}

	p := procfs.Process(pid)
	envVars, err := p.Environ()
	if err != nil {
		logger.Warningf("failed to get process %d environ: %v", p, err)
	} else {
		environ["DISPLAY"] = envVars.Get("DISPLAY")
		environ["XAUTHORITY"] = envVars.Get("XAUTHORITY")
		environ["DEEPIN_LASTORE_LANG"] = getLang(envVars)
	}
	return environ, nil
}

func getUsedLang(environ map[string]string) string {
	return environ["DEEPIN_LASTORE_LANG"]
}

func getLang(envVars procfs.EnvVars) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGE", "LANG"} {
		value := envVars.Get(name)
		if value != "" {
			return value
		}
	}
	return ""
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
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	job, err := m.jobManager.CreateJob(jobName, system.UpdateJobType, pkgs, environ)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("UpdatePackage %q error: %v\n", packages, err)
	}
	job.caller = caller
	return job, err
}

func (m *Manager) UpdatePackage(sender dbus.Sender, jobName string, packages string) (job dbus.ObjectPath,
	busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.updatePackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) installPackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
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

func (m *Manager) InstallPackage(sender dbus.Sender, jobName string, packages string) (job dbus.ObjectPath,
	busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	execPath, cmdLine, err := m.getExecutablePathAndCmdline(sender)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}
	if !allowInstallPackageExecPaths.Contains(execPath) &&
		uid != 0 {
		err = fmt.Errorf("%q is not allowed to install packages", execPath)
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	jobObj, err := m.installPackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	jobObj.next.caller = mapMethodCaller(execPath, cmdLine)
	return jobObj.getPath(), nil
}

func sendInstallMsgToUserExperModule(msg, path, name, id string) {
	bus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	userexp := bus.Object(UserExperServiceName, UserExperPath)
	ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFn()
	// 设置两秒的超时，如果两秒内函数没处理完，则返回err，并且不会阻塞
	err = userexp.CallWithContext(ctx, UserExperServiceName+".SendAppInstallData", 0, msg, path, name, id).Err
	if err != nil {
		logger.Warningf("failed to call %s.SendAppInstallData, %v", UserExperServiceName, err)
	} else {
		logger.Debugf("send %s message to ue module", msg)
	}
}

func (m *Manager) installPkg(jobName, packages string, environ map[string]string) (*Job, error) {
	pList := strings.Fields(packages)

	m.do.Lock()
	job, err := m.jobManager.CreateJob(jobName, system.InstallJobType, pList, environ)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("installPackage %q error: %v\n", packages, err)
	}

	if job != nil && !isCommunity() {
		job.setHooks(map[string]func(){
			string(system.SucceedStatus): func() {
				for _, pkg := range job.Packages {
					logger.Debugf("install app %s success, notify ue module", pkg)
					sendInstallMsgToUserExperModule(UserExperInstallApp, "", jobName, pkg)
				}
			},
		})
	}

	return job, err
}

// dont collect experience message if edition is community
func isCommunity() bool {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/os-version")
	// 为避免收集数据的风险，读不到此文件，或者Edition文件不存在也不收集数据
	if err != nil {
		return true
	}
	edition, err := kf.GetString("Version", "EditionName")
	if err != nil {
		return true
	}
	if edition == "Community" {
		return true
	}
	return false
}

func (m *Manager) needPostSystemUpgradeMessage() bool {
	return strv.Strv(m.config.AllowPostSystemUpgradeMessageVersion).Contains(getEditionName())
}

func getEditionName() string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/os-version")
	// 为避免收集数据的风险，读不到此文件，或者Edition文件不存在也不收集数据
	if err != nil {
		return ""
	}
	edition, err := kf.GetString("Version", "EditionName")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(edition)
}

func listPackageDesktopFiles(pkg string) []string {
	var result []string
	filenames := system.ListPackageFile(pkg)
	for _, filename := range filenames {
		if strings.HasPrefix(filename, "/usr/") {
			// len /usr/ is 5
			if strings.HasSuffix(filename, ".desktop") &&
				(strings.HasPrefix(filename[5:], "share/applications") ||
					strings.HasPrefix(filename[5:], "local/share/applications")) {

				fileInfo, err := os.Stat(filename)
				if err != nil {
					continue
				}
				if fileInfo.IsDir() {
					continue
				}
				if !utf8.ValidString(filename) {
					continue
				}
				result = append(result, filename)
			}
		}
	}
	return result
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

	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	job, err := m.jobManager.CreateJob(jobName, system.RemoveJobType, pkgs, environ)
	m.do.Unlock()

	if job != nil && !isCommunity() {
		job.setHooks(map[string]func(){
			string(system.SucceedStatus): func() {
				for _, pkg := range job.Packages {
					logger.Debugf("uninstall app %s success, notify ue module", pkg)
					sendInstallMsgToUserExperModule(UserExperUninstallApp, "", jobName, pkg)
				}
			},
		})
	}

	if err != nil {
		logger.Warningf("removePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) RemovePackage(sender dbus.Sender, jobName string, packages string) (job dbus.ObjectPath,
	busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	execPath, cmdLine, err := m.getExecutablePathAndCmdline(sender)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	if !allowRemovePackageExecPaths.Contains(execPath) &&
		uid != 0 {
		err = fmt.Errorf("%q is not allowed to remove packages", execPath)
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	jobObj, err := m.removePackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	jobObj.caller = mapMethodCaller(execPath, cmdLine)
	return jobObj.getPath(), nil
}

func (m *Manager) ensureUpdateSourceOnce() {
	m.PropsMu.Lock()
	updateOnce := m.updateSourceOnce
	m.PropsMu.Unlock()

	if updateOnce {
		return
	}

	_, err := m.updateSource(false, false)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = m.config.UpdateLastCheckTime()
	if err != nil {
		logger.Warning(err)
		return
	}
	m.updateAutoCheckSystemUnit()
}

func (m *Manager) handleUpdateInfosChanged(autoCheck bool) {
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

	m.PropsMu.Lock()
	isUpdateSucceed := m.isUpdateSucceed
	m.PropsMu.Unlock()
	if m.updater.AutoDownloadUpdates && len(m.updater.UpdatablePackages) > 0 && isUpdateSucceed {
		logger.Info("auto download updates")
		go func() {
			m.inhibitAutoQuitCountAdd()
			_, err := m.classifiedUpgrade(dbus.Sender(m.service.Conn().Names()[0]), m.UpdateMode, false, autoCheck) // 自动下载使用控制中心的配置
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

func (m *Manager) updateSource(needNotify bool, autoCheck bool) (*Job, error) {
	m.do.Lock()
	var jobName string
	if needNotify {
		jobName = "+notify"
	}
	m.jobManager.dispatch() // 解决 bug 59351问题（防止CreatJob获取到状态为end但是未被删除的job）
	job, err := m.jobManager.CreateJob(jobName, system.UpdateSourceJobType, nil, nil)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("UpdateSource error: %v\n", err)
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
				go m.installUOSReleaseNote()
			},
			string(system.EndStatus): func() {
				m.handleUpdateInfosChanged(autoCheck)
			},
		})
	}
	return job, err
}

func (m *Manager) UpdateSource() (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.updateSource(false, false)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	_ = m.config.UpdateLastCheckTime()
	m.updateAutoCheckSystemUnit()
	return jobObj.getPath(), nil
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

func (m *Manager) DistUpgrade(sender dbus.Sender) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.distUpgrade(sender)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) distUpgrade(sender dbus.Sender) (*Job, error) {
	execPath, cmdLine, err := m.getExecutablePathAndCmdline(sender)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	caller := mapMethodCaller(execPath, cmdLine)
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.updateJobList()

	m.PropsMu.RLock()
	upgradableApps := m.UpgradableApps
	m.PropsMu.RUnlock()
	if len(upgradableApps) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}

	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob("", system.DistUpgradeJobType, upgradableApps, environ)
	if err != nil {
		logger.Warningf("DistUpgrade error: %v\n", err)
		return nil, err
	}
	job.caller = caller
	cancelErr := m.cancelAllUpdateJob()
	if cancelErr != nil {
		logger.Warning(cancelErr)
	}

	return job, err
}

func (m *Manager) PrepareDistUpgrade() (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.prepareDistUpgrade()
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) prepareDistUpgrade() (*Job, error) {
	m.ensureUpdateSourceOnce()
	m.updateJobList()

	m.PropsMu.RLock()
	upgradableApps := m.UpgradableApps
	m.PropsMu.RUnlock()

	if len(upgradableApps) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}
	if s, err := system.QueryPackageDownloadSize(upgradableApps...); err == nil && s == 0 {
		return nil, system.NotFoundError("no need download")
	}

	m.do.Lock()
	job, err := m.jobManager.CreateJob("", system.PrepareDistUpgradeJobType, upgradableApps, nil)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("PrepareDistUpgrade error: %v\n", err)
		return nil, err
	}
	return job, err
}

func (m *Manager) ClassifiedUpgrade(sender dbus.Sender, updateType system.UpdateType) ([]dbus.ObjectPath, *dbus.Error) {
	m.service.DelayAutoQuit()
	return m.classifiedUpgrade(sender, updateType, true, false)
}

// 根据更新类型,创建对应的下载或下载+安装的job
func (m *Manager) classifiedUpgrade(sender dbus.Sender, updateType system.UpdateType, isUpgrade bool, autoCheck bool) ([]dbus.ObjectPath, *dbus.Error) {
	var jobPaths []dbus.ObjectPath
	var err error
	var errList []string
	// 保证任务创建顺序
	for _, t := range system.AllUpdateType() {
		category := updateType & t
		if category != 0 {
			var upgradeJob, prepareJob *Job
			if isUpgrade {
				upgradeJob, err = m.createClassifiedUpgradeJob(sender, category)
				if err != nil {
					if !strings.Contains(err.Error(), system.NotFoundErrorMsg) {
						errList = append(errList, err.Error())
						logger.Warning(err)
					} else {
						logger.Info(err)
					}
					continue
				}
				jobPaths = append(jobPaths, upgradeJob.getPath())
				if upgradeJob.next != nil {
					jobPaths = append(jobPaths, upgradeJob.next.getPath())
				}
			} else {
				prepareJob, err = m.createPrepareClassifiedUpgradeJob(category)
				if err != nil {
					if !strings.Contains(err.Error(), system.NotFoundErrorMsg) {
						errList = append(errList, err.Error())
						logger.Warning(err)
					} else {
						logger.Info(err)
						if autoCheck && m.categorySupportAutoInstall(category) {
							go m.handlePackagesDownloaded(sender, category)
						}
					}
					continue
				}
				prepareJob.autoCheck = autoCheck
				jobPaths = append(jobPaths, prepareJob.getPath())
				if autoCheck {
					prepareJob.setHooks(map[string]func(){
						string(system.EndStatus): func() {
							if m.categorySupportAutoInstall(category) {
								go m.handlePackagesDownloaded(sender, category)
							}
						},
					})
				}
			}
		}
	}
	if len(errList) > 0 {
		return jobPaths, dbusutil.ToError(errors.New(strings.Join(errList, ",")))
	}
	return jobPaths, nil
}

func (m *Manager) createPrepareClassifiedUpgradeJob(updateType system.UpdateType) (*Job, error) {
	m.ensureUpdateSourceOnce()
	m.updateJobList()

	m.updater.PropsMu.RLock()
	classifiedUpdatablePackagesMap := m.updater.ClassifiedUpdatablePackages
	m.updater.PropsMu.RUnlock()

	if len(classifiedUpdatablePackagesMap[updateType.JobType()]) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}
	if s, err := system.QueryPackageDownloadSize(classifiedUpdatablePackagesMap[updateType.JobType()]...); err == nil && s == 0 {
		return nil, system.NotFoundError("no need download")
	}
	categoryMap := GetUpgradeInfoMap()
	jobType := categoryMap[updateType.JobType()].PrepareJobId
	const jobName = "OnlyDownload" // 提供给daemon的lastore模块判断当前下载任务是否还有后续更新任务
	m.do.Lock()
	job, err := m.jobManager.CreateJob(jobName, jobType, classifiedUpdatablePackagesMap[updateType.JobType()], nil)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("PrepareDistUpgrade error: %v\n", err)
		return nil, err
	}
	return job, nil
}

func (m *Manager) createClassifiedUpgradeJob(sender dbus.Sender, updateType system.UpdateType) (*Job, error) {
	execPath, cmdLine, err := m.getExecutablePathAndCmdline(sender)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	caller := mapMethodCaller(execPath, cmdLine)
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.updateJobList()

	m.updater.PropsMu.RLock()
	classifiedUpdatablePackagesMap := m.updater.ClassifiedUpdatablePackages
	m.updater.PropsMu.RUnlock()
	if len(classifiedUpdatablePackagesMap[updateType.JobType()]) == 0 {
		return nil, system.NotFoundError(fmt.Sprintf("empty %v UpgradableApps", updateType.JobType()))
	}

	m.do.Lock()
	defer m.do.Unlock()
	categoryMap := GetUpgradeInfoMap()
	jobType := categoryMap[updateType.JobType()].UpgradeJobId
	job, err := m.jobManager.CreateJob("", jobType, classifiedUpdatablePackagesMap[updateType.JobType()], environ)
	if err != nil {
		logger.Warningf("DistUpgrade error: %v\n", err)
		return nil, err
	}
	if job.next != nil {
		job.next.setHooks(map[string]func(){
			string(system.SucceedStatus): func() {
				if m.needPostSystemUpgradeMessage() && updateType == system.SystemUpdate {
					go m.postSystemUpgradeMessage(upgradeSucceed, job, updateType)
				}
			},
			string(system.FailedStatus): func() {
				if m.needPostSystemUpgradeMessage() && updateType == system.SystemUpdate {
					go m.postSystemUpgradeMessage(upgradeFailed, job, updateType)
				}
			},
		})
	}

	job.caller = caller
	if job.next != nil {
		job.next.caller = caller
	}
	cancelErr := m.cancelAllUpdateJob()
	if cancelErr != nil {
		logger.Warning(cancelErr)
	}

	return job, err
}

func (m *Manager) StartJob(jobId string) *dbus.Error {
	m.service.DelayAutoQuit()
	m.do.Lock()
	err := m.jobManager.MarkStart(jobId)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("StartJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) PauseJob(jobId string) *dbus.Error {
	m.do.Lock()
	err := m.jobManager.PauseJob(jobId)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("PauseJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) CleanJob(jobId string) *dbus.Error {
	m.service.DelayAutoQuit()
	m.do.Lock()
	err := m.jobManager.CleanJob(jobId)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("CleanJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) PackagesDownloadSize(packages []string) (size int64, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	m.ensureUpdateSourceOnce()

	s, err := system.QueryPackageDownloadSize(packages...)
	if err != nil || s == system.SizeUnknown {
		logger.Warningf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), s, err)
	}
	return int64(s), dbusutil.ToError(err)
}

func (m *Manager) PackageInstallable(pkgId string) (installable bool, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return system.QueryPackageInstallable(pkgId), nil
}

func (m *Manager) PackageExists(pkgId string) (exist bool, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return system.QueryPackageInstalled(pkgId), nil
}

// PackageDesktopPath TODO: Remove this API
func (m *Manager) PackageDesktopPath(pkgId string) (desktopPath string, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	p, err := utils.RunCommand("/usr/bin/lastore-tools", "querydesktop", pkgId)
	if err != nil {
		logger.Warningf("QueryDesktopPath failed: %q\n", err)
		return "", dbusutil.ToError(err)
	}
	return p, nil
}

func (m *Manager) SetRegion(region string) *dbus.Error {
	m.service.DelayAutoQuit()
	err := m.config.SetAppstoreRegion(region)
	return dbusutil.ToError(err)
}

func (m *Manager) SetAutoClean(enable bool) *dbus.Error {
	m.service.DelayAutoQuit()
	if m.AutoClean == enable {
		return nil
	}

	// save the config to disk
	err := m.config.SetAutoClean(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	m.AutoClean = enable
	err = m.emitPropChangedAutoClean(enable)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (m *Manager) GetArchivesInfo() (info string, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	info, err := getArchiveInfo()
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return info, nil
}

func getArchiveInfo() (string, error) {
	out, err := exec.Command("/usr/bin/lastore-apt-clean", "-print-json").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (m *Manager) CleanArchives() (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.cleanArchives(false)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) cleanArchives(needNotify bool) (*Job, error) {
	var jobName string
	if needNotify {
		jobName = "+notify"
	}

	m.do.Lock()
	job, err := m.jobManager.CreateJob(jobName, system.CleanJobType, nil, nil)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("CleanArchives error: %v", err)
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

func (m *Manager) FixError(sender dbus.Sender, errType string) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.fixError(sender, errType)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) fixError(sender dbus.Sender, errType string) (*Job, error) {
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
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
	job, err := m.jobManager.CreateJob("", system.FixErrorJobType,
		[]string{errType}, environ)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("fixError error: %v", err)
		return nil, err
	}
	return job, err
}

func (m *Manager) installUOSReleaseNote() {
	logger.Info("installUOSReleaseNote begin")
	bExists, _ := m.PackageExists(uosReleaseNotePkgName)
	if bExists {
		for _, v := range m.updater.UpdatablePackages {
			if v == uosReleaseNotePkgName {
				_, err := m.installPkg("", uosReleaseNotePkgName, nil)
				if err != nil {
					logger.Warning(err)
				}
				break
			}
		}
	} else {
		bInstalled := system.QueryPackageInstallable(uosReleaseNotePkgName)
		if bInstalled {
			_, err := m.installPkg("", uosReleaseNotePkgName, nil)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

func (m *Manager) handlePackagesDownloaded(sender dbus.Sender, updateType system.UpdateType) {
	m.inhibitAutoQuitCountAdd()
	defer m.inhibitAutoQuitCountSub()

	onBattery, err := m.sysPower.OnBattery().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	batteryPercentage, err := m.sysPower.BatteryPercentage().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	if onBattery && batteryPercentage > 50 || !onBattery {
		canBackup, err := m.abRecovery.CanBackup(0)
		if err != nil {
			logger.Warning(err)
			return
		}
		if canBackup {
			hasBackedUp, err := m.abRecovery.HasBackedUp().Get(0)
			if err != nil {
				logger.Warning(err)
				return
			}
			if hasBackedUp { // 本次开机,已经完成过备份,则无需再进行备份
				_, err := m.createClassifiedUpgradeJob(sender, updateType)
				if err != nil {
					logger.Warning(err)
					return
				}
			} else {
				m.PropsMu.Lock()
				m.autoInstallType |= updateType
				m.PropsMu.Unlock()
				isBackingUp, err := m.abRecovery.BackingUp().Get(0)
				if err != nil {
					logger.Warning(err)
					return
				}
				if isBackingUp {
					return
				}
				err = m.abRecovery.StartBackup(0)
				if err != nil {
					logger.Warning(err)
					return
				}
				m.inhibitAutoQuitCountAdd()
				m.PropsMu.Lock()
				m.shouldHandleBackupJobEnd = true
				m.PropsMu.Unlock()
			}
		} else {
			_, err := m.createClassifiedUpgradeJob(sender, updateType)
			if err != nil {
				logger.Warning(err)
				return
			}
		}
	}
}

func getNeedCleanCacheSize() (float64, error) {
	output, err := exec.Command("/usr/bin/lastore-apt-clean", "-print-json").Output()
	if err != nil {
		return 0, err
	}
	var archivesInfo map[string]json.RawMessage
	err = json.Unmarshal(output, &archivesInfo)
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseFloat(string(archivesInfo["total"]), 64)
	if err != nil {
		return 0, err
	}
	return size, nil
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

var _securityConfigUpdateMu sync.Mutex

// 在控制中心打开仅安全更新时,在apt配置文件中增加参数,用户使用命令行安装更新时,也同样仅会进行安全更新
func updateSecurityConfigFile(create bool) error {
	_securityConfigUpdateMu.Lock()
	defer _securityConfigUpdateMu.Unlock()
	configPath := path.Join(aptConfDir, securityConfFileName)
	if create {
		_, err := os.Stat(configPath)
		if err == nil {
			return nil
		}
		configContent := []string{
			`Dir::Etc::SourceParts "/dev/null";`,
			fmt.Sprintf(`Dir::Etc::SourceList "/etc/apt/sources.list.d/%v";`, system.SecurityList),
		}
		config := strings.Join(configContent, "\n")
		err = ioutil.WriteFile(configPath, []byte(config), 0644)
		if err != nil {
			return err
		}
	} else {
		err := os.RemoveAll(configPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) initAutoInstall(conn *dbus.Conn) {
	const jobKindBackup = "backup"
	m.sysPower = power.NewPower(conn)
	m.abRecovery = abrecovery.NewABRecovery(conn)
	m.signalLoop = dbusutil.NewSignalLoop(conn, 10)
	m.signalLoop.Start()
	m.abRecovery.InitSignalExt(m.signalLoop, true)
	_, _ = m.abRecovery.ConnectJobEnd(func(kind string, success bool, errMsg string) {
		m.PropsMu.RLock()
		updateType := m.autoInstallType
		shouldHandleBackupJobEnd := m.shouldHandleBackupJobEnd
		m.PropsMu.RUnlock()
		if kind == jobKindBackup && shouldHandleBackupJobEnd {
			if success {
				m.PropsMu.Lock()
				m.autoInstallType = 0
				m.shouldHandleBackupJobEnd = false
				m.PropsMu.Unlock()
				_, err := m.classifiedUpgrade(dbus.Sender(m.service.Conn().Names()[0]), updateType, true, false)
				if err != nil {
					logger.Warning(err)
				}
			}
			m.inhibitAutoQuitCountSub()
		}
	})
	canRestore, err := m.abRecovery.CanRestore(0) // 如果初始化时系统处于可回退状态,则不进行自动安装更新（true： 不自动安装）
	if err != nil {
		m.PropsMu.Lock()
		m.canRestore = true
		m.PropsMu.Unlock()
	} else {
		m.PropsMu.Lock()
		m.canRestore = canRestore
		m.PropsMu.Unlock()
	}
}

//SystemUpgradeInfo 将update_infos.json数据解析成map
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

	m.PropsMu.RLock()
	canRestore := m.canRestore
	m.PropsMu.RUnlock()
	return !canRestore && autoInstallUpdates && (autoInstallUpdateType&category != 0)
}

func (m *Manager) HandleSystemEvent(sender dbus.Sender, eventType string) *dbus.Error {
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if uid != 0 {
		err = fmt.Errorf("%q is not allowed to trigger system event", uid)
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	m.service.DelayAutoQuit()
	switch eventType {
	case "AutoCheck":
		go m.handleAutoCheckEvent()
	case "AutoClean":
		go m.handleAutoCleanEvent()
	case "UpdateInfosChanged":
		m.handleUpdateInfosChanged(false)
	case "OsVersionChanged":
		updateTokenConfigFile()
	default:
		return dbusutil.ToError(fmt.Errorf("can not handle %s event", eventType))
	}

	return nil
}

func (m *Manager) handleAutoCheckEvent() error {
	if m.config.AutoCheckUpdates {
		_, err := m.updateSource(m.updater.UpdateNotify, true)
		if err != nil {
			logger.Warning(err)
			return err
		}
		err = m.config.UpdateLastCheckTime()
		if err != nil {
			logger.Warning(err)
			return err
		}
		m.updateAutoCheckSystemUnit()
	}
	if !m.config.DisableUpdateMetadata {
		startUpdateMetadataInfoService()
	}
	return nil
}

func (m *Manager) handleAutoCleanEvent() error {

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
	if upgradeStatus == upgradeFailed {
		upgradeErrorMsg = j.Description
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
	}
	content, err := json.Marshal(postContent)
	if err != nil {
		logger.Warning(err)
		return
	}
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
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

var _urlReg = regexp.MustCompile(`^[ ]*deb .*((?:https?|ftp|file)://[^ ]+)`)

// 获取list文件或list.d文件夹中所有list文件的未被屏蔽的仓库地址
func getUpgradeUrls(path string) []string {
	var upgradeUrls []string
	info, err := os.Stat(path)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	if info.IsDir() {
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			logger.Warning(err)
			return nil
		}
		for _, info := range infos {
			upgradeUrls = append(upgradeUrls, getUpgradeUrls(filepath.Join(path, info.Name()))...)
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			logger.Warning(err)
			return nil
		}
		defer f.Close()
		r := bufio.NewReader(f)
		for {
			s, err := r.ReadString('\n')
			if err != nil {
				break
			}
			allMatchedString := _urlReg.FindAllStringSubmatch(s, -1)
			for _, MatchedString := range allMatchedString {
				if len(MatchedString) == 2 {
					upgradeUrls = append(upgradeUrls, MatchedString[1])
				}
			}
		}
	}
	return upgradeUrls
}

type lastoreSystemUnit struct {
	UnitName string
	Args     []string
}

const (
	lastoreUnitCache    = "/tmp/lastoreUnitCache"
	lastoreJobCacheJson = "/tmp/lastoreJobCache.json"
	run                 = "systemd-run"
	lastoreDBusCmd      = "dbus-send --system --print-reply --dest=com.deepin.lastore /com/deepin/lastore com.deepin.lastore.Manager.HandleSystemEvent"
)

func (m *Manager) getNextUpdateDelay() time.Duration {
	elapsed := time.Since(m.config.LastCheckTime)
	remained := m.config.CheckInterval - elapsed
	if remained < 0 {
		return 0
	}
	// ensure delay at least have 10 seconds
	return remained + time.Second*10

}

type lastoreUnitMap map[string][]string

// 定时任务和文件监听
func (m *Manager) getLastoreSystemUnitMap() lastoreUnitMap {
	unitMap := make(lastoreUnitMap)
	unitMap["lastoreOnline"] = []string{
		"/bin/bash",
		"-c",
		fmt.Sprintf("/usr/bin/nm-online -t 3600 && %s string:%s", lastoreDBusCmd, "AutoCheck"), // 等待网络联通后检查更新
	}
	unitMap["lastoreAutoClean"] = []string{
		"--on-active=600",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, "AutoClean"), // 10分钟后自动检查是否需要清理
	}
	unitMap["lastoreAutoCheck"] = []string{
		fmt.Sprintf("--on-active=%d", m.getNextUpdateDelay()/time.Second),
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, "AutoCheck"), // 根据上次检查时间,设置下一次自动检查时间
	}
	unitMap["lastoreAutoUpdateToken"] = []string{
		"--on-active=60",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, "OsVersionChanged"), // 60s后更新token文件
	}
	unitMap["watchOsVersion"] = []string{
		"--path-property=PathModified=/etc/os-version",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, "OsVersionChanged"), // 监听os-version文件，更新token
	}
	unitMap["watchUpdateInfo"] = []string{
		"--path-property=PathModified=/var/lib/lastore/update_infos.json",
		"/bin/bash",
		"-c",
		fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, "UpdateInfosChanged"), //监听update_infos.json文件
	}
	return unitMap
}

// systemd计时服务需要根据上一次更新时间而变化
func (m *Manager) updateAutoCheckSystemUnit() {
	const autoCheckUnit = "lastoreAutoCheck.timer"
	systemd := systemd1.NewManager(m.service.Conn())
	_, err := systemd.GetUnit(0, autoCheckUnit)
	if err == nil {
		_, err = systemd.StopUnit(0, autoCheckUnit, "replace")
		if err != nil {
			logger.Warning(err)
			return
		}
	}
	var args []string
	args = append(args, fmt.Sprintf("--unit=%s", "lastoreAutoCheck"))
	autoCheckArgs := m.getLastoreSystemUnitMap()["lastoreAutoCheck"]
	args = append(args, autoCheckArgs...)
	cmd := exec.Command(run, args...)
	var errBuffer bytes.Buffer
	cmd.Stderr = &errBuffer
	err = cmd.Run()
	if err != nil {
		logger.Warning(err)
		logger.Warning(errBuffer.String())
	}
	logger.Debug(cmd.String())
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
		kf.SetString("UnitName", name, fmt.Sprintf("%s.unit", name))
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
	return !haveActiveJob && inhibitAutoQuitCount == 0
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
	AutoCheck bool
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
		if j.Status == system.FailedStatus {
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
		} else if j.Status == system.PausedStatus {
			var updateType system.UpdateType
			switch j.Id {
			case genJobId(system.PrepareSystemUpgradeJobType), genJobId(system.SystemUpgradeJobType):
				updateType = system.SystemUpdate
			case genJobId(system.PrepareSecurityUpgradeJobType), genJobId(system.SecurityUpgradeJobType):
				updateType = system.OnlySecurityUpdate
			case genJobId(system.PrepareUnknownUpgradeJobType), genJobId(system.UnknownUpgradeJobType):
				updateType = system.UnknownUpdate
			default: // lastore目前是对控制中心提供功能，任务暂停场景只有三种类型的分类更新（下载）
				continue
			}
			_, err := m.classifiedUpgrade(dbus.Sender(m.service.Conn().Names()[0]), updateType, j.HaveNext, j.AutoCheck)
			if err != nil {
				logger.Warning(err)
				return
			}
			pausedJob := m.jobManager.findJobById(j.Id)
			if pausedJob != nil {
				err := m.jobManager.pauseJob(pausedJob)
				if err != nil {
					logger.Warning(err)
				}
				pausedJob.Progress = j.Progress
			}
		} else {
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
				job.autoCheck,
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
