// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"internal/system"
	"internal/utils"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/godbus/dbus"
	agent "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/procfs"
)

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func (m *Manager) ClassifiedUpgrade(sender dbus.Sender, updateType system.UpdateType) ([]dbus.ObjectPath, *dbus.Error) {
	m.service.DelayAutoQuit()
	return m.classifiedUpgrade(sender, updateType, true)
}

func (m *Manager) CleanArchives() (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.cleanArchives(false)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) CleanJob(jobId string) *dbus.Error {
	m.service.DelayAutoQuit()
	m.do.Lock()
	err := m.jobManager.CleanJob(jobId)
	// 在clean后需要执行一次dispatch,将end状态的job清除,防止重新创建时出现异常
	m.jobManager.dispatch()
	m.do.Unlock()
	if err != nil {
		logger.Warningf("CleanJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) DistUpgrade(sender dbus.Sender) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	m.PropsMu.RLock()
	mode := m.UpdateMode
	m.PropsMu.RUnlock()
	jobObj, err := m.distUpgrade(sender, mode, false, true, false)
	if err != nil && !errors.Is(err, JobExistError) {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) FixError(sender dbus.Sender, errType string) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.fixError(sender, errType)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) GetArchivesInfo() (info string, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	info, err := getArchiveInfo()
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return info, nil
}

func (m *Manager) HandleSystemEvent(sender dbus.Sender, eventType string) *dbus.Error {
	return dbusutil.ToError(m.handleSystemEvent(sender, eventType))
}

func (m *Manager) InstallPackage(sender dbus.Sender, jobName string, packages string) (job dbus.ObjectPath,
	busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
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

func (m *Manager) PackageExists(pkgId string) (exist bool, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return system.QueryPackageInstalled(pkgId), nil
}

func (m *Manager) PackageInstallable(pkgId string) (installable bool, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return system.QueryPackageInstallable(pkgId), nil
}

func (m *Manager) UpdatablePackages(updateType string) (pkgs []string, busErr *dbus.Error) {
	switch updateType {
	case system.SystemUpdate.JobType():
		return m.updater.ClassifiedUpdatablePackages[updateType], nil
	case system.SecurityUpdate.JobType():
		return m.updater.ClassifiedUpdatablePackages[updateType], nil
	default:
		return nil, dbusutil.ToError(fmt.Errorf("%s", "Unknown update type"))
	}
}

func (m *Manager) GetUpdateLogs(updateType system.UpdateType) (changeLogs string, busErr *dbus.Error) {
	res := make(map[system.UpdateType]string)
	if updateType&system.SystemUpdate != 0 {
		res[system.SystemUpdate] = m.updatePlatform.GetSystemUpdateLogs()
	}

	if updateType&system.SecurityUpdate != 0 {
		res[system.SecurityUpdate] = m.updatePlatform.GetCVEUpdateLogs(m.allUpgradableInfo[system.SecurityUpdate])
	}

	if len(res) == 0 {
		return "", dbusutil.ToError(fmt.Errorf("%s", "Unknown update type"))
	}

	logs, err := json.Marshal(res)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	return string(logs), nil
}

// GetHistoryLogs changeLogs json解析后数据结构
// type recordInfo struct {
//	UUID        string
//	UpgradeTime string
//	UpgradeMode system.UpdateType
//	ChangelogEn []string
//	ChangelogZh []string
// }

func (m *Manager) GetHistoryLogs() (changeLogs string, busErr *dbus.Error) {
	return getHistoryChangelog(upgradeRecordPath), nil
}

func (m *Manager) PackagesSize(packages []string) (int64, *dbus.Error) {
	m.service.DelayAutoQuit()
	m.ensureUpdateSourceOnce()
	var err error
	var allPackageSize float64
	m.PropsMu.RLock()
	mode := m.UpdateMode
	m.PropsMu.RUnlock()
	if packages == nil || len(packages) == 0 { // 如果传的参数为空,则根据updateMode获取所有需要下载包的大小
		_, allPackageSize, err = system.QuerySourceDownloadSize(mode)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		// 查询包(可能不止一个)的大小,即使当前开启的仓库没有包含该包,依旧返回该包的大小
		_, allPackageSize, err = system.QueryPackageDownloadSize(system.AllCheckUpdate, packages...)
	}
	if err != nil || allPackageSize == system.SizeUnknown {
		logger.Warningf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), allPackageSize, err)
	}

	return int64(allPackageSize), dbusutil.ToError(err)
}

func (m *Manager) PackagesDownloadSize(packages []string) (int64, *dbus.Error) {
	m.service.DelayAutoQuit()
	m.ensureUpdateSourceOnce()
	var err error
	var size float64
	m.PropsMu.RLock()
	mode := m.UpdateMode
	m.PropsMu.RUnlock()
	if packages == nil || len(packages) == 0 { // 如果传的参数为空,则根据updateMode获取所有需要下载包的大小
		size, _, err = system.QuerySourceDownloadSize(mode)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		// 查询包(可能不止一个)需要下载的大小,如果当前打开的仓库没有该包,则返回0
		size, _, err = system.QueryPackageDownloadSize(mode, packages...)
	}
	if err != nil || size == system.SizeUnknown {
		logger.Warningf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), size, err)
	}

	return int64(size), dbusutil.ToError(err)
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

func (m *Manager) PrepareDistUpgrade(sender dbus.Sender) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	m.PropsMu.RLock()
	mode := m.CheckUpdateMode
	m.PropsMu.RUnlock()
	jobObj, err := m.prepareDistUpgrade(sender, mode, false)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) RegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	logger.Infof("Register lastore agent form %v, sender:%v.", path, sender)
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	uidStr := strconv.Itoa(int(uid))
	m.userAgents.addUser(uidStr)

	sessionDetails, err := m.loginManager.ListSessions(0)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	sysBus := m.service.Conn()
	// 更新LANG
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	proc := procfs.Process(pid)
	envVars, err := proc.Environ()
	logger.Infof(" agent envVars: %+v", getLang(envVars))
	if err != nil {
		logger.Warningf("failed to get process %d environ: %v", proc, err)
	} else {
		m.userAgents.addLang(uidStr, getLang(envVars))
		gettext.SetLocale(gettext.LcAll, m.userAgents.getActiveLastoreAgentLang())
	}

	for _, detail := range sessionDetails {
		if detail.UID == uid {
			session, err := login1.NewSession(sysBus, detail.Path)
			if err != nil {
				logger.Warning(err)
				continue
			}
			newlyAdded := m.userAgents.addSession(uidStr, session)
			if newlyAdded {
				m.watchSession(uidStr, session)
			}
		}
	}

	a, err := agent.NewAgent(m.service.Conn(), string(sender), path)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	m.userAgents.addAgent(uidStr, a)
	m.saveLastoreCache()
	return nil
}

func (m *Manager) RemovePackage(sender dbus.Sender, jobName string, packages string) (job dbus.ObjectPath,
	busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
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

func (m *Manager) SetRegion(region string) *dbus.Error {
	m.service.DelayAutoQuit()
	err := m.config.SetAppstoreRegion(region)
	return dbusutil.ToError(err)
}

func (m *Manager) StartJob(jobId string) *dbus.Error {
	m.service.DelayAutoQuit()
	m.do.Lock()
	err := m.jobManager.MarkStart(jobId)
	m.do.Unlock()

	if err != nil {
		logger.Warningf("StartJob %q error: %v\n", jobId, err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) UnRegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	err = m.userAgents.removeAgent(strconv.Itoa(int(uid)), path)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	logger.Debugf("agent unregistered, sender: %q, agentPath: %q", sender, path)
	return nil
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

func (m *Manager) UpdateSource(sender dbus.Sender) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.updateSource(sender, false)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	return jobObj.getPath(), nil
}

func (m *Manager) DistUpgradePartly(sender dbus.Sender, mode system.UpdateType, needBackup bool) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return m.distUpgradePartly(sender, mode, needBackup)
}

func (m *Manager) PrepareFullScreenUpgrade(sender dbus.Sender, option string) *dbus.Error {
	// TODO 离线更新需要处理

	checkExecPath := func() (bool, error) {
		// 只有dde-lock可以设置
		execPath, _, err := getExecutablePathAndCmdline(m.service, sender)
		if err != nil {
			logger.Warning(err)
			return false, err
		}
		if !strings.Contains(execPath, "dde-lock") && !strings.Contains(execPath, "deepin-offline-update-tool") {
			err = fmt.Errorf("%v not allow to call this method", execPath)
			logger.Warning(err)
			return false, err
		}

		return strings.Contains(execPath, "deepin-offline-update-tool"), nil
	}
	var isOffline bool
	uid, err := m.service.GetConnUID(string(sender))
	if err == nil && uid == 0 {
		logger.Info("auth root caller")
	} else {
		isOffline, err = checkExecPath()
		if err != nil {
			return dbusutil.ToError(err)
		}
	}

	// 如果没有/usr/bin/dde-update,则需要进入fallback流程
	const fullScreenUpdatePath = "/usr/bin/dde-update"
	if !system.NormalFileExists(fullScreenUpdatePath) {
		err = fmt.Errorf("%v not exist, need run fallback process", fullScreenUpdatePath)
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	if isOffline {
		content, err := json.Marshal(&fullUpgradeOption{
			DoUpgrade:         true,
			DoUpgradeMode:     system.OfflineUpdate,
			IsPowerOff:        false,
			PreGreeterCheck:   false,
			AfterGreeterCheck: false,
		})
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		_ = ioutil.WriteFile(optionFilePathTemp, content, 0644)
	} else {
		_ = ioutil.WriteFile(optionFilePathTemp, []byte(option), 0644)
	}

	for {
		pid, err := m.service.GetConnPID(string(sender))
		if err != nil {
			logger.Warning(err)
			break
		}
		sessionPath, err := m.loginManager.GetSessionByPID(0, pid)
		if err != nil {
			logger.Warning(err)
			break
		}
		session, err := login1.NewSession(m.service.Conn(), sessionPath)
		if err != nil {
			logger.Warning(err)
			break
		}
		seatPath, err := session.Seat().Get(0)
		if err != nil {
			logger.Warning(err)
			break
		}
		seat, err := login1.NewSeat(m.service.Conn(), seatPath.Path)
		if err != nil {
			logger.Warning(err)
			break
		}
		err = seat.Terminate(0)
		if err != nil {
			logger.Warning(err)
			break
		} else {
			return nil
		}
	}

	// 如果上述方法出错，需要采用重启lightdm方案，此时所有图形session也都会退出
	_, err = m.systemd.RestartUnit(0, "lightdm.service", "replace")
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}
func (m *Manager) QueryAllSizeWithSource(mode system.UpdateType) (int64, *dbus.Error) {
	var sourcePathList []string
	for _, t := range system.AllCheckUpdateType() {
		category := mode & t
		if category != 0 {
			sourcePath := system.GetCategorySourceMap()[category]
			sourcePathList = append(sourcePathList, sourcePath)
		}
	}
	_, allSize, err := system.QueryPackageDownloadSize(mode, m.updater.getUpdatablePackagesByType(mode)...)
	if err != nil || allSize == system.SizeUnknown {
		logger.Warningf("failed to get %v source size:%v", strings.Join(sourcePathList, " and "), err)
	} else {
		logger.Infof("%v size is:%v M", strings.Join(sourcePathList, " and "), int64(allSize/(1000*1000)))
	}

	return int64(allSize), dbusutil.ToError(err)
}

func (m *Manager) PrepareDistUpgradePartly(sender dbus.Sender, mode system.UpdateType) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.prepareDistUpgrade(sender, mode, false)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) CheckUpgrade(sender dbus.Sender, checkMode system.UpdateType, checkOrder uint32) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return m.checkUpgrade(sender, checkMode, checkType(checkOrder))
}

func (m *Manager) UpdateOfflineSource(sender dbus.Sender, paths []string, option string) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()

	jobObj, err := m.updateOfflineSource(sender, paths, option)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}

	return jobObj.getPath(), nil
}
