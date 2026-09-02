// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

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
			system.SystemUpdate.JobType():  system.NotDownload,
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
			system.SystemUpdate.JobType():  system.NotDownload,
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
