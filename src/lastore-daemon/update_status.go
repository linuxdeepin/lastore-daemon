// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"internal/system"
	"sync"
)

type updateModeStatusManager struct {
	checkMode                           system.UpdateType
	updateMode                          system.UpdateType
	lsConfig                            *Config
	systemUpdateStatus                  system.UpdateModeStatus
	securityUpdateStatus                system.UpdateModeStatus
	unKnownUpdateStatus                 system.UpdateModeStatus
	updateModeStatusObj                 map[string]system.UpdateModeStatus // 每一个更新项的状态 object,在检查更新、下载更新、安装更新的过程中修改
	updateModeDownloadSizeMap           map[string]float64
	abStatus                            system.ABStatus
	abError                             system.ABErrorType
	statusMapMu                         sync.RWMutex
	handleStatusChangedCallback         func(string)
	handleSystemStatusChangedCallback   func(interface{})
	handleSecurityStatusChangedCallback func(interface{})
	handleUnKnownStatusChangedCallback  func(interface{})
	checkModeChangedCallback            func(interface{})
	updateModeChangedCallback           func(interface{})
}

type allStatus struct {
	ABStatus     system.ABStatus
	ABError      system.ABErrorType
	UpdateStatus map[string]system.UpdateModeStatus
}

func NewStatusManager(config *Config, callback func(newStatus string)) *updateModeStatusManager {
	m := &updateModeStatusManager{
		lsConfig:                    config,
		checkMode:                   config.CheckUpdateMode,
		updateMode:                  config.UpdateMode,
		handleStatusChangedCallback: callback,
	}
	return m
}

func (m *updateModeStatusManager) InitModifyData() {
	m.updateMode, m.checkMode = filterMode(m.updateMode, m.checkMode)
	err := m.lsConfig.SetUpdateMode(m.updateMode)
	if err != nil {
		logger.Warning(err)
	}
	if m.updateModeChangedCallback != nil {
		m.updateModeChangedCallback(m.updateMode)
	}
	err = m.lsConfig.SetCheckUpdateMode(m.checkMode)
	if err != nil {
		logger.Warning(err)
	}
	if m.checkModeChangedCallback != nil {
		m.checkModeChangedCallback(m.checkMode)
	}
	obj := &allStatus{
		ABStatus:     system.NotBackup,
		ABError:      system.NoABError,
		UpdateStatus: make(map[string]system.UpdateModeStatus),
	}
	m.statusMapMu.Lock()
	err = json.Unmarshal([]byte(m.lsConfig.updateStatus), &obj)
	if err != nil {
		logger.Warning(err)
		m.updateModeStatusObj = make(map[string]system.UpdateModeStatus)
		for _, typ := range system.AllUpdateType() {
			m.updateModeStatusObj[typ.JobType()] = system.NotDownload
		}
		m.abStatus = system.NotBackup
		m.abError = system.NoABError
		m.syncUpdateStatusNoLock()
	} else {
		m.updateModeStatusObj = obj.UpdateStatus
		m.abStatus = obj.ABStatus
		if isFirstBoot() {
			for key, value := range m.updateModeStatusObj {
				switch value {
				case system.Upgraded, system.IsDownloading, system.DownloadPause, system.DownloadErr:
					m.updateModeStatusObj[key] = system.NotDownload
				case system.UpgradeErr, system.Upgrading:
					m.updateModeStatusObj[key] = system.CanUpgrade
				}
			}
			m.abStatus = system.NotBackup
			m.abError = system.NoABError
		}
		m.syncUpdateStatusNoLock()
	}
	m.SetRunningUpgradeStatus(false)
	m.statusMapMu.Unlock()
	m.updateModeDownloadSizeMap = make(map[string]float64)
}

// filterMode 去除 updateMode 和 checkMode 不满足条件的数据
func filterMode(updateMode, checkMode system.UpdateType) (system.UpdateType, system.UpdateType) {
	var res0 system.UpdateType // updateMode
	var res1 system.UpdateType // checkMode
	// 过滤掉不存在的类型，updateMode没有的类型，checkMode的也需要清理
	for _, typ := range system.AllUpdateType() {
		if updateMode&typ != 0 {
			res0 |= typ
			if typ&checkMode != 0 {
				res1 |= typ
			}
		}
	}
	// 将仅安全更新迁移为安全更新
	if updateMode&system.OnlySecurityUpdate != 0 {
		err := updateSecurityConfigFile(false)
		if err != nil {
			logger.Warning(err)
		}
		res0 |= system.SecurityUpdate
	}
	if checkMode&system.OnlySecurityUpdate != 0 {
		res1 |= system.SecurityUpdate
	}

	return res0, res1
}

const (
	handlerKeyCheckMode      = "checkMode"
	handlerKeyUpdateMode     = "updateMode"
	handlerKeySystemStatus   = "SystemStatus"
	handlerKeySecurityStatus = "SecurityStatus"
	handlerKeyUnKnownStatus  = "UnKnownStatus"
)

func (m *updateModeStatusManager) RegisterChangedHandler(key string, handler func(value interface{})) {
	switch key {
	case handlerKeyCheckMode:
		m.checkModeChangedCallback = handler
	case handlerKeyUpdateMode:
		m.updateModeChangedCallback = handler
	case handlerKeySystemStatus:
		m.handleSystemStatusChangedCallback = handler
	case handlerKeySecurityStatus:
		m.handleSecurityStatusChangedCallback = handler
	case handlerKeyUnKnownStatus:
		m.handleUnKnownStatusChangedCallback = handler
	default:
		logger.Info("invalid key")
	}
}

// TODO delete
func (m *updateModeStatusManager) getUpdateStatusString() string {
	m.statusMapMu.RLock()
	defer m.statusMapMu.RUnlock()
	content, err := json.Marshal(m.updateModeStatusObj)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return string(content)
}

func (m *updateModeStatusManager) GetUpdateStatus(typ system.UpdateType) system.UpdateModeStatus {
	m.statusMapMu.RLock()
	defer m.statusMapMu.RUnlock()
	return m.updateModeStatusObj[typ.JobType()]
}

func (m *updateModeStatusManager) SetUpdateStatus(mode system.UpdateType, status system.UpdateModeStatus) {
	m.statusMapMu.Lock()
	for _, typ := range system.AllUpdateType() {
		if mode&typ != 0 {
			m.updateModeStatusObj[typ.JobType()] = status
		}
	}
	m.syncUpdateStatusNoLock()
	m.statusMapMu.Unlock()
	m.UpdateCheckCanUpgradeByEachStatus()
}

func (m *updateModeStatusManager) SetABStatus(status system.ABStatus, error system.ABErrorType) {
	if m.abStatus == status && m.abError == error {
		return
	}
	m.abStatus = status
	m.abError = error
	m.syncUpdateStatusNoLock()
}

func (m *updateModeStatusManager) SetRunningUpgradeStatus(running bool) {
	err := m.lsConfig.UpdateLastoreDaemonStatus(runningUpgradeBackend, running)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *updateModeStatusManager) syncUpdateStatusNoLock() {
	obj := &allStatus{
		ABStatus:     m.abStatus,
		ABError:      m.abError,
		UpdateStatus: m.updateModeStatusObj,
	}
	content, err := json.Marshal(obj)
	if err != nil {
		logger.Warning(err)
		return
	}
	if m.handleStatusChangedCallback != nil {
		m.handleStatusChangedCallback(string(content))
	}
	_ = m.lsConfig.SetUpdateStatus(string(content))
}

func (m *updateModeStatusManager) getUpdateMode() system.UpdateType {
	return m.updateMode
}

func (m *updateModeStatusManager) getCheckMode() system.UpdateType {
	return m.checkMode
}

func (m *updateModeStatusManager) SetUpdateMode(newWriteMode system.UpdateType) system.UpdateType {
	if newWriteMode == m.updateMode {
		return newWriteMode
	}
	oldMode := m.updateMode
	var checkMode system.UpdateType
	// 1.过滤新的UpdateMode数据
	newWriteMode, checkMode = filterMode(newWriteMode, m.checkMode)
	m.updateMode = newWriteMode
	err := m.lsConfig.SetUpdateMode(m.updateMode)
	if err != nil {
		logger.Warning(err)
	}
	if m.updateModeChangedCallback != nil {
		m.updateModeChangedCallback(m.updateMode)
	}

	// 2.updateMode修改后，checkMode要随之修改
	for _, typ := range system.AllUpdateType() {
		oldBit := oldMode & typ
		newBit := newWriteMode & typ
		// updateMode清零的，应该在filter中已经清零了 TODO delete
		if oldBit == typ && newBit == 0 {
			// 该位清零,选中位也需要清零
			checkMode &= ^typ
		}

		if oldBit == 0 && newBit == typ {
			// 该位置一,选中为也需要置一
			checkMode |= typ
		}
	}
	m.SetCheckMode(checkMode)
	return m.updateMode
}

func (m *updateModeStatusManager) SetCheckMode(mode system.UpdateType) system.UpdateType {
	if mode == m.checkMode {
		return mode
	}
	_, m.checkMode = filterMode(m.updateMode, mode)
	err := m.lsConfig.SetCheckUpdateMode(m.checkMode)
	if err != nil {
		logger.Warning(err)
	}
	if m.checkModeChangedCallback != nil {
		m.checkModeChangedCallback(m.checkMode)
	}
	// check的内容修改后,canUpgrade的状态要随之修改
	m.UpdateCheckCanUpgradeByEachStatus()
	return m.checkMode
}

func (m *updateModeStatusManager) UpdateModeAllStatusBySize() {
	m.updateModeStatusBySize(system.AllUpdate)
}

// 单项计算
func (m *updateModeStatusManager) updateModeStatusBySize(mode system.UpdateType) {
	// 该处的处理,不会将更新项的状态修改为Upgraded.该状态只有可能在DistUpgrade中处理
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	var wg sync.WaitGroup
	for _, typ := range system.AllUpdateType() {
		if mode&typ == 0 {
			continue
		}
		currentMode := typ
		wg.Add(1)
		go func() {
			defer wg.Done()
			oldStatus := m.updateModeStatusObj[currentMode.JobType()]
			newStatus := oldStatus
			needDownloadSize, allPackageSize, err := system.QuerySourceDownloadSize(currentMode)
			if err != nil {
				logger.Warning(err)
			} else {
				m.updateModeDownloadSizeMap[currentMode.JobType()] = needDownloadSize
				logger.Infof("currentMode:%v,needDownloadSize:%v,allPackageSize:%v,oldStatus:%v.", currentMode, needDownloadSize, allPackageSize, oldStatus)
				// allPackageSize == 0 有两种情况：1.无需更新;2.更新完成需要重启;
				if allPackageSize == 0 {
					if oldStatus != system.Upgraded {
						newStatus = system.NoUpdate
					}
				} else {
					// allPackageSize > 0 需要更新
					// needDownloadSize == 0 可能有3种状态: 可更新,更新中,更新失败;
					if needDownloadSize == 0 {
						if oldStatus == system.NotDownload ||
							oldStatus == system.IsDownloading ||
							oldStatus == system.Upgraded ||
							oldStatus == system.NoUpdate {
							// 如果为未下载、下载中、更新完成、无更新内容状态,需要迁移到可更新状态
							newStatus = system.CanUpgrade
						}
					}
					// needDownloadSize > 0 可能有3种状态: 没下载,下载中;或者是安装更新完成后仓库又有推送
					if needDownloadSize > 0 {
						if oldStatus == system.CanUpgrade ||
							oldStatus == system.UpgradeErr ||
							oldStatus == system.Upgraded ||
							oldStatus == system.DownloadErr ||
							oldStatus == system.NoUpdate {
							// 如果状态为可更新、更新失败、更新完成、下载失败、无更新内容,需要迁移到未下载;更新中、下载中状态不变
							newStatus = system.NotDownload
						}
					}
				}
			}
			if newStatus != oldStatus {
				m.updateModeStatusObj[currentMode.JobType()] = newStatus
			}
		}()
	}
	wg.Wait()
	m.syncUpdateStatusNoLock()
	logger.Infof("status:%+v", m.updateModeStatusObj)
}

func (m *updateModeStatusManager) updateCanUpgradeStatus(can bool) {
	err := m.lsConfig.UpdateLastoreDaemonStatus(canUpgrade, can)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *updateModeStatusManager) UpdateCheckCanUpgradeByEachStatus() {
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	checkCanUpgrade := false
	checkMode := m.checkMode
	for _, typ := range system.AllUpdateType() {
		// 先检查该项是否选中,未选中则无需判断
		if checkMode&typ == 0 {
			continue
		}
		// 判断该项的状态是否为可更新
		status, ok := m.updateModeStatusObj[typ.JobType()]
		if !ok {
			// 默认当成未下载处理处理
			m.updateModeStatusObj[typ.JobType()] = system.NotDownload
			checkCanUpgrade = false
			break
		} else {
			// 可更新条件:至少存在一项为可更新,且其他项为更新完成或更新失败或无更新内容
			// 包含更新失败的情况下,如果进行模态更新,只更新可更新部分,不更新已经更新失败的部分
			if status == system.CanUpgrade {
				checkCanUpgrade = true
			} else if status != system.Upgraded && status != system.UpgradeErr && status != system.NoUpdate {
				checkCanUpgrade = false
				break
			}
		}
	}
	m.updateCanUpgradeStatus(checkCanUpgrade)
}

// GetCanPrepareDistUpgradeMode 根据check和status判断,排除不能下载的类型
func (m *updateModeStatusManager) GetCanPrepareDistUpgradeMode(origin system.UpdateType) system.UpdateType {
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	var canPrepareUpgradeMode system.UpdateType
	checkMode := m.checkMode
	for _, typ := range system.AllUpdateType() {
		if origin&typ == 0 {
			continue
		}
		// 先检查该项是否选中,未选中则无需判断
		if checkMode&typ == 0 {
			continue
		}
		// 判断该项的状态是否为可更新
		status, ok := m.updateModeStatusObj[typ.JobType()]
		if !ok {
			continue
		} else {
			// 可下载类型判断条件：该类型为未下载(如果不存在可下载的会在size或package查询中判断),或正在下载
			if status == system.NotDownload || status == system.IsDownloading || status == system.DownloadPause {
				canPrepareUpgradeMode |= typ
			}
		}
	}
	return canPrepareUpgradeMode
}

// GetCanDistUpgradeMode 根据check和status判断,排除不能更新的类型
func (m *updateModeStatusManager) GetCanDistUpgradeMode(origin system.UpdateType) system.UpdateType {
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	var canUpgradeMode system.UpdateType
	var upgradeFailedMode system.UpdateType
	checkMode := m.checkMode
	canUpgradeCount := 0
	upgradeFailedCount := 0
	for _, typ := range system.AllUpdateType() {
		if origin&typ == 0 {
			continue
		}
		// 先检查该项是否选中,未选中则无需判断
		if checkMode&typ == 0 {
			continue
		}
		// 判断该项的状态是否为可更新
		status, ok := m.updateModeStatusObj[typ.JobType()]
		if !ok {
			continue
		} else {
			// 可安装类型判断条件：该类型为可安装、正在安装或安装失败
			// 如果全都为更新失败,那么属于重试更新->可以更新
			// 如果可更新和更新失败都有,那么只能可更新项去更新
			if status == system.CanUpgrade || status == system.Upgrading {
				canUpgradeCount++
				canUpgradeMode |= typ
			}
			if status == system.UpgradeErr {
				upgradeFailedCount++
				upgradeFailedMode |= typ
			}
		}
	}
	if upgradeFailedCount > 0 {
		return upgradeFailedMode
	} else {
		return canUpgradeMode
	}
}

func (m *updateModeStatusManager) GetAllUpdateModeDownloadSize() map[string]float64 {
	return m.updateModeDownloadSizeMap
}
