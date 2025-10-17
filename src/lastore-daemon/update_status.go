// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
)

type UpdateModeStatusManager struct {
	checkMode                           system.UpdateType
	updateMode                          system.UpdateType
	lsConfig                            *config.Config
	systemUpdateStatus                  system.UpdateModeStatus
	securityUpdateStatus                system.UpdateModeStatus
	unKnownUpdateStatus                 system.UpdateModeStatus
	updateModeStatusObj                 map[string]system.UpdateModeStatus // 每一个更新项的状态 object,在检查更新、下载更新、安装更新的过程中修改
	updateModeDownloadSizeMap           map[string]float64
	updateModeDownloadSizeMapLock       sync.Mutex
	abStatus                            system.ABStatus
	abError                             system.ABErrorType
	currentTriggerBackingUpType         system.UpdateType
	backupFailedType                    system.UpdateType
	statusMapMu                         sync.RWMutex
	handleStatusChangedCallback         func(string)
	handleSystemStatusChangedCallback   func(interface{})
	handleSecurityStatusChangedCallback func(interface{})
	handleUnKnownStatusChangedCallback  func(interface{})
	checkModeChangedCallback            func(interface{})
	updateModeChangedCallback           func(interface{})

	updateSourceOnce bool // 是否完成过检查更新
}

type daemonStatus struct {
	ABStatus             system.ABStatus
	ABError              system.ABErrorType
	TriggerBackingUpType system.UpdateType
	BackupFailedType     system.UpdateType
	UpdateStatus         map[string]system.UpdateModeStatus
}

func NewStatusManager(config *config.Config, callback func(newStatus string)) *UpdateModeStatusManager {
	m := &UpdateModeStatusManager{
		lsConfig:                    config,
		checkMode:                   config.CheckUpdateMode,
		updateMode:                  config.UpdateMode,
		handleStatusChangedCallback: callback,
	}
	return m
}

func (m *UpdateModeStatusManager) InitModifyData() {
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
	obj := &daemonStatus{
		TriggerBackingUpType: system.AllInstallUpdate,
		ABStatus:             system.NotBackup,
		ABError:              system.NoABError,
		UpdateStatus:         make(map[string]system.UpdateModeStatus),
	}
	m.statusMapMu.Lock()
	err = json.Unmarshal([]byte(m.lsConfig.UpdateStatus), &obj)
	if err != nil {
		logger.Warning(err)
		m.updateModeStatusObj = make(map[string]system.UpdateModeStatus)
		for _, typ := range system.AllInstallUpdateType() {
			m.updateModeStatusObj[typ.JobType()] = system.NotDownload
		}
		m.currentTriggerBackingUpType = system.AllInstallUpdate
		m.abStatus = system.NotBackup
		m.abError = system.NoABError
		m.syncUpdateStatusNoLock()
	} else {
		m.updateModeStatusObj = obj.UpdateStatus
		m.currentTriggerBackingUpType = obj.TriggerBackingUpType
		m.abStatus = obj.ABStatus
		m.abError = obj.ABError
		if isFirstBoot() {
			for key, value := range m.updateModeStatusObj {
				switch value {
				case system.IsDownloading, system.DownloadPause, system.DownloadErr:
					m.updateModeStatusObj[key] = system.NotDownload
				case system.UpgradeErr, system.Upgrading, system.WaitRunUpgrade, system.CanUpgrade:
					m.updateModeStatusObj[key] = system.NotDownload
				case system.Upgraded:
					m.updateModeStatusObj[key] = system.NoUpdate
				}
			}
			m.currentTriggerBackingUpType = system.AllInstallUpdate
			m.abStatus = system.NotBackup
			m.abError = system.NoABError
			err := m.lsConfig.UpdateLastoreDaemonStatus(config.CanUpgrade, false)
			if err != nil {
				logger.Warning(err)
			}
		}
		m.syncUpdateStatusNoLock()
	}
	m.statusMapMu.Unlock()
	m.updateModeDownloadSizeMap = make(map[string]float64)
}

// filterMode 去除 updateMode 和 checkMode 不满足条件的数据
func filterMode(updateMode, checkMode system.UpdateType) (system.UpdateType, system.UpdateType) {
	var res0 system.UpdateType // updateMode
	var res1 system.UpdateType // checkMode
	// 过滤掉不存在的类型，updateMode没有的类型，checkMode的也需要清理
	for _, typ := range system.AllInstallUpdateType() {
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

func (m *UpdateModeStatusManager) RegisterChangedHandler(key string, handler func(value interface{})) {
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
func (m *UpdateModeStatusManager) getUpdateStatusString() string {
	m.statusMapMu.RLock()
	defer m.statusMapMu.RUnlock()
	content, err := json.Marshal(m.updateModeStatusObj)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return string(content)
}

func (m *UpdateModeStatusManager) GetUpdateStatus(typ system.UpdateType) system.UpdateModeStatus {
	m.statusMapMu.RLock()
	defer m.statusMapMu.RUnlock()
	return m.updateModeStatusObj[typ.JobType()]
}

func canTransition(oldStatus, newStatus system.UpdateModeStatus) bool {
	if newStatus == system.DownloadPause && oldStatus != system.IsDownloading {
		return false
	}
	if newStatus == system.IsDownloading && oldStatus == system.CanUpgrade {
		return false
	}
	if newStatus == system.NotDownload && oldStatus == system.CanUpgrade {
		return false
	}
	if newStatus == system.NotDownload && oldStatus == system.DownloadErr {
		return false
	}
	if newStatus == system.NotDownload && oldStatus == system.Upgraded {
		return false
	}
	if newStatus == system.IsDownloading && oldStatus == system.Upgraded {
		return false
	}
	if newStatus == system.NotDownload && oldStatus == system.WaitRunUpgrade {
		return false
	}
	if newStatus == system.IsDownloading && oldStatus == system.WaitRunUpgrade {
		return false
	}
	if newStatus == system.IsDownloading && oldStatus == system.UpgradeErr {
		return false
	}
	if newStatus == system.NotDownload && oldStatus == system.UpgradeErr {
		return false
	}
	if newStatus == system.NotDownload && oldStatus == system.Upgrading {
		return false
	}
	if newStatus == system.IsDownloading && oldStatus == system.Upgrading {
		return false
	}
	return true
}

// SetUpdateStatus 外部调用,会对设置的状态进行过滤
func (m *UpdateModeStatusManager) SetUpdateStatus(mode system.UpdateType, newStatus system.UpdateModeStatus) {
	changed := false
	m.statusMapMu.Lock()
	for _, typ := range system.AllInstallUpdateType() {
		if mode&typ != 0 && m.checkMode&typ != 0 {
			oldStatus := m.updateModeStatusObj[typ.JobType()]
			if !canTransition(oldStatus, newStatus) {
				logger.Infof("inhibit %v transition state from %v to %v", typ.JobType(), oldStatus, newStatus)
				continue
			}
			if oldStatus != newStatus {
				logger.Infof("%v transition state from %v to %v", typ.JobType(), oldStatus, newStatus)
				m.updateModeStatusObj[typ.JobType()] = newStatus
				changed = true
			}

		}
	}
	if !changed {
		m.statusMapMu.Unlock()
		return
	}
	m.syncUpdateStatusNoLock()
	m.statusMapMu.Unlock()
	m.UpdateCheckCanUpgradeByEachStatus()
}

// TransitionUpdateStatusValid 用于判断前后类型是否可以迁移
// 非下载中不能迁移到下载暂停(updateInfo重复触发暂停);
// 下载完成不能迁移到下载中(串联下载时用于规避);
func TransitionUpdateStatusValid(oldStatus, newStatus system.UpdateModeStatus) bool {
	// map的key为旧状态,value为不能迁移的新状态合集
	invalidationMap := map[system.UpdateModeStatus][]system.UpdateModeStatus{
		system.NoUpdate:       {system.DownloadPause},
		system.NotDownload:    {system.DownloadPause},
		system.IsDownloading:  {},
		system.DownloadPause:  {system.DownloadPause},
		system.DownloadErr:    {system.DownloadPause},
		system.CanUpgrade:     {system.IsDownloading},
		system.WaitRunUpgrade: {system.DownloadPause},
		system.Upgrading:      {system.DownloadPause},
		system.UpgradeErr:     {system.DownloadPause},
		system.Upgraded:       {system.DownloadPause},
	}
	tos, ok := invalidationMap[oldStatus]
	if !ok {
		return true
	}
	for _, v := range tos {
		if v == newStatus {
			return false
		}
	}
	return true
}

func (m *UpdateModeStatusManager) SetABStatus(typ system.UpdateType, status system.ABStatus, error system.ABErrorType) {
	if m.currentTriggerBackingUpType == typ && m.abStatus == status && m.abError == error {
		return
	}
	m.currentTriggerBackingUpType = typ
	switch status {
	case system.BackingUp:
		m.backupFailedType &= ^typ
	case system.BackupFailed:
		m.backupFailedType |= typ
	case system.NotBackup, system.HasBackedUp:
		m.backupFailedType = 0
	}
	m.abStatus = status
	m.abError = error
	m.syncUpdateStatusNoLock()
}

func (m *UpdateModeStatusManager) syncUpdateStatusNoLock() {
	obj := &daemonStatus{
		TriggerBackingUpType: m.currentTriggerBackingUpType,
		BackupFailedType:     m.backupFailedType,
		ABStatus:             m.abStatus,
		ABError:              m.abError,
		UpdateStatus:         m.updateModeStatusObj,
	}
	content, err := json.Marshal(obj)
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Infof("sync new status %v to config", string(content))
	if m.handleStatusChangedCallback != nil {
		m.handleStatusChangedCallback(string(content))
	}
	_ = m.lsConfig.SetUpdateStatus(string(content))
}

func (m *UpdateModeStatusManager) getUpdateMode() system.UpdateType {
	return m.updateMode
}

func (m *UpdateModeStatusManager) getCheckMode() system.UpdateType {
	return m.checkMode
}

func (m *UpdateModeStatusManager) SetUpdateMode(newWriteMode system.UpdateType) system.UpdateType {
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
	for _, typ := range system.AllInstallUpdateType() {
		oldBit := oldMode & typ
		newBit := newWriteMode & typ
		// updateMode清零的，应该在filter中已经清零了 TODO delete
		if oldBit == typ && newBit == 0 {
			// 该位清零,选中位也需要清零
			checkMode &= ^typ
		}

		if oldBit == 0 && newBit == typ {
			// 该位置一,选中位也需要置一
			checkMode |= typ
		}
	}
	m.SetCheckMode(checkMode)
	return m.updateMode
}

func (m *UpdateModeStatusManager) SetCheckMode(mode system.UpdateType) system.UpdateType {
	if mode == m.checkMode {
		return mode
	}
	_, checkMode := filterMode(m.updateMode, mode)
	err := m.lsConfig.SetCheckUpdateMode(checkMode)
	if err != nil {
		logger.Warning(err)
	}
	if m.checkModeChangedCallback != nil {
		m.checkModeChangedCallback(checkMode)
	}
	m.checkMode = checkMode
	// check的内容修改后,canUpgrade的状态要随之修改
	m.UpdateCheckCanUpgradeByEachStatus()
	return checkMode
}

// UpdateModeAllStatusBySize 根据size计算更新所有状态,会把除了安装失败之外的所有错误去除
func (m *UpdateModeStatusManager) UpdateModeAllStatusBySize(coreList []string) {
	m.updateModeStatusBySize(system.AllInstallUpdate, coreList)
}

// 单项计算
func (m *UpdateModeStatusManager) updateModeStatusBySize(mode system.UpdateType, coreList []string) {
	// 该处的处理,不会将更新项的状态修改为Upgraded.该状态只有可能在DistUpgrade中处理
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	var wg sync.WaitGroup
	changed := false
	for _, typ := range system.AllInstallUpdateType() {
		if mode&typ == 0 {
			continue
		}
		currentMode := typ
		wg.Add(1)
		go func() {
			defer wg.Done()
			oldStatus := m.updateModeStatusObj[currentMode.JobType()]
			newStatus := oldStatus
			needDownloadSize, allPackageSize, err := system.QuerySourceDownloadSize(currentMode, coreList)
			if err != nil {
				logger.Warning(err)
				// 初始化配置值为noDownload，如果query失败，不会变更，造成前端状态异常
				// 升级问题处理

				// 如果有job正在安装，而且出现了half-installed导致的报错，走重试继续
				if strings.Contains(err.Error(), "needs to be reinstalled, but I can't find an archive for it.") {
					for _, modeStat := range m.updateModeStatusObj {
						if modeStat == system.Upgrading {
							logger.Warning("package half-installed, need retry!")
							return
						}
					}
				}
				if oldStatus != system.NoUpdate {
					m.updateModeStatusObj[currentMode.JobType()] = system.NoUpdate
					changed = true
				}
			} else {
				if m.lsConfig.IncrementalUpdate && needDownloadSize > 0 && apt.IsIncrementalUpdateCached() {
					needDownloadSize = 0.0
				}

				m.updateModeDownloadSizeMapLock.Lock()
				m.updateModeDownloadSizeMap[currentMode.JobType()] = needDownloadSize
				m.updateModeDownloadSizeMapLock.Unlock()
				logger.Infof("currentMode:%v,needDownloadSize:%v,allPackageSize:%v,oldStatus:%v.", currentMode.JobType(), needDownloadSize, allPackageSize, oldStatus)
				// allPackageSize == 0 有两种情况：1.无需更新;2.更新完成需要重启;
				if allPackageSize == 0 {
					if oldStatus != system.Upgraded {
						newStatus = system.NoUpdate
					}
				} else {
					// allPackageSize > 0 需要更新
					// needDownloadSize == 0 可能有3种状态: 下载完成可更新,更新中,更新失败;
					// needDownloadSize > 0 可能有4种状态: 没下载,下载中,下载暂停,下载失败或者是安装更新完成后仓库又有推送
					switch oldStatus {
					case system.NoUpdate:
						if needDownloadSize == 0 {
							newStatus = system.CanUpgrade
						} else {
							newStatus = system.NotDownload
						}
					case system.NotDownload:
						if needDownloadSize == 0 {
							newStatus = system.CanUpgrade
						} else {
							newStatus = system.NotDownload
							// 无需处理
						}
					case system.IsDownloading:
						if needDownloadSize == 0 {
							newStatus = system.CanUpgrade
						} else {
							// 无需处理
							newStatus = system.IsDownloading
						}
					case system.DownloadPause:
						if needDownloadSize == 0 {
							newStatus = system.CanUpgrade
						} else {
							// 无需处理
							newStatus = system.DownloadPause
						}
					case system.DownloadErr:
						if needDownloadSize == 0 {
							newStatus = system.CanUpgrade
						} else {
							// 当成未下载处理
							newStatus = system.NotDownload
						}
					case system.CanUpgrade:
						if needDownloadSize == 0 {
							// 无需处理
							newStatus = system.CanUpgrade
						} else {
							newStatus = system.NotDownload
						}
					case system.WaitRunUpgrade:
						// 无法根据size判断该状态是否需要修改,不处理
						newStatus = system.WaitRunUpgrade
					case system.Upgrading:
						if needDownloadSize == 0 {
							// 无需处理
							newStatus = system.Upgrading
						} else {
							// 无需处理
							newStatus = system.Upgrading
						}
					case system.UpgradeErr:
						if needDownloadSize == 0 {
							// 无需处理
							newStatus = system.UpgradeErr
						} else {
							newStatus = system.NotDownload
						}
					case system.Upgraded:
						if needDownloadSize == 0 {
							// 无需处理
							newStatus = system.Upgraded
						} else {
							newStatus = system.NotDownload
						}
					}
				}
			}
			if newStatus != oldStatus {
				m.updateModeStatusObj[currentMode.JobType()] = newStatus
				changed = true
			}
		}()
	}
	wg.Wait()
	if changed {
		m.syncUpdateStatusNoLock()
	}
	logger.Infof("status:%+v", m.updateModeStatusObj)
}

func (m *UpdateModeStatusManager) updateCanUpgradeStatus(can bool) {
	oldCanUpgrade := m.lsConfig.GetLastoreDaemonStatusByBit(config.CanUpgrade) == config.CanUpgrade
	if oldCanUpgrade == can {
		return
	}
	logger.Infof("CanUpgradeStatus transition state from %v to %v", oldCanUpgrade, can)
	err := m.lsConfig.UpdateLastoreDaemonStatus(config.CanUpgrade, can)
	if err != nil {
		logger.Warning(err)
	}
}

// UpdateCheckCanUpgradeByEachStatus 必须在检查更新完成之后，才可以更新
func (m *UpdateModeStatusManager) UpdateCheckCanUpgradeByEachStatus() {
	if !m.updateSourceOnce {
		logger.Warning("not update source, don't need to update can-upgrade")
		return
	}
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	checkCanUpgrade := false
	checkMode := m.checkMode
	for _, typ := range system.AllInstallUpdateType() {
		// 先检查该项是否选中,未选中则无需判断
		if checkMode&typ == 0 {
			continue
		}
		// 判断该项的状态是否为可更新
		status, ok := m.updateModeStatusObj[typ.JobType()]
		if !ok {
			// 默认当成未下载处理处理
			m.updateModeStatusObj[typ.JobType()] = system.NotDownload
		} else {
			// 可更新条件:至少存在一项为可更新或更新失败
			if status == system.CanUpgrade || status == system.UpgradeErr {
				checkCanUpgrade = true
				break
			}
		}
	}
	m.updateCanUpgradeStatus(checkCanUpgrade)
}

// GetCanPrepareDistUpgradeMode 根据check和status判断,排除不能下载的类型
func (m *UpdateModeStatusManager) GetCanPrepareDistUpgradeMode(origin system.UpdateType) system.UpdateType {
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	var canPrepareUpgradeMode system.UpdateType
	checkMode := m.checkMode
	for _, typ := range system.AllInstallUpdateType() {
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
			// 可下载类型判断条件：该类型为未下载(如果不存在可下载的会在size或package查询中判断),或正在下载,下载失败
			if status == system.NotDownload || status == system.IsDownloading || status == system.DownloadPause || status == system.DownloadErr || status == system.CanUpgrade {
				canPrepareUpgradeMode |= typ
			}
		}
	}
	return canPrepareUpgradeMode
}

// GetCanDistUpgradeMode 根据check和status判断,排除不能更新的类型
func (m *UpdateModeStatusManager) GetCanDistUpgradeMode(origin system.UpdateType) system.UpdateType {
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	var canUpgradeMode system.UpdateType
	var upgradeFailedMode system.UpdateType
	checkMode := m.checkMode
	canUpgradeCount := 0
	for _, typ := range system.AllInstallUpdateType() {
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
			// 可更新+更新失败的情况下,如果进行更新,只更新可更新部分,不更新已经更新失败的部分
			// 可更新+未下载的情况下,如果进行更新,只更新可更新部分,不更新未下载部分、
			// 只有更新失败,属于重试更新,对更新失败的进行更新
			if status == system.CanUpgrade || status == system.Upgrading || status == system.WaitRunUpgrade {
				canUpgradeCount++
				canUpgradeMode |= typ
			}
			if status == system.UpgradeErr {
				upgradeFailedMode |= typ
			}
		}
	}
	if canUpgradeCount == 0 {
		return upgradeFailedMode
	} else {
		return canUpgradeMode
	}
}

func (m *UpdateModeStatusManager) GetAllUpdateModeDownloadSize() map[string]float64 {
	return m.updateModeDownloadSizeMap
}

// SetFrontForceUpdate 前端(dde-lock dde-dock)强制更新:隐藏关机和重启选项
func (m *UpdateModeStatusManager) SetFrontForceUpdate(force bool) {
	oldForceUpdate := m.lsConfig.GetLastoreDaemonStatusByBit(config.ForceUpdate) == config.ForceUpdate
	if oldForceUpdate == force {
		return
	}
	logger.Infof("ForceUpdateStatus transition state from %v to %v", oldForceUpdate, force)
	err := m.lsConfig.UpdateLastoreDaemonStatus(config.ForceUpdate, force)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *UpdateModeStatusManager) isUpgrading() bool {
	m.statusMapMu.Lock()
	defer m.statusMapMu.Unlock()
	for _, v := range m.updateModeStatusObj {
		if v == system.WaitRunUpgrade || v == system.Upgrading {
			return true
		}
	}
	return false
}
