// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strconv"
	"strings"

	"internal/system"
	"internal/utils"

	"github.com/godbus/dbus"
	agent "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
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
	jobObj, err := m.distUpgrade(sender, mode, false)
	if err != nil {
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
	typ := systemdEventType(eventType)
	switch typ {
	case AutoCheck:
		go func() {
			err := m.handleAutoCheckEvent()
			if err != nil {
				logger.Warning(err)
			}
		}()
	case AutoClean:
		go func() {
			err := m.handleAutoCleanEvent()
			if err != nil {
				logger.Warning(err)
			}
		}()
	case UpdateInfosChanged:
		m.handleUpdateInfosChanged()
	case OsVersionChanged:
		go updateTokenConfigFile()
	case AutoDownload:
		m.updater.PropsMu.RLock()
		enable := m.updater.idleDownloadConfigObj.IdleDownloadEnabled // 如果自动下载关闭,则空闲下载同样会关闭
		m.updater.PropsMu.RUnlock()
		if enable {
			m.handleAutoDownload()
			go func() {
				err := m.updateAutoDownloadTimer()
				if err != nil {
					logger.Warning(err)
				}
			}()
		}
	case AbortAutoDownload:
		m.updater.PropsMu.RLock()
		enable := m.updater.idleDownloadConfigObj.IdleDownloadEnabled
		m.updater.PropsMu.RUnlock()
		if enable {
			m.handleAbortAutoDownload()
			go func() {
				err := m.updateAutoDownloadTimer()
				if err != nil {
					logger.Warning(err)
				}
			}()
		}
	default:
		return dbusutil.ToError(fmt.Errorf("can not handle %s event", eventType))
	}

	return nil
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

func (m *Manager) PackagesSize(packages []string) (int64, *dbus.Error) {
	m.service.DelayAutoQuit()
	m.ensureUpdateSourceOnce()
	var err error
	var size float64
	if packages == nil || len(packages) == 0 { // 如果传的参数为空,则根据updateMode获取所有需要下载包的大小
		m.PropsMu.RLock()
		mode := m.UpdateMode
		m.PropsMu.RUnlock()
		size, err = system.QuerySourceDownloadSize(mode, true)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		size, err = system.QueryPackageDownloadSize(true, packages...)
	}
	if err != nil || size == system.SizeUnknown {
		logger.Warningf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), size, err)
	}

	return int64(size), dbusutil.ToError(err)
}

func (m *Manager) PackagesDownloadSize(packages []string) (int64, *dbus.Error) {
	m.service.DelayAutoQuit()
	m.ensureUpdateSourceOnce()
	var err error
	var size float64
	if packages == nil || len(packages) == 0 { // 如果传的参数为空,则根据updateMode获取所有需要下载包的大小
		m.PropsMu.RLock()
		mode := m.UpdateMode
		m.PropsMu.RUnlock()
		size, err = system.QuerySourceDownloadSize(mode, false)
		if err != nil {
			logger.Warning(err)
		} else {
			m.PropsMu.Lock()
			updatablePackages := m.UpgradableApps
			m.PropsMu.Unlock()
			err = m.config.UpdateLastoreDaemonStatus(canUpgrade, size == 0 && len(updatablePackages) != 0)
			if err != nil {
				logger.Warning(err)
			}
		}
	} else {
		size, err = system.QueryPackageDownloadSize(false, packages...)
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
	mode := m.UpdateMode
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
	// 更新LANG
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	proc := procfs.Process(pid)
	envVars, err := proc.Environ()
	if err != nil {
		logger.Warningf("failed to get process %d environ: %v", proc, err)
	} else {
		m.userAgents.addLang(uidStr, envVars.Get("LANG"))
	}
	return nil
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
	}
	return dbusutil.ToError(err)
}

func (m *Manager) UnRegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	uidStr := strconv.Itoa(int(uid))
	err = m.userAgents.removeAgent(uidStr, path)
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

	_ = m.config.UpdateLastCheckTime()
	err = m.updateAutoCheckSystemUnit()
	if err != nil {
		logger.Warning(err)
	}
	return jobObj.getPath(), nil
}
