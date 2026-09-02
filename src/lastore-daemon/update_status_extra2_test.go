// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestGetUpdateStatusStringExtra(t *testing.T) {
	m := &UpdateModeStatusManager{
		updateModeStatusObj: map[string]system.UpdateModeStatus{
			system.SystemUpdate.JobType():  system.CanUpgrade,
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
			system.SecurityUpdate.JobType():  system.CanUpgrade,
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
			system.SecurityUpdate.JobType():  system.UpgradeErr,
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
