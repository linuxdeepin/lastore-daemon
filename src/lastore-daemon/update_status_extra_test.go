// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		old, new_ system.UpdateModeStatus
		want      bool
	}{
		{system.IsDownloading, system.DownloadPause, true},
		{system.NoUpdate, system.DownloadPause, false},
		{system.CanUpgrade, system.IsDownloading, false},
		{system.CanUpgrade, system.NotDownload, false},
		{system.DownloadErr, system.NotDownload, false},
		{system.Upgraded, system.NotDownload, false},
		{system.Upgraded, system.IsDownloading, false},
		{system.WaitRunUpgrade, system.NotDownload, false},
		{system.WaitRunUpgrade, system.IsDownloading, false},
		{system.UpgradeErr, system.IsDownloading, false},
		{system.UpgradeErr, system.NotDownload, false},
		{system.Upgrading, system.NotDownload, false},
		{system.Upgrading, system.IsDownloading, false},
		{system.NotDownload, system.IsDownloading, true},
		{system.NoUpdate, system.NotDownload, true},
	}
	for _, tt := range tests {
		got := canTransition(tt.old, tt.new_)
		assert.Equal(t, tt.want, got, "canTransition(%v, %v)", tt.old, tt.new_)
	}
}

func TestTransitionUpdateStatusValid(t *testing.T) {
	tests := []struct {
		old, new_ system.UpdateModeStatus
		want      bool
	}{
		// DownloadPause is only valid from IsDownloading
		{system.IsDownloading, system.DownloadPause, true},
		{system.NoUpdate, system.DownloadPause, false},
		{system.NotDownload, system.DownloadPause, false},
		{system.DownloadPause, system.DownloadPause, false},
		{system.DownloadErr, system.DownloadPause, false},
		{system.CanUpgrade, system.DownloadPause, true},
		{system.WaitRunUpgrade, system.DownloadPause, false},
		{system.Upgrading, system.DownloadPause, false},
		{system.UpgradeErr, system.DownloadPause, false},
		{system.Upgraded, system.DownloadPause, false},
		// CanUpgrade cannot transition to IsDownloading
		{system.CanUpgrade, system.IsDownloading, false},
		// Valid transitions
		{system.NoUpdate, system.NotDownload, true},
		{system.NotDownload, system.IsDownloading, true},
		{system.IsDownloading, system.CanUpgrade, true},
		{system.CanUpgrade, system.WaitRunUpgrade, true},
		{system.Upgraded, system.NoUpdate, true},
	}
	for _, tt := range tests {
		got := TransitionUpdateStatusValid(tt.old, tt.new_)
		assert.Equal(t, tt.want, got, "TransitionUpdateStatusValid(%v, %v)", tt.old, tt.new_)
	}
}

func TestFilterMode(t *testing.T) {
	updateMode := system.SystemUpdate | system.SecurityUpdate
	checkMode := system.SystemUpdate
	res0, res1 := filterMode(updateMode, checkMode)
	assert.True(t, res0&system.SystemUpdate != 0)
	assert.True(t, res1&system.SystemUpdate != 0)
}

func TestFilterModeWithOnlySecurity(t *testing.T) {
	updateMode := system.OnlySecurityUpdate
	checkMode := system.OnlySecurityUpdate
	res0, res1 := filterMode(updateMode, checkMode)
	assert.True(t, res0&system.SecurityUpdate != 0)
	assert.True(t, res1&system.SecurityUpdate != 0)
}

func TestFilterModeEmptyCheckMode(t *testing.T) {
	updateMode := system.SystemUpdate
	checkMode := system.UpdateType(0)
	res0, res1 := filterMode(updateMode, checkMode)
	assert.True(t, res0&system.SystemUpdate != 0)
	assert.Equal(t, system.UpdateType(0), res1)
}

func TestGetUpdateStatusStringExtra(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType():   system.CanUpgrade,
			system.SecurityUpdate.JobType(): system.NotDownload,
		},
	}
	s := m.getUpdateStatusString()
	assert.Contains(t, s, system.SystemUpdate.JobType())
	assert.Contains(t, s, system.SecurityUpdate.JobType())
}

func TestGetUpdateStatusStringEmpty(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{},
	}
	s := m.getUpdateStatusString()
	assert.Equal(t, "{}", s)
}

func TestGetUpdateStatusExtra(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.CanUpgrade,
		},
	}
	assert.Equal(t, system.CanUpgrade, m.GetUpdateStatus(system.SystemUpdate))
	assert.Equal(t, system.UpdateModeStatus(""), m.GetUpdateStatus(system.SecurityUpdate))
}

func TestGetUpdateStatusMissing(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{},
	}
	s := m.GetUpdateStatus(system.SystemUpdate)
	assert.Equal(t, system.UpdateModeStatus(""), s)
}

func TestIsUpgradingFalse(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.CanUpgrade,
		},
	}
	assert.False(t, m.isUpgrading())
}

func TestIsUpgradingTrue(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.Upgrading,
		},
	}
	assert.True(t, m.isUpgrading())
}

func TestIsUpgradingWaitRun(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SecurityUpdate.JobType(): system.WaitRunUpgrade,
		},
	}
	assert.True(t, m.isUpgrading())
}

func TestGetAllUpdateModeDownloadSizeExtra(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeDownloadSizeMap: map[string]float64{
			"system": 1024.0,
		},
	}
	result := m.GetAllUpdateModeDownloadSize()
	assert.Equal(t, 1024.0, result["system"])
}

func TestGetCanPrepareDistUpgradeMode(t *testing.T) {
	m := &UpdateModeStatusManager{
		checkMode: system.SystemUpdate | system.SecurityUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType():   system.NotDownload,
			system.SecurityUpdate.JobType(): system.CanUpgrade,
		},
	}
	result := m.GetCanPrepareDistUpgradeMode(system.SystemUpdate | system.SecurityUpdate)
	assert.True(t, result&system.SystemUpdate != 0)
	assert.True(t, result&system.SecurityUpdate != 0)
}

func TestGetCanPrepareDistUpgradeModeNone(t *testing.T) {
	m := &UpdateModeStatusManager{
		checkMode: system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.Upgraded,
		},
	}
	result := m.GetCanPrepareDistUpgradeMode(system.SystemUpdate)
	assert.Equal(t, system.UpdateType(0), result)
}

func TestGetCanDistUpgradeMode(t *testing.T) {
	m := &UpdateModeStatusManager{
		checkMode: system.SystemUpdate | system.SecurityUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType():   system.CanUpgrade,
			system.SecurityUpdate.JobType(): system.UpgradeErr,
		},
	}
	result := m.GetCanDistUpgradeMode(system.SystemUpdate | system.SecurityUpdate)
	assert.True(t, result&system.SystemUpdate != 0)
}

func TestGetCanDistUpgradeModeOnlyFailed(t *testing.T) {
	m := &UpdateModeStatusManager{
		checkMode: system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.UpgradeErr,
		},
	}
	result := m.GetCanDistUpgradeMode(system.SystemUpdate)
	assert.True(t, result&system.SystemUpdate != 0)
}

func TestGetCanDistUpgradeModeEmpty(t *testing.T) {
	m := &UpdateModeStatusManager{
		checkMode: system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	result := m.GetCanDistUpgradeMode(system.SystemUpdate)
	assert.Equal(t, system.UpdateType(0), result)
}

func TestNewStatusManager(t *testing.T) {
	cfg := config.NewConfig("")
	cfg.UpdateMode = system.SystemUpdate | system.SecurityUpdate
	cfg.CheckUpdateMode = system.SystemUpdate
	m := NewStatusManager(cfg, func(s string) {})
	assert.NotNil(t, m)
	assert.Equal(t, system.SystemUpdate, m.checkMode)
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, m.updateMode)
	assert.NotNil(t, m.handleStatusChangedCallback)
}

func TestInitModifyData(t *testing.T) {
	cfg := config.NewConfig("")
	_ = cfg.SetUpdateStatus("")
	cfg.UpdateMode = system.SystemUpdate | system.SecurityUpdate
	cfg.CheckUpdateMode = system.SystemUpdate
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: cfg.UpdateMode,
		checkMode:  cfg.CheckUpdateMode,
	}
	m.InitModifyData()
	assert.NotNil(t, m.updateModeStatusObj)
	for _, typ := range system.AllInstallUpdateType() {
		_, ok := m.updateModeStatusObj[typ.JobType()]
		assert.True(t, ok, "expected key %s in updateModeStatusObj", typ.JobType())
	}
	assert.NotNil(t, m.updateModeDownloadSizeMap)
}

func TestSetUpdateStatusTransition(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	m.SetUpdateStatus(system.SystemUpdate, system.IsDownloading)
	assert.Equal(t, system.IsDownloading, m.GetUpdateStatus(system.SystemUpdate))
}

func TestSetUpdateStatusInhibitedTransition(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.CanUpgrade,
		},
	}
	m.SetUpdateStatus(system.SystemUpdate, system.IsDownloading)
	assert.Equal(t, system.CanUpgrade, m.GetUpdateStatus(system.SystemUpdate))
}

func TestSetUpdateStatusNoChange(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	m.SetUpdateStatus(system.SystemUpdate, system.NotDownload)
	assert.Equal(t, system.NotDownload, m.GetUpdateStatus(system.SystemUpdate))
}

func TestSetUpdateStatusUncheckMode(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate | system.SecurityUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType():   system.NotDownload,
			system.SecurityUpdate.JobType(): system.NotDownload,
		},
	}
	m.SetUpdateStatus(system.SecurityUpdate, system.IsDownloading)
	assert.Equal(t, system.NotDownload, m.GetUpdateStatus(system.SecurityUpdate))
}

func TestSetABStatusBackingUp(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	m.SetABStatus(system.SystemUpdate, system.BackingUp, system.NoABError)
	assert.Equal(t, system.BackingUp, m.abStatus)
	assert.Equal(t, system.SystemUpdate, m.currentTriggerBackingUpType)
	assert.Equal(t, system.UpdateType(0), m.backupFailedType)
}

func TestSetABStatusBackupFailed(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate | system.SecurityUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	m.SetABStatus(system.SystemUpdate, system.BackupFailed, system.CanNotBackup)
	assert.Equal(t, system.BackupFailed, m.abStatus)
	assert.Equal(t, system.CanNotBackup, m.abError)
	assert.Equal(t, system.SystemUpdate, m.backupFailedType)
}

func TestSetABStatusNotBackupClearsFailed(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
		backupFailedType: system.SystemUpdate,
	}
	m.SetABStatus(system.SystemUpdate, system.NotBackup, system.NoABError)
	assert.Equal(t, system.NotBackup, m.abStatus)
	assert.Equal(t, system.UpdateType(0), m.backupFailedType)
}

func TestSetABStatusNoChange(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
		currentTriggerBackingUpType: system.SystemUpdate,
		abStatus:                    system.BackingUp,
		abError:                     system.NoABError,
	}
	m.SetABStatus(system.SystemUpdate, system.BackingUp, system.NoABError)
	assert.Equal(t, system.BackingUp, m.abStatus)
}

func TestSyncUpdateStatusNoLock(t *testing.T) {
	cfg := config.NewConfig("")
	var captured string
	m := &UpdateModeStatusManager{
		lsConfig: cfg,
		handleStatusChangedCallback: func(s string) {
			captured = s
		},
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
		currentTriggerBackingUpType: system.SystemUpdate,
		abStatus:                    system.NotBackup,
		abError:                     system.NoABError,
	}
	m.syncUpdateStatusNoLock()
	assert.NotEmpty(t, captured)
	assert.Contains(t, captured, "notBackup")
}

func TestGetUpdateMode(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateMode: system.SystemUpdate | system.SecurityUpdate,
	}
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, m.getUpdateMode())
}

func TestGetCheckMode(t *testing.T) {
	m := &UpdateModeStatusManager{
		checkMode: system.SecurityUpdate,
	}
	assert.Equal(t, system.SecurityUpdate, m.getCheckMode())
}

func TestSetUpdateModeNew(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	result := m.SetUpdateMode(system.SystemUpdate | system.SecurityUpdate)
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, result)
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, m.updateMode)
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, m.checkMode)
}

func TestSetUpdateModeSame(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	result := m.SetUpdateMode(system.SystemUpdate)
	assert.Equal(t, system.SystemUpdate, result)
}

func TestSetCheckModeNew(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate | system.SecurityUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType():   system.NotDownload,
			system.SecurityUpdate.JobType(): system.NotDownload,
		},
	}
	result := m.SetCheckMode(system.SystemUpdate | system.SecurityUpdate)
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, result)
	assert.Equal(t, system.SystemUpdate|system.SecurityUpdate, m.checkMode)
}

func TestSetCheckModeSame(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.NotDownload,
		},
	}
	result := m.SetCheckMode(system.SystemUpdate)
	assert.Equal(t, system.SystemUpdate, result)
}

func TestSetFrontForceUpdate(t *testing.T) {
	cfg := config.NewConfig("")
	_ = cfg.SetLastoreDaemonStatus(0)
	m := &UpdateModeStatusManager{
		lsConfig: cfg,
	}
	m.SetFrontForceUpdate(true)
	m.SetFrontForceUpdate(true)
	m.SetFrontForceUpdate(false)
}

func TestRegisterChangedHandlerCheckMode(t *testing.T) {
	m := &UpdateModeStatusManager{}
	called := false
	m.RegisterChangedHandler(handlerKeyCheckMode, func(v interface{}) {
		called = true
	})
	assert.NotNil(t, m.checkModeChangedCallback)
	m.checkModeChangedCallback(system.SystemUpdate)
	assert.True(t, called)
}

func TestRegisterChangedHandlerUpdateMode(t *testing.T) {
	m := &UpdateModeStatusManager{}
	called := false
	m.RegisterChangedHandler(handlerKeyUpdateMode, func(v interface{}) {
		called = true
	})
	assert.NotNil(t, m.updateModeChangedCallback)
	m.updateModeChangedCallback(system.SystemUpdate)
	assert.True(t, called)
}

func TestRegisterChangedHandlerSystemStatus(t *testing.T) {
	m := &UpdateModeStatusManager{}
	called := false
	m.RegisterChangedHandler(handlerKeySystemStatus, func(v interface{}) {
		called = true
	})
	assert.NotNil(t, m.handleSystemStatusChangedCallback)
	m.handleSystemStatusChangedCallback("test")
	assert.True(t, called)
}

func TestRegisterChangedHandlerSecurityStatus(t *testing.T) {
	m := &UpdateModeStatusManager{}
	called := false
	m.RegisterChangedHandler(handlerKeySecurityStatus, func(v interface{}) {
		called = true
	})
	assert.NotNil(t, m.handleSecurityStatusChangedCallback)
	m.handleSecurityStatusChangedCallback("test")
	assert.True(t, called)
}

func TestRegisterChangedHandlerUnKnownStatus(t *testing.T) {
	m := &UpdateModeStatusManager{}
	called := false
	m.RegisterChangedHandler(handlerKeyUnKnownStatus, func(v interface{}) {
		called = true
	})
	assert.NotNil(t, m.handleUnKnownStatusChangedCallback)
	m.handleUnKnownStatusChangedCallback("test")
	assert.True(t, called)
}

func TestRegisterChangedHandlerInvalidKey(t *testing.T) {
	m := &UpdateModeStatusManager{}
	m.RegisterChangedHandler("invalidKey", func(v interface{}) {})
	assert.Nil(t, m.checkModeChangedCallback)
	assert.Nil(t, m.updateModeChangedCallback)
}

func TestUpdateCheckCanUpgradeByEachStatusNotReady(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:            cfg,
		updateMode:          system.SystemUpdate,
		checkMode:           system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{},
		updateSourceOnce:    false,
	}
	m.UpdateCheckCanUpgradeByEachStatus()
	assert.Equal(t, config.LastoreDaemonStatus(0), cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCheckCanUpgradeByEachStatusReady(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.CanUpgrade,
		},
		updateSourceOnce: true,
	}
	m.UpdateCheckCanUpgradeByEachStatus()
	assert.Equal(t, config.CanUpgrade, cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCheckCanUpgradeByEachStatusNotChecked(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate | system.SecurityUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SecurityUpdate.JobType(): system.CanUpgrade,
		},
		updateSourceOnce: true,
	}
	m.UpdateCheckCanUpgradeByEachStatus()
	assert.Equal(t, config.LastoreDaemonStatus(0), cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCheckCanUpgradeByEachStatusUpgradeErr(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:   cfg,
		updateMode: system.SystemUpdate,
		checkMode:  system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType(): system.UpgradeErr,
		},
		updateSourceOnce: true,
	}
	m.UpdateCheckCanUpgradeByEachStatus()
	assert.Equal(t, config.CanUpgrade, cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCheckCanUpgradeByEachStatusMissingKey(t *testing.T) {
	cfg := config.NewConfig("")
	m := &UpdateModeStatusManager{
		lsConfig:            cfg,
		updateMode:          system.SystemUpdate,
		checkMode:           system.SystemUpdate,
		updateModeStatusObj: map[string]system.UpdateModeStatus{},
		updateSourceOnce:    true,
	}
	m.UpdateCheckCanUpgradeByEachStatus()
	assert.Equal(t, system.NotDownload, m.GetUpdateStatus(system.SystemUpdate))
	assert.Equal(t, config.LastoreDaemonStatus(0), cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCanUpgradeStatus_Set(t *testing.T) {
	cfg := config.NewConfig("")
	_ = cfg.SetLastoreDaemonStatus(0)
	m := &UpdateModeStatusManager{lsConfig: cfg}

	m.updateCanUpgradeStatus(true)
	assert.Equal(t, config.CanUpgrade, cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCanUpgradeStatus_Clear(t *testing.T) {
	cfg := config.NewConfig("")
	_ = cfg.SetLastoreDaemonStatus(config.CanUpgrade)
	m := &UpdateModeStatusManager{lsConfig: cfg}

	m.updateCanUpgradeStatus(false)
	assert.Equal(t, config.LastoreDaemonStatus(0), cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}

func TestUpdateCanUpgradeStatus_Noop(t *testing.T) {
	cfg := config.NewConfig("")
	_ = cfg.SetLastoreDaemonStatus(0)
	m := &UpdateModeStatusManager{lsConfig: cfg}

	// already false -> no-op
	m.updateCanUpgradeStatus(false)
	assert.Equal(t, config.LastoreDaemonStatus(0), cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))

	// set then already true -> no-op
	m.updateCanUpgradeStatus(true)
	m.updateCanUpgradeStatus(true)
	assert.Equal(t, config.CanUpgrade, cfg.GetLastoreDaemonStatusByBit(config.CanUpgrade))
}
