// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	power "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFullManager builds a Manager with every dependency a hook path may touch,
// so downstream goroutines never deref a nil field.
func newFullManager(t *testing.T) *Manager {
	t.Helper()
	cfg := newTestConfig(t)
	service := newTestService()
	statusManager := NewStatusManager(cfg, nil)
	statusManager.InitModifyData()
	m := &Manager{
		service:        service,
		config:         cfg,
		updatePlatform: updateplatform.NewUpdatePlatformManager(cfg, false),
		statusManager:  statusManager,
		jobManager:     newTestJobManager(),
		userAgents:     newUserAgentMap(),
	}
	m.updater = &Updater{service: service, config: cfg, manager: m}
	return m
}

func TestDistUpgradePartlyFailingConn(t *testing.T) {
	m := &Manager{
		service:        newFailingConnService(t),
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	_, err := m.distUpgradePartly(ifcTestSender, system.SystemUpdate, false)
	assert.Error(t, err)
}

func TestDistUpgradeFailingConn(t *testing.T) {
	m := &Manager{
		service:          newFailingConnService(t),
		userAgents:       newUserAgentMap(),
		updateSourceOnce: true,
	}
	_, err := m.distUpgrade(ifcTestSender, system.SystemUpdate, false, false, false)
	assert.Error(t, err)
}

func TestDistUpgradePartlyNoDistUpgradeMode(t *testing.T) {
	// Fake bus reports the caller as uid 0, so GetConnUID succeeds without a
	// real bus or polkit; the empty status manager then yields mode 0 and the
	// method bails out before starting any upgrade.
	m := &Manager{
		service:        newRootConnService(t),
		updatePlatform: &updateplatform.UpdatePlatformManager{},
		statusManager:  newTestStatusManager(t),
	}
	_, busErr := m.distUpgradePartly(ifcTestSender, system.SystemUpdate, false)
	assert.NotNil(t, busErr)
}

func TestHandleSysPowerChanged(t *testing.T) {
	m := &Manager{sysPower: power.NewPower(newFailingConnService(t).Conn())}
	m.handleSysPowerChanged()
}

func TestPreRunningHook(t *testing.T) {
	m := &Manager{
		config:        newTestConfig(t),
		statusManager: newTestStatusManager(t),
	}
	m.preRunningHook(false, system.SecurityUpdate)
}

func TestPreFailedHook(t *testing.T) {
	m := &Manager{
		config:         newTestConfig(t),
		statusManager:  newTestStatusManager(t),
		updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false),
		userAgents:     newUserAgentMap(),
	}
	assert.NoError(t, m.preFailedHook(&Job{}, system.SecurityUpdate, "uuid"))
}

func TestPreUpgradeCmdSuccessHook(t *testing.T) {
	m := &Manager{
		config:         newTestConfig(t),
		statusManager:  newTestStatusManager(t),
		updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false),
		systemd:        newFailingSystemdManager(t),
	}
	job := NewJob(newTestService(), "id", "name", nil, system.UpdateJobType, LockQueue, nil)
	assert.NoError(t, m.preUpgradeCmdSuccessHook(job, system.SecurityUpdate, "uuid", false))
}

func TestAfterUpgradeCmdSuccessHook(t *testing.T) {
	m := &Manager{config: newTestConfig(t), updater: &Updater{}}
	assert.NoError(t, m.afterUpgradeCmdSuccessHook())
}

func TestHandleAfterUpgradeSuccess(t *testing.T) {
	m := &Manager{
		updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false),
		userAgents:     newUserAgentMap(),
	}
	m.handleAfterUpgradeSuccess(system.SecurityUpdate, "des", "uuid")
}

func TestCancelAllUpdateJob(t *testing.T) {
	m := &Manager{jobManager: newTestJobManager()}
	assert.NoError(t, m.cancelAllUpdateJob())
}

func TestPrepareAptCheckInvalidMode(t *testing.T) {
	_, err := (&Manager{}).prepareAptCheck(system.AppStoreUpdate)
	assert.Error(t, err)
}

func TestPreRunningHookSystemUpdate(t *testing.T) {
	m := &Manager{
		config:         newTestConfig(t),
		statusManager:  newTestStatusManager(t),
		updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false),
	}
	m.preRunningHook(false, system.SystemUpdate)
}

func TestPreFailedHookInsufficientSpace(t *testing.T) {
	m := newFullManager(t)
	desc, _ := json.Marshal(system.JobError{ErrType: system.ErrorInsufficientSpace})
	assert.NoError(t, m.preFailedHook(&Job{Description: string(desc)}, system.SystemUpdate, "uuid"))
}

func TestPreFailedHookOtherError(t *testing.T) {
	m := newFullManager(t)
	desc, _ := json.Marshal(system.JobError{ErrType: system.ErrorUnknown})
	assert.NoError(t, m.preFailedHook(&Job{Description: string(desc)}, system.SystemUpdate, "uuid"))
}

func TestPreFailedHookCheckError(t *testing.T) {
	m := newFullManager(t)
	desc, _ := json.Marshal(system.JobError{ErrType: system.ErrorUnknown, IsCheckError: true})
	assert.NoError(t, m.preFailedHook(&Job{Description: string(desc)}, system.SystemUpdate, "uuid"))
}

func TestPreFailedHookDamagePackage(t *testing.T) {
	orig := aptGetBin
	aptGetBin = "/bin/true"
	t.Cleanup(func() { aptGetBin = orig })
	m := newFullManager(t)
	desc, _ := json.Marshal(system.JobError{ErrType: system.ErrorDamagePackage})
	assert.NoError(t, m.preFailedHook(&Job{Description: string(desc)}, system.SystemUpdate, "uuid"))
}

func TestPreUpgradeCmdSuccessHookDisabledRebootCheck(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.PlatformDisabled = config.DisabledRebootCheck
	m := &Manager{
		config:         cfg,
		statusManager:  newTestStatusManager(t),
		updatePlatform: updateplatform.NewUpdatePlatformManager(cfg, false),
		userAgents:     newUserAgentMap(),
	}
	job := NewJob(newTestService(), "id", "name", nil, system.UpdateJobType, LockQueue, nil)
	assert.NoError(t, m.preUpgradeCmdSuccessHook(job, system.SecurityUpdate, "uuid", false))
}

func TestPreUpgradeCmdSuccessHookHasBackedUp(t *testing.T) {
	redirectRebootCheckOptionPaths(t)
	installFakeImmutableCtl(t)
	cfg := newTestConfig(t)
	sm := newTestStatusManager(t)
	sm.SetABStatus(system.AllInstallUpdate, system.HasBackedUp, system.NoABError)
	m := &Manager{
		config:           cfg,
		statusManager:    sm,
		updatePlatform:   updateplatform.NewUpdatePlatformManager(cfg, false),
		userAgents:       newUserAgentMap(),
		systemd:          newFailingSystemdManager(t),
		immutableManager: newImmutableManager(func(info system.JobProgressInfo) {}),
	}
	job := NewJob(newTestService(), "id", "name", nil, system.UpdateJobType, LockQueue, nil)
	assert.NoError(t, m.preUpgradeCmdSuccessHook(job, system.SecurityUpdate, "uuid", false))
}

func TestCancelAllUpdateJobWithJob(t *testing.T) {
	jm := newTestJobManager()
	j := NewJob(nil, "update-1", "test", nil, system.UpdateJobType, LockQueue, nil)
	require.NoError(t, jm.addJob(j))
	m := &Manager{jobManager: jm}
	assert.NoError(t, m.cancelAllUpdateJob())
	assert.Equal(t, system.EndStatus, j.Status)
}
