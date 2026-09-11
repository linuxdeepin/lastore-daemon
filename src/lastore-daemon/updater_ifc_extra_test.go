// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
)

// newFailingUpdater builds an Updater whose manager rejects every invocation
// (failing bus conn), so permission-gated DBus methods hit their early return.
func newFailingUpdater(t *testing.T) *Updater {
	t.Helper()
	return &Updater{
		service: newTestService(),
		manager: &Manager{service: newFailingConnService(t)},
		config:  config.NewConfig(""),
	}
}

// newRootUpdater builds an Updater whose manager sees every caller as root, so
// permission-gated DBus methods reach their post-permission branches without a
// real bus or polkit. The config is backed by a temp file so writes are safe.
func newRootUpdater(t *testing.T) *Updater {
	t.Helper()
	svc := newRootConnService(t)
	return &Updater{
		service: newTestService(),
		manager: &Manager{service: svc},
		config:  newTestConfig(t),
	}
}

func TestUpdaterGetCheckIntervalAndTimePermissionDenied(t *testing.T) {
	u := newFailingUpdater(t)
	interval, checkTime, busErr := u.GetCheckIntervalAndTime(ifcTestSender)
	assert.Equal(t, float64(0), interval)
	assert.Equal(t, "", checkTime)
	assert.NotNil(t, busErr)
}

func TestUpdaterSetAutoCheckUpdatesPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetAutoCheckUpdates(ifcTestSender, true))
}

func TestUpdaterSetAutoDownloadUpdatesPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetAutoDownloadUpdates(ifcTestSender, true))
}

func TestUpdaterSetAutoDownloadUpdatesEqualEarlyReturn(t *testing.T) {
	u := &Updater{
		config:              config.NewConfig(""),
		AutoDownloadUpdates: true,
	}
	assert.NoError(t, u.setAutoDownloadUpdates(true))
}

func TestUpdaterListMirrorSourcesPermissionDenied(t *testing.T) {
	sources, busErr := newFailingUpdater(t).ListMirrorSources(ifcTestSender, "zh_CN")
	assert.Nil(t, sources)
	assert.NotNil(t, busErr)
}

func TestUpdaterSetMirrorSourcePermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetMirrorSource(ifcTestSender, "mirror1"))
}

func TestUpdaterSetUpdateNotifyPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetUpdateNotify(ifcTestSender, true))
}

func TestUpdaterSetIdleDownloadConfigPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetIdleDownloadConfig(ifcTestSender, "{}"))
}

func TestUpdaterSetIdleDownloadConfigInvalidJSON(t *testing.T) {
	u := &Updater{}
	assert.Error(t, u.setIdleDownloadConfig(""))
}

func TestUpdaterSetDownloadSpeedLimitPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetDownloadSpeedLimit(ifcTestSender, "{}"))
}

func TestUpdaterSetP2PUpdateEnablePolkitDenied(t *testing.T) {
	// SetP2PUpdateEnable calls polkit.CheckAuth, which dials the real system bus
	// and would pop the polkit auth dialog on a desktop session. Point the bus at
	// a nonexistent socket so CheckAuth fails fast without touching polkit.
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")

	u := newFailingUpdater(t)
	assert.NotNil(t, u.SetP2PUpdateEnable(dbus.Sender(":1.999"), true))
}

func TestUpdaterCleanTransmissionFilesPolkitDenied(t *testing.T) {
	// CleanTransmissionFiles calls polkit.CheckAuth, which dials the real system
	// bus and would pop the polkit auth dialog on a desktop session. Point the bus
	// at a nonexistent socket so CheckAuth fails fast without touching polkit.
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")

	u := newFailingUpdater(t)
	assert.NotNil(t, u.CleanTransmissionFiles(dbus.Sender(":1.999")))
}

func TestUpdaterSetDeliveryDownloadSpeedLimitPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetDeliveryDownloadSpeedLimit(ifcTestSender, "{}"))
}

func TestUpdaterSetDeliveryUploadSpeedLimitPermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetDeliveryUploadSpeedLimit(ifcTestSender, "{}"))
}

// --- post-permission branches ---

func TestUpdaterGetCheckIntervalAndTime(t *testing.T) {
	u := newRootUpdater(t)
	interval, checkTime, busErr := u.GetCheckIntervalAndTime(ifcTestSender)
	assert.Nil(t, busErr)
	assert.Greater(t, interval, float64(0))
	assert.NotEmpty(t, checkTime)
}

func TestUpdaterSetAutoCheckUpdates(t *testing.T) {
	u := newRootUpdater(t)
	u.AutoCheckUpdates = false
	assert.Nil(t, u.SetAutoCheckUpdates(ifcTestSender, true))
	assert.True(t, u.AutoCheckUpdates)
}

func TestUpdaterSetAutoCheckUpdatesEqualEarlyReturn(t *testing.T) {
	u := newRootUpdater(t)
	u.AutoCheckUpdates = true
	assert.Nil(t, u.SetAutoCheckUpdates(ifcTestSender, true))
}

func TestUpdaterSetAutoDownloadUpdates(t *testing.T) {
	u := newRootUpdater(t)
	u.AutoDownloadUpdates = false
	assert.Nil(t, u.SetAutoDownloadUpdates(ifcTestSender, true))
	assert.True(t, u.AutoDownloadUpdates)
}

func TestUpdaterSetUpdateNotify(t *testing.T) {
	u := newRootUpdater(t)
	u.UpdateNotify = false
	assert.Nil(t, u.SetUpdateNotify(ifcTestSender, true))
	assert.True(t, u.UpdateNotify)
}

func TestUpdaterSetUpdateNotifyEqualEarlyReturn(t *testing.T) {
	u := newRootUpdater(t)
	u.UpdateNotify = true
	assert.Nil(t, u.SetUpdateNotify(ifcTestSender, true))
}

func TestUpdaterSetIdleDownloadConfig(t *testing.T) {
	u := newRootUpdater(t)
	// Pre-seed the property so the debounced timer callback observes no change
	// and never reaches applyIdleDownloadConfig (which touches systemd).
	u.IdleDownloadConfig = `{"IdleDownloadEnabled":true,"BeginTime":"10:00","EndTime":"11:00"}`
	assert.Nil(t, u.SetIdleDownloadConfig(ifcTestSender, `{"IdleDownloadEnabled":true,"BeginTime":"10:00","EndTime":"11:00"}`))
}

func TestUpdaterSetDownloadSpeedLimitInvalidJSON(t *testing.T) {
	assert.NotNil(t, newRootUpdater(t).SetDownloadSpeedLimit(ifcTestSender, "not-json"))
}

func TestUpdaterSetDownloadSpeedLimitOnlinePriority(t *testing.T) {
	u := newRootUpdater(t)
	u.downloadSpeedLimitConfigObj.IsOnlineSpeedLimit = true
	assert.Nil(t, u.SetDownloadSpeedLimit(ifcTestSender, `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"100","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDownloadSpeedLimit(t *testing.T) {
	u := newRootUpdater(t)
	u.DownloadSpeedLimitConfig = `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"100","IsOnlineSpeedLimit":false}`
	assert.Nil(t, u.SetDownloadSpeedLimit(ifcTestSender, `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"100","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryDownloadSpeedLimitInvalidJSON(t *testing.T) {
	assert.NotNil(t, newRootUpdater(t).SetDeliveryDownloadSpeedLimit(ifcTestSender, "not-json"))
}

func TestUpdaterSetDeliveryDownloadSpeedLimitInvalidRate(t *testing.T) {
	assert.NotNil(t, newRootUpdater(t).SetDeliveryDownloadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":true,"LimitSpeed":"abc","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryDownloadSpeedLimitDisabledSame(t *testing.T) {
	u := newRootUpdater(t)
	u.config.DeliveryLocalDownloadGlobalLimit = "{}"
	assert.Nil(t, u.SetDeliveryDownloadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":false,"LimitSpeed":"","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryUploadSpeedLimitInvalidJSON(t *testing.T) {
	assert.NotNil(t, newRootUpdater(t).SetDeliveryUploadSpeedLimit(ifcTestSender, "not-json"))
}

func TestUpdaterSetDeliveryUploadSpeedLimitInvalidRate(t *testing.T) {
	assert.NotNil(t, newRootUpdater(t).SetDeliveryUploadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":true,"LimitSpeed":"abc","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryUploadSpeedLimitDisabledSame(t *testing.T) {
	u := newRootUpdater(t)
	u.config.DeliveryLocalUploadGlobalLimit = "{}"
	assert.Nil(t, u.SetDeliveryUploadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":false,"LimitSpeed":"","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetAutoDownloadUpdatesHelper(t *testing.T) {
	u := &Updater{service: newTestService(), config: newTestConfig(t), AutoDownloadUpdates: false}
	assert.NoError(t, u.setAutoDownloadUpdates(true))
	assert.True(t, u.AutoDownloadUpdates)
}

func TestUpdaterSetDownloadSpeedLimitNoChange(t *testing.T) {
	u := &Updater{
		service:                  newTestService(),
		DownloadSpeedLimitConfig: `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"100","IsOnlineSpeedLimit":false}`,
	}
	cfg := downloadSpeedLimitConfig{DownloadSpeedLimitEnabled: true, LimitSpeed: "100", IsOnlineSpeedLimit: false}
	assert.NoError(t, u.setDownloadSpeedLimit(cfg))
}

func TestUpdaterSetDownloadSpeedLimitChanged(t *testing.T) {
	u := &Updater{
		service:                  newTestService(),
		manager:                  &Manager{jobManager: newTestJobManager()},
		config:                   newTestConfig(t),
		DownloadSpeedLimitConfig: `{"DownloadSpeedLimitEnabled":false,"LimitSpeed":"","IsOnlineSpeedLimit":false}`,
	}
	cfg := downloadSpeedLimitConfig{DownloadSpeedLimitEnabled: true, LimitSpeed: "100", IsOnlineSpeedLimit: false}
	assert.NoError(t, u.setDownloadSpeedLimit(cfg))
	// Let the debounced timer fire so the local-limit write callback runs.
	time.Sleep(1500 * time.Millisecond)
}

func TestUpdaterSetDownloadSpeedLimitOnlineTimer(t *testing.T) {
	u := &Updater{
		service:                  newTestService(),
		manager:                  &Manager{jobManager: newTestJobManager()},
		config:                   newTestConfig(t),
		DownloadSpeedLimitConfig: `{"DownloadSpeedLimitEnabled":false,"LimitSpeed":"","IsOnlineSpeedLimit":false}`,
	}
	cfg := downloadSpeedLimitConfig{DownloadSpeedLimitEnabled: true, LimitSpeed: "100", IsOnlineSpeedLimit: true}
	assert.NoError(t, u.setDownloadSpeedLimit(cfg))
	time.Sleep(1500 * time.Millisecond)
}

func TestUpdaterSetIdleDownloadConfigTimer(t *testing.T) {
	orig := systemdTimerDir
	systemdTimerDir = t.TempDir()
	t.Cleanup(func() { systemdTimerDir = orig })

	u := &Updater{
		service:        newTestService(),
		manager:        &Manager{ImmutableAutoRecovery: true},
		config:         newTestConfig(t),
		systemdManager: newFailingSystemdManager(t),
	}
	assert.NoError(t, u.setIdleDownloadConfig(`{"IdleDownloadEnabled":true,"BeginTime":"10:00","EndTime":"11:00"}`))
	time.Sleep(1500 * time.Millisecond)
}

func TestUpdaterSetAutoDownloadUpdatesDisableWithIdle(t *testing.T) {
	u := &Updater{
		service:               newTestService(),
		config:                newTestConfig(t),
		AutoDownloadUpdates:   true,
		idleDownloadConfigObj: idleDownloadConfig{IdleDownloadEnabled: true},
	}
	assert.NoError(t, u.setAutoDownloadUpdates(false))
	assert.False(t, u.AutoDownloadUpdates)
}

func TestUpdaterSetDeliveryDownloadSpeedLimitLocalSame(t *testing.T) {
	u := newRootUpdater(t)
	u.config.DeliveryLocalDownloadGlobalLimit = `{"LimitType":1,"LimitRate":10240,"CurrentRate":10240}`
	assert.Nil(t, u.SetDeliveryDownloadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":true,"LimitSpeed":"10","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryDownloadSpeedLimitOutOfRange(t *testing.T) {
	u := newRootUpdater(t)
	// An absurdly large rate is clamped to the default (10240 KB/s).
	u.config.DeliveryLocalDownloadGlobalLimit = `{"LimitType":1,"LimitRate":10485760,"CurrentRate":10485760}`
	assert.Nil(t, u.SetDeliveryDownloadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":true,"LimitSpeed":"9999999","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryUploadSpeedLimitLocalSame(t *testing.T) {
	u := newRootUpdater(t)
	u.config.DeliveryLocalUploadGlobalLimit = `{"LimitType":1,"LimitRate":10240,"CurrentRate":10240}`
	assert.Nil(t, u.SetDeliveryUploadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":true,"LimitSpeed":"10","IsOnlineSpeedLimit":false}`))
}

func TestUpdaterSetDeliveryUploadSpeedLimitOutOfRange(t *testing.T) {
	u := newRootUpdater(t)
	u.config.DeliveryLocalUploadGlobalLimit = `{"LimitType":1,"LimitRate":10485760,"CurrentRate":10485760}`
	assert.Nil(t, u.SetDeliveryUploadSpeedLimit(ifcTestSender, `{"SpeedLimitEnabled":true,"LimitSpeed":"9999999","IsOnlineSpeedLimit":false}`))
}
