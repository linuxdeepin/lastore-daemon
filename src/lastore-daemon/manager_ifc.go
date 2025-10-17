// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/polkit"
	agent "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.lastore1.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/procfs"
	utils2 "github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
)

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

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

func (m *Manager) FixError(sender dbus.Sender, errType string) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.delFixError(sender, errType)
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
	return dbusutil.ToError(m.delHandleSystemEvent(sender, eventType))
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
		// 白名单未通过时需要管理员鉴权
		err = polkit.CheckAuth(polkitActionChangeOwnData, string(sender), nil)
		if err != nil {
			err = fmt.Errorf("%q is not in allowed install package paths.And the caller not pass the authentication, don't allow to install packages %v", execPath, packages)
			logger.Warning(err)
			return "/", dbusutil.ToError(err)
		}
	}

	jobObj, err := m.installPackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	jobObj.next.caller = mapMethodCaller(execPath, cmdLine)
	return jobObj.getPath(), nil
}

func (m *Manager) InstallPackageFromRepo(sender dbus.Sender, jobName string, sourceListPath string, repoListPath string, cachePath string, packageName []string) (jobPath dbus.ObjectPath,
	busErr *dbus.Error) {
	logger.Infof("enter InstallPackageFromRepo,jobName:%v, sourceListPath:%v, repoListPath:%v, cachePath:%v", jobName, sourceListPath, repoListPath, cachePath)

	jobObj, err := m.delInstallPackageFromRepo(sender, jobName, sourceListPath, repoListPath, cachePath, packageName)
	if err != nil {
		logger.Error(err)
		return "/", dbusutil.ToError(err)
	}

	return jobObj.getPath(), nil
}

func (m *Manager) PackageExists(pkgId string) (exist bool, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return system.QueryPackageInstalled(pkgId), nil
}

func (m *Manager) PackageInstallable(pkgId string) (installable bool, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	return system.QueryPackageInstallable(pkgId), nil
}

func (m *Manager) GetUpdateLogs(updateType system.UpdateType) (changeLogs string, busErr *dbus.Error) {
	res := make(map[system.UpdateType]interface{})
	if updateType&system.SystemUpdate != 0 {
		res[system.SystemUpdate] = m.updatePlatform.SystemUpdateLogs
	}

	if updateType&system.SecurityUpdate != 0 {
		res[system.SecurityUpdate] = m.updatePlatform.GetCVEUpdateLogs(m.updater.getUpdatablePackagesByType(system.SecurityUpdate))
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
		_, allPackageSize, err = system.QuerySourceDownloadSize(mode, nil)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		// 查询包(可能不止一个)的大小,即使当前开启的仓库没有包含该包,依旧返回该包的大小
		_, allPackageSize, err = system.QueryPackageDownloadSize(system.AllInstallUpdate, packages...)
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
		size, _, err = system.QuerySourceDownloadSize(mode, nil)
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

	if m.config.IncrementalUpdate && size > 0 && apt.IsIncrementalUpdateCached() {
		size = 0.0
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
	//execPath, cmdLine, err := getExecutablePathAndCmdline(m.service, sender)
	//if err != nil {
	//	logger.Warning(err)
	//	return "/", dbusutil.ToError(err)
	//}
	//
	//uid, err := m.service.GetConnUID(string(sender))
	//if err != nil {
	//	logger.Warning(err)
	//	return "/", dbusutil.ToError(err)
	//}
	//
	//if !allowRemovePackageExecPaths.Contains(execPath) &&
	//	uid != 0 {
	//	err = fmt.Errorf("%q is not allowed to remove packages", execPath)
	//	logger.Warning(err)
	//	return "/", dbusutil.ToError(err)
	//}
	// TODO
	// 鉴权或者给 dde-launcher 加 loader 启动,或者是否可以给 dde-launcher setgid and set group deepin-daemon
	jobObj, err := m.removePackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	//jobObj.caller = mapMethodCaller(execPath, cmdLine)
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

func (m *Manager) UpdateSource(sender dbus.Sender) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.updateSource(sender)
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

// PrepareFullScreenUpgrade option json -> struct
//
//	type fullUpgradeOption struct {
//		DoUpgrade         bool
//		DoUpgradeMode     system.UpdateType
//		IsPowerOff        bool
//		PreGreeterCheck   bool
//		AfterGreeterCheck bool
//	}
func (m *Manager) PrepareFullScreenUpgrade(sender dbus.Sender, option string) *dbus.Error {
	supportOption := len(strings.TrimSpace(option)) > 0

	// TODO
	// 应该只有 dde-lock 会调用
	// 用鉴权方案或者给 dde-lock 增加 group
	logger.Info("start PrepareFullScreenUpgrade")

	if supportOption {
		opt := fullUpgradeOption{}
		err := json.Unmarshal([]byte(option), &opt)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		// 在线更新时填充部分属性
		opt.DoUpgrade = true
		opt.PreGreeterCheck = false
		opt.AfterGreeterCheck = false
		content, err := json.Marshal(opt)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		if utils2.IsSymlink(optionFilePathTemp) {
			_ = os.RemoveAll(optionFilePathTemp)
		}
		_ = os.WriteFile(optionFilePathTemp, content, 0644)
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
			logger.Info("terminate seat")
			return nil
		}
	}

	// 如果上述方法出错，需要采用重启lightdm方案，此时所有图形session也都会退出
	_, err := m.systemd.RestartUnit(0, "lightdm.service", "replace")
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	logger.Info("RestartUnit lightdm")
	return nil
}

func (m *Manager) QueryAllSizeWithSource(mode system.UpdateType) (int64, *dbus.Error) {
	var sourcePathList []string
	for _, t := range system.AllInstallUpdateType() {
		category := mode & t
		if category != 0 {
			sourcePath := system.GetCategorySourceMap()[category]
			sourcePathList = append(sourcePathList, sourcePath)
		}
	}
	var pkgList []string
	if mode&system.SystemUpdate != 0 {
		pkgList = m.coreList
	}
	_, allSize, err := system.QuerySourceDownloadSize(mode, pkgList)
	if err != nil || allSize == system.SizeUnknown {
		logger.Warningf("failed to get %v source size:%v", strings.Join(sourcePathList, " and "), err)
	} else {
		logger.Infof("%v size is:%v M", strings.Join(sourcePathList, " and "), int64(allSize/(1000*1000)))
	}

	return int64(allSize), dbusutil.ToError(err)
}

func (m *Manager) PrepareDistUpgradePartly(sender dbus.Sender, mode system.UpdateType) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	jobObj, err := m.prepareDistUpgrade(sender, mode, initiatorUser)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}
	return jobObj.getPath(), nil
}

func (m *Manager) CheckUpgrade(sender dbus.Sender, checkMode system.UpdateType, checkOrder uint32) (job dbus.ObjectPath, busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	job, err := m.checkUpgrade(sender, checkMode, checkType(checkOrder))
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	logger.Info("CheckUpgrade jobPath:", job)
	return job, nil
}

func (m *Manager) PowerOff(sender dbus.Sender, reboot bool) *dbus.Error {
	checkExecPath := func() error {
		// 只有dde-update可以设置
		execPath, _, err := getExecutablePathAndCmdline(m.service, sender)
		if err != nil {
			logger.Warning(err)
			return err
		}
		if strings.Contains(execPath, "dde-update") ||
			strings.Contains(execPath, "dde-rollback") {
			return nil
		} else {
			err = fmt.Errorf("%v not allow to call this method", execPath)
			logger.Warning(err)
			return err
		}
	}
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil || uid != 0 {
		err = checkExecPath()
		if err != nil {
			return dbusutil.ToError(err)
		}
	}
	args := []string{
		"-f",
	}
	if reboot {
		args = append(args, "--reboot")
	}
	cmd := exec.Command("poweroff", args...)
	logger.Info(cmd.String())
	var errBuffer bytes.Buffer
	cmd.Stderr = &errBuffer
	err = cmd.Run()
	if err != nil {
		logger.Warning(err)
		logger.Warning(errBuffer.String())
		return dbusutil.ToError(err)
	}
	return nil
}

// SetUpdateSources 设置系统、安全更新的仓库
func (m *Manager) SetUpdateSources(sender dbus.Sender, updateType system.UpdateType, repoType config.RepoType, repoConfig []string, isReset bool) *dbus.Error {
	// 管理员鉴权
	err := checkInvokePermission(m.service, sender)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	// 修改仓库后，重置可安装状态
	err = m.config.UpdateLastoreDaemonStatus(config.CanUpgrade, false)
	if err != nil {
		logger.Warning(err)
	}
	// 判断设置的源类型
	if !repoType.IsValid() {
		return dbusutil.ToError(fmt.Errorf("invalid repo type: %v", repoType))
	}
	if repoType == config.CustomRepo {
		if len(repoConfig) == 0 {
			logger.Warning("custom repo config is invalid")
			return dbusutil.ToError(errors.New("custom repo config is invalid"))
		}
		// 使用apt-get check 检查仓库时候合规
		tmpList := fmt.Sprintf("/tmp/custom_repo_%v", time.Now().Unix())
		err = os.WriteFile(tmpList, []byte(strings.Join(repoConfig, "\n")), 0600)
		if err != nil {
			logger.Warning(err)
		} else {
			o, err := exec.Command("/usr/bin/apt-get", "check", "-o", "Debug::NoLocking=1",
				"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", tmpList), "-o", "Dir::Etc::SourceParts=/dev/null").CombinedOutput()
			if err != nil {
				logger.Warning("apt-get check error", string(o))
				return dbusutil.ToError(fmt.Errorf("repo format error:%v", string(o)))
			}
		}
	}
	// 判断是系统或安全仓库，分别设置配置
	switch updateType {
	case system.SystemUpdate:
		err := m.config.SetSystemRepoType(repoType)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		if repoType == config.CustomRepo {
			if isReset {
				err = m.config.SetSystemCustomSource(repoConfig)
			} else {
				err = m.config.SetSystemCustomSource(append(m.config.SystemCustomSource, repoConfig...))
			}
			if err != nil {
				logger.Warning(err)
				return dbusutil.ToError(err)
			}
		}
	case system.SecurityUpdate:
		err := m.config.SetSecurityRepoType(repoType)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		m.config.SecurityRepoType = repoType
		if repoType == config.CustomRepo {
			if isReset {
				err = m.config.SetSecurityCustomSource(repoConfig)
			} else {
				err = m.config.SetSecurityCustomSource(append(m.config.SecurityCustomSource, repoConfig...))
			}
			if err != nil {
				logger.Warning(err)
				return dbusutil.ToError(err)
			}
		}
	default:
		return dbusutil.ToError(fmt.Errorf("not supported update type: %v to set source", updateType))
	}
	m.reloadOemConfig(false)
	return nil
}

func (m *Manager) ConfirmRollback(sender dbus.Sender, confirm bool) *dbus.Error {
	var err error
	if confirm {
		needReboot := m.immutableManager.osTreeNeedRebootAfterRollback()
		err = m.immutableManager.osTreeRollback()
		if err != nil {
			logger.Warning(err)
		}
		if m.grub != nil {
			err = m.grub.changeGrubDefaultEntry(normalBootEntry)
			if err != nil {
				logger.Warning(err)
			}
		}

		if needReboot {
			err := m.PowerOff(sender, true)
			if err != nil {
				logger.Warning(err)
			}
		}
	} else {
		return m.PowerOff(sender, true)
	}
	return nil
}

func (m *Manager) CanRollback() (bool, string, *dbus.Error) {
	can, info := m.immutableManager.osTreeCanRollback()
	return can, info, nil
}

func (m *Manager) GetUpdateDetails(sender dbus.Sender, fd dbus.UnixFD, realTime bool) (busErr *dbus.Error) {
	m.service.DelayAutoQuit()
	// default pass
	err := polkit.CheckAuth("org.deepin.dde.lastore.doAction", string(sender), nil)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	f := os.NewFile(uintptr(fd), "")
	if realTime {
		m.logFdsMu.Lock()
		m.logFds = append(m.logFds, f)
		m.logFdsMu.Unlock()
	} else {
		defer f.Close()
		// 使用流式复制，避免将整个文件读入内存
		logFile, err := os.Open(logTmpPath)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		defer logFile.Close()

		_, err = io.Copy(f, logFile)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
	}
	return nil
}
