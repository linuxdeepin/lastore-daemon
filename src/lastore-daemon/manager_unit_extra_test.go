// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSystemdManager embeds the generated Manager interface and overrides only
// the unit-controlling methods under test, so stopTimerUnit/updateTimerUnit can
// be exercised without a real systemd bus (which would require polkit auth).
type fakeSystemdManager struct {
	systemd1.Manager
	getUnitErr  error
	stopUnitErr error
}

func (f *fakeSystemdManager) GetUnit(flags dbus.Flags, name string) (dbus.ObjectPath, error) {
	return dbus.ObjectPath("/org/freedesktop/systemd1/unit/fake"), f.getUnitErr
}

func (f *fakeSystemdManager) StopUnit(flags dbus.Flags, name string, mode string) (dbus.ObjectPath, error) {
	return dbus.ObjectPath(""), f.stopUnitErr
}

func TestGetLastoreSystemUnitMap(t *testing.T) {
	m := &Manager{config: newTestConfig(t), updater: &Updater{}}
	unitMap := m.getLastoreSystemUnitMap()
	assert.NotNil(t, unitMap)
}

// stubSystemdRun redirects systemd-run to a no-op binary so tests exercising
// startOfflineTask/updateTimerUnit do not trigger polkit authentication for
// managing system services.
func stubSystemdRun(t *testing.T) {
	t.Helper()
	old := systemdRunBin
	systemdRunBin = "/bin/true"
	t.Cleanup(func() { systemdRunBin = old })
}

func TestStartOfflineTask(t *testing.T) {
	stubSystemdRun(t)
	m := &Manager{config: newTestConfig(t), updater: &Updater{}}
	m.startOfflineTask()
}

func TestUpdateAutoCheckSystemUnitImmutable(t *testing.T) {
	m := &Manager{ImmutableAutoRecovery: true, systemd: newFailingSystemdManager(t)}
	assert.Error(t, m.updateAutoCheckSystemUnit())
}

func TestStopTimerUnit(t *testing.T) {
	m := &Manager{systemd: newFailingSystemdManager(t)}
	assert.Error(t, m.stopTimerUnit(lastoreAutoCheck))
}

func TestUpdateTimerUnit(t *testing.T) {
	stubSystemdRun(t)
	m := &Manager{systemd: newFailingSystemdManager(t), config: newTestConfig(t), updater: &Updater{}}
	_ = m.updateTimerUnit(lastoreAutoCheck)
}

func TestHandleAutoDownloadImmutable(t *testing.T) {
	(&Manager{ImmutableAutoRecovery: true}).handleAutoDownload()
}

func TestHandleAbortAutoDownloadImmutable(t *testing.T) {
	(&Manager{ImmutableAutoRecovery: true}).handleAbortAutoDownload()
}

func TestHandleAbortAutoDownloadNoJob(t *testing.T) {
	(&Manager{jobManager: newTestJobManager()}).handleAbortAutoDownload()
}

func TestHandleAbortAutoDownloadUserInitiated(t *testing.T) {
	jm := newTestJobManager()
	j := NewJob(nil, system.PrepareDistUpgradeJobType, "test", nil, system.PrepareDistUpgradeJobType, LockQueue, nil)
	j.initiator = initiatorUser
	require.NoError(t, jm.addJob(j))

	(&Manager{jobManager: jm}).handleAbortAutoDownload()
	assert.Equal(t, system.ReadyStatus, j.Status)
}

func TestHandleAbortAutoDownloadAutoInitiated(t *testing.T) {
	jm := newTestJobManager()
	j := NewJob(nil, system.PrepareDistUpgradeJobType, "test", nil, system.PrepareDistUpgradeJobType, LockQueue, nil)
	j.initiator = initiatorAuto
	require.NoError(t, jm.addJob(j))

	(&Manager{jobManager: jm}).handleAbortAutoDownload()
	assert.Equal(t, system.EndStatus, j.Status)
}

func TestDelHandleSystemEventFailingConn(t *testing.T) {
	m := &Manager{service: newFailingConnService(t)}
	assert.Error(t, m.delHandleSystemEvent(ifcTestSender, "AutoCheck"))
}

func TestStopTimerUnitSuccess(t *testing.T) {
	m := &Manager{systemd: &fakeSystemdManager{}}
	assert.NoError(t, m.stopTimerUnit(lastoreAutoCheck))
}

func TestStopTimerUnitStopError(t *testing.T) {
	m := &Manager{systemd: &fakeSystemdManager{stopUnitErr: errors.New("stop failed")}}
	assert.Error(t, m.stopTimerUnit(lastoreAutoCheck))
}

func TestGetNextAutoCheckDelay(t *testing.T) {
	t.Run("intranet first run", func(t *testing.T) {
		m := &Manager{config: newTestConfig(t)}
		m.config.StartCheckRange = []int{100, 200}
		m.config.IntranetUpdate = true
		m.isAutoCheckTimerFirstRun = true
		d := m.getNextAutoCheckDelay()
		assert.GreaterOrEqual(t, d, 100)
		assert.Less(t, d, 200)
	})

	t.Run("intranet subsequent", func(t *testing.T) {
		m := &Manager{config: newTestConfig(t)}
		m.config.IntranetUpdate = true
		m.config.CheckInterval = 50 * time.Second
		assert.Equal(t, 50, m.getNextAutoCheckDelay())
	})

	t.Run("intranet negative interval", func(t *testing.T) {
		m := &Manager{config: newTestConfig(t)}
		m.config.IntranetUpdate = true
		m.config.CheckInterval = -time.Hour
		assert.Equal(t, 0, m.getNextAutoCheckDelay())
	})

	t.Run("invalid range falls back", func(t *testing.T) {
		m := &Manager{config: newTestConfig(t)}
		m.config.StartCheckRange = []int{}
		m.config.LastCheckTime = time.Now()
		m.config.CheckInterval = time.Minute
		d := m.getNextAutoCheckDelay()
		// 60s check interval + random within [1800, 21600)
		assert.GreaterOrEqual(t, d, 1860)
		assert.Less(t, d, 21660)
	})

	t.Run("valid range non intranet", func(t *testing.T) {
		m := &Manager{config: newTestConfig(t)}
		m.config.StartCheckRange = []int{100, 200}
		m.config.LastCheckTime = time.Now()
		m.config.CheckInterval = time.Minute
		d := m.getNextAutoCheckDelay()
		// 60s check interval + random within [100, 200)
		assert.GreaterOrEqual(t, d, 160)
		assert.Less(t, d, 260)
	})
}

func TestGetLastoreSystemUnitMapBranches(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cfg := newTestConfig(t)
		cfg.PostUpgradeCron = ""
		cfg.IntranetUpdate = false
		cfg.UpdateTime = ""
		m := &Manager{config: cfg, updater: &Updater{}}
		um := m.getLastoreSystemUnitMap()
		assert.Contains(t, um, lastoreAutoCheck)
		assert.Contains(t, um, lastoreAutoClean)
		assert.Contains(t, um, lastoreAutoUpdateToken)
		assert.Contains(t, um, watchOsVersion)
		assert.NotContains(t, um, lastoreInitIdleDownload)
		assert.NotContains(t, um, lastoreRetryPostMsg)
		assert.NotContains(t, um, lastoreRegularlyUpdate)
		assert.NotContains(t, um, UnitName(lastoreGatherInfo))
	})

	t.Run("immutable recovery", func(t *testing.T) {
		m := &Manager{config: newTestConfig(t), updater: &Updater{}, ImmutableAutoRecovery: true}
		um := m.getLastoreSystemUnitMap()
		assert.NotContains(t, um, lastoreAutoCheck)
		assert.NotContains(t, um, lastoreInitIdleDownload)
	})

	t.Run("idle download enabled", func(t *testing.T) {
		u := &Updater{}
		u.idleDownloadConfigObj = idleDownloadConfig{IdleDownloadEnabled: true}
		m := &Manager{config: newTestConfig(t), updater: u}
		assert.Contains(t, m.getLastoreSystemUnitMap(), lastoreInitIdleDownload)
	})

	t.Run("post upgrade cron and intranet", func(t *testing.T) {
		cfg := newTestConfig(t)
		cfg.PostUpgradeCron = "0/30"
		cfg.IntranetUpdate = true
		cfg.UpdateTime = time.Now().Add(time.Hour).Format(time.RFC3339)
		m := &Manager{config: cfg, updater: &Updater{}}
		um := m.getLastoreSystemUnitMap()
		assert.Contains(t, um, lastoreRetryPostMsg)
		assert.Contains(t, um, lastoreRegularlyUpdate)
		assert.Contains(t, um, UnitName(lastoreGatherInfo))
	})
}
