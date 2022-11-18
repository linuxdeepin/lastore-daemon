// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"internal/system"
	"internal/utils"
	"strconv"
	"strings"

	"github.com/godbus/dbus"
	agent "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func (m *Manager) ClassifiedUpgrade(sender dbus.Sender, updateType system.UpdateType) ([]dbus.ObjectPath, *dbus.Error) {
	m.service.DelayAutoQuit()
	return m.classifiedUpgrade(sender, updateType, true, false)
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
	jobObj, err := m.distUpgrade(sender)
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
	switch eventType {
	case "AutoCheck":
		go func() {
			err := m.handleAutoCheckEvent()
			if err != nil {
				logger.Warning(err)
			}
		}()
	case "AutoClean":
		go func() {
			err := m.handleAutoCleanEvent()
			if err != nil {
				logger.Warning(err)
			}
		}()
	case "UpdateInfosChanged":
		m.handleUpdateInfosChanged(false)
	case "OsVersionChanged":
		go updateTokenConfigFile()
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

func (m *Manager) PackagesDownloadSize(packages []string) (size int64, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	m.ensureUpdateSourceOnce()

	s, err := system.QueryPackageDownloadSize(packages...)
	if err != nil || s == system.SizeUnknown {
		logger.Warningf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), s, err)
	}
	return int64(s), dbusutil.ToError(err)
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

func (m *Manager) PrepareDistUpgrade() (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.prepareDistUpgrade()
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) RegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	logger.Info("RegisterAgent:", path)
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
