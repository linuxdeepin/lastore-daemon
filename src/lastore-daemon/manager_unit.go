// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"fmt"
	"internal/config"
	"internal/system"
	"internal/updateplatform"
	"math/rand"
	"os/exec"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
)

const (
	lastoreUnitCache = "/tmp/lastoreUnitCache"
	run              = "systemd-run"
	lastoreDBusCmd   = "dbus-send --system --print-reply --dest=com.deepin.lastore /com/deepin/lastore com.deepin.lastore.Manager.HandleSystemEvent"
)

// isFirstBoot startOfflineTask执行前执行有效
func isFirstBoot() bool {
	return !system.NormalFileExists(lastoreUnitCache)
}

type systemdEventType string

const (
	AutoCheck          systemdEventType = "AutoCheck"
	AutoClean          systemdEventType = "AutoClean"
	UpdateInfosChanged systemdEventType = "UpdateInfosChanged"
	OsVersionChanged   systemdEventType = "OsVersionChanged"
	AutoDownload       systemdEventType = "AutoDownload"
	AbortAutoDownload  systemdEventType = "AbortAutoDownload"
	UpdateTimer        systemdEventType = "UpdateTimer"
)

type UnitName string

const (
	lastoreOnline            UnitName = "lastoreOnline"
	lastoreAutoClean         UnitName = "lastoreAutoClean"
	lastoreAutoCheck         UnitName = "lastoreAutoCheck"
	lastoreAutoUpdateToken   UnitName = "lastoreAutoUpdateToken"
	watchOsVersion           UnitName = "watchOsVersion"
	lastoreAutoDownload      UnitName = "lastoreAutoDownload"
	lastoreAbortAutoDownload UnitName = "lastoreAbortAutoDownload"
	lastoreRegularlyUpdate   UnitName = "lastoreRegularlyUpdate" // 到触发时间后开始检查更新->下载更新->安装更新
	lastoreCronCheck         UnitName = "lastoreCronCheck"
	lastorePostUpgrade       UnitName = "lastorePostUpgrade"
)

type lastoreUnitMap map[UnitName][]string

// 定时任务和文件监听
func (m *Manager) getLastoreSystemUnitMap() lastoreUnitMap {
	unitMap := make(lastoreUnitMap)
	if (m.config.GetLastoreDaemonStatus() & config.DisableUpdate) == 0 { // 更新禁用未开启时
		unitMap[lastoreOnline] = []string{
			// 随机数范围1800-21600，时间为0.5~6小时
			fmt.Sprintf("--on-active=%d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(m.config.StartCheckRange[1]-m.config.StartCheckRange[0])+m.config.StartCheckRange[0]),
			"/bin/bash",
			"-c",
			fmt.Sprintf("/usr/bin/nm-online -t 3600 && %s string:%s", lastoreDBusCmd, AutoCheck), // 等待网络联通后检查更新
		}
		unitMap[lastoreAutoCheck] = []string{
			// 随机数范围1800-21600，时间为0.5~6小时
			fmt.Sprintf("--on-active=%d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(m.config.StartCheckRange[1]-m.config.StartCheckRange[0])+m.config.StartCheckRange[0]),
			"/bin/bash",
			"-c",
			fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, AutoCheck), // 根据上次检查时间,设置下一次自动检查时间
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
	unitMap[lastorePostUpgrade] = []string{
		fmt.Sprintf("--unit=%s", lastorePostUpgrade),
		fmt.Sprintf(`--on-calendar=%v`, m.config.PostUpgradeCron),
		"/bin/bash",
		"-c",
		"lastore-tools postupgrade", // 定时上报更新结果
	}
	if m.updater.getIdleDownloadEnabled() {
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
	updateTime, err := time.Parse(time.RFC3339, m.config.UpdateTime)
	if err == nil {
		nowTime := time.Now()
		updateTime = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), updateTime.Hour(), updateTime.Minute(), 0, 0, nowTime.Location())
		if updateTime.Before(nowTime) {
			updateTime = updateTime.Add(time.Duration(24) * time.Hour)
		}
		// 提前60s触发
		unitMap[lastoreRegularlyUpdate] = []string{
			fmt.Sprintf("--on-active=%d", int(updateTime.Sub(nowTime)/time.Second-60)),
			"/bin/bash",
			"-c",
			fmt.Sprintf(`%s string:"%s"`, lastoreDBusCmd, AutoCheck),
		}
	}

	return unitMap
}

// 开启定时任务和文件监听(通过systemd-run实现)
func (m *Manager) startOfflineTask() {
	m.lastoreUnitCacheMu.Lock()
	defer m.lastoreUnitCacheMu.Unlock()

	if !isFirstBoot() {
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
	// 如果关闭闲时更新，需要终止下载job
	// if !m.updater.getIdleDownloadEnabled() {
	// 	m.handleAbortAutoDownload()
	// }
	return nil
}

// systemd计时服务需要根据上一次更新时间而变化
func (m *Manager) updateAutoCheckSystemUnit() error {
	err := m.stopTimerUnit(lastoreOnline)
	if err != nil {
		logger.Info(err)
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

func (m *Manager) startCheckPolicyTask() {
	if len(m.config.CheckPolicyCron) == 0 {
		logger.Info("config: not CheckPolicyCron")
		return
	}
	args := []string{
		fmt.Sprintf("--unit=%s", lastoreCronCheck),
		fmt.Sprintf(`--on-calendar=%v`, m.config.CheckPolicyCron),
		"/bin/bash",
		"-c",
		"lastore-tools checkpolicy", // 定时检查policy变化
	}
	cmd := exec.Command(run, args...)
	logger.Info(cmd.String())
	var errBuffer bytes.Buffer
	cmd.Stderr = &errBuffer
	err := cmd.Run()
	if err != nil {
		logger.Warning(err)
		logger.Warning(errBuffer.String())
	}
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

func (m *Manager) getNextUpdateDelay() time.Duration {
	elapsed := time.Since(m.config.LastCheckTime)
	remained := m.config.CheckInterval - elapsed
	if remained < 0 {
		return _minDelayTime
	}
	// ensure delay at least have 10 seconds
	return remained + _minDelayTime
}

func (m *Manager) handleSystemEvent(sender dbus.Sender, eventType string) error {
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if uid != 0 && systemdEventType(eventType) != OsVersionChanged {
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
			m.startCheckPolicyTask() // 在第一次自动检查更新后再加任务
		}()
	case AutoClean:
		go func() {
			err := m.handleAutoCleanEvent()
			if err != nil {
				logger.Warning(err)
			}
		}()
	// case UpdateInfosChanged:
	// 	logger.Info("UpdateInfos Changed")
	// 	m.handleUpdateInfosChanged()
	case OsVersionChanged:
		go updateplatform.UpdateTokenConfigFile(m.config.IncludeDiskInfo)
	case AutoDownload:
		if m.updater.getIdleDownloadEnabled() { // 如果自动下载关闭,则空闲下载同样会关闭
			m.handleAutoDownload()
			go func() {
				err := m.updateAutoDownloadTimer()
				if err != nil {
					logger.Warning(err)
				}
			}()
		}
	case AbortAutoDownload:
		if m.updater.getIdleDownloadEnabled() {
			m.handleAbortAutoDownload()
			go func() {
				err := m.updateAutoDownloadTimer()
				if err != nil {
					logger.Warning(err)
				}
			}()
		}
	case UpdateTimer:
		go func() {
			// 触发检查更新，检查更新时会处理定时更新
			_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]), m.updater.UpdateNotify)
			if err != nil {
				logger.Warning(err)
			}
		}()
	default:
		return dbusutil.ToError(fmt.Errorf("can not handle %s event", eventType))
	}

	return nil
}
