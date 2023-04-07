// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"internal/system"
	"internal/utils"

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
	jobObj, err := m.distUpgrade(sender, mode, false, true)
	if err != nil && err != JobExistError {
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
		_, allPackageSize, err = system.QueryPackageDownloadSize(system.AllUpdate, packages...)
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
		m.userAgents.addLang(uidStr, getLang(envVars))
	}
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

	return jobObj.getPath(), nil
}

func (m *Manager) DistUpgradePartly(sender dbus.Sender, mode system.UpdateType, needBackup bool) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	// 创建job，但是不添加到任务队列中
	var upgradeJob *Job
	var createJobErr error
	var startJobErr error
	upgradeJob, createJobErr = m.distUpgrade(sender, mode, false, false)
	if createJobErr != nil {
		logger.Warning(createJobErr)
		return "", dbusutil.ToError(createJobErr)
	}
	var inhibitFd dbus.UnixFD = -1
	why := Tr("Installing updates...")
	inhibit := func(enable bool) {
		if enable {
			if inhibitFd == -1 {
				fd, err := Inhibitor("shutdown:sleep", dbusServiceName, why)
				if err != nil {
					logger.Infof("prevent shutdown failed: fd:%v, err:%v\n", fd, err)
				} else {
					logger.Infof("prevent shutdown: fd:%v\n", fd)
					inhibitFd = fd
				}
			}
		} else {
			if inhibitFd == -1 {
				err := syscall.Close(int(inhibitFd))
				if err != nil {
					logger.Infof("enable shutdown failed: fd:%d, err:%s\n", inhibitFd, err)
				} else {
					logger.Info("enable shutdown")
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

	// 对hook进行包装:增加配置状态更新的操作
	upgradeJob.wrapHooks(map[string]func(){
		string(system.EndStatus): func() {
			m.statusManager.setRunningUpgradeStatus(false)
		},
		string(system.SucceedStatus): func() {
			inhibit(false)
		},
		string(system.FailedStatus): func() {
			inhibit(false)
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
		}
	}()
	m.statusManager.setRunningUpgradeStatus(true)
	if needBackup {
		m.statusManager.setABStatus(system.NotBackup, system.NoABError)
		canBackup, abErr = m.abObj.CanBackup(0)
		if abErr != nil || !canBackup {
			logger.Info("can not backup,", abErr)

			msg := gettext.Tr("备份失败")
			action := []string{"continue", gettext.Tr("继续更新")}
			hints := map[string]dbus.Variant{"x-deepin-action-continue": dbus.MakeVariant(
				fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, false))}
			m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)

			m.inhibitAutoQuitCountSub()
			m.statusManager.setRunningUpgradeStatus(false)
			m.statusManager.setABStatus(system.BackupFailed, system.CanNotBackup)
			abErr = errors.New("can not backup")
			return "", dbusutil.ToError(abErr)
		}
		hasBackedUp, err = m.abObj.HasBackedUp().Get(0)
		if err != nil {
			logger.Warning(err)
		} else {
			m.statusManager.setABStatus(system.HasBackedUp, system.NoABError)
		}
		if !hasBackedUp {
			// 没有备份过，先备份再更新
			abErr = m.abObj.StartBackup(0)
			if abErr != nil {
				logger.Warning(abErr)

				msg := gettext.Tr("备份失败")
				action := []string{"backup", gettext.Tr("重新备份"), "continue", gettext.Tr("继续更新")}
				hints := map[string]dbus.Variant{
					"x-deepin-action-backup": dbus.MakeVariant(
						fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, true)),
					"x-deepin-action-continue": dbus.MakeVariant(
						fmt.Sprintf("dbus-send,--system,--print-reply,--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, false))}
				m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)

				m.inhibitAutoQuitCountSub()
				m.statusManager.setRunningUpgradeStatus(false)
				m.statusManager.setABStatus(system.BackupFailed, system.OtherError)
				return "", dbusutil.ToError(abErr)
			}
			m.statusManager.setABStatus(system.BackingUp, system.NoABError)
			abHandler, err = m.abObj.ConnectJobEnd(func(kind string, success bool, errMsg string) {
				if kind == "backup" {
					m.abObj.RemoveHandler(abHandler)
					if success {
						m.statusManager.setABStatus(system.HasBackedUp, system.NoABError)
						// 开始更新
						startJobErr = startUpgrade()
						if startJobErr != nil {
							logger.Warning(startJobErr)
						}
					} else {
						m.statusManager.setABStatus(system.BackupFailed, system.OtherError)
						logger.Warning("ab backup failed:", errMsg)

						msg := gettext.Tr("备份失败")
						action := []string{"backup", gettext.Tr("重新备份"), "continue", gettext.Tr("继续更新")}
						hints := map[string]dbus.Variant{
							"x-deepin-action-backup": dbus.MakeVariant(
								fmt.Sprintf("dbus-send,--system,--print-reply,"+
									"--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, true)),
							"x-deepin-action-continue": dbus.MakeVariant(
								fmt.Sprintf("dbus-send,--system,--print-reply,"+
									"--dest=com.deepin.lastore,/com/deepin/lastore,com.deepin.lastore.Manager.DistUpgradePartly,uint64:%v,boolean:%v", mode, false))}
						m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)

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

func (m *Manager) QueryAllSizeWithSource(mode system.UpdateType) (int64, *dbus.Error) {
	var sourcePathList []string
	for _, t := range system.AllUpdateType() {
		category := mode & t
		if category != 0 {
			sourcePath := system.GetCategorySourceMap()[category]
			sourcePathList = append(sourcePathList, sourcePath)
		}
	}
	_, allSize, err := system.QuerySourceDownloadSize(mode)
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
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}
