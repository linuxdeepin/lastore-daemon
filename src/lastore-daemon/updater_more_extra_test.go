// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFailingSystemdManager(t *testing.T) systemd1.Manager {
	t.Helper()
	return systemd1.NewManager(newFailingConnService(t).Conn())
}

func TestUpdaterNewUpdater(t *testing.T) {
	u := NewUpdater(newFailingConnService(t), &Manager{}, config.NewConfig(""))
	require.NotNil(t, u)
}

func TestSetAPTSmartMirror(t *testing.T) {
	dir := t.TempDir()
	old := aptConfDir
	aptConfDir = dir
	t.Cleanup(func() { aptConfDir = old })

	require.NoError(t, SetAPTSmartMirror("http://example.com/mirror"))

	data, err := os.ReadFile(filepath.Join(dir, smartMirrorConfigName))
	require.NoError(t, err)
	assert.Equal(t, aptSmartMirrorConfig("http://example.com/mirror"), string(data))
}

func TestUpdaterSetMirrorSourceEmptyID(t *testing.T) {
	u := &Updater{}
	assert.NoError(t, u.setMirrorSource(""))
}

func TestUpdaterDisableDeliveryServiceFailingConn(t *testing.T) {
	u := &Updater{service: newFailingConnService(t)}
	assert.Error(t, u.disableDeliveryService())
}

func TestUpdaterRefreshUpgradeDeliveryService(t *testing.T) {
	u := &Updater{service: newFailingConnService(t), config: config.NewConfig("")}
	u.refreshUpgradeDeliveryService()
}

func TestUpdaterListMirrorSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mirrors.json")
	require.NoError(t, os.WriteFile(path, []byte(`[
		{"id": "Aliyun", "url": "https://aliyun.example.com/deepin/", "weight": 10,
		 "name_locale": {"zh_CN": "阿里云"}},
		{"id": "Disabled", "url": "https://disabled.example.com/deepin/", "weight": -1}
	]`), 0644))
	old := mirrorsJSONPath
	mirrorsJSONPath = path
	t.Cleanup(func() { mirrorsJSONPath = old })

	got := loadMirrorSources("zh_CN")
	require.Len(t, got, 1, "negative-weight sources must be filtered out")
	assert.Equal(t, "Aliyun", got[0].Id)
	assert.Equal(t, "https://aliyun.example.com/deepin/", got[0].Url)
	assert.Equal(t, "阿里云", got[0].Name)
}

func TestUpdaterSetInstallUpdateTimePermissionDenied(t *testing.T) {
	assert.NotNil(t, newFailingUpdater(t).SetInstallUpdateTime(ifcTestSender, ""))
}

func TestUpdaterSetClassifiedUpdatablePackages(t *testing.T) {
	u := &Updater{service: newTestService(), config: config.NewConfig("")}
	u.setClassifiedUpdatablePackages(map[string][]string{})
}

func TestUpdaterAutoInstallUpdatesWriteCallbackPermissionDenied(t *testing.T) {
	u := newFailingUpdater(t)
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          true,
	}
	assert.NotNil(t, u.autoInstallUpdatesWriteCallback(pw))
}

func TestUpdaterAutoInstallUpdatesSuitesWriteCallbackPermissionDenied(t *testing.T) {
	u := newFailingUpdater(t)
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          uint64(0),
	}
	assert.NotNil(t, u.autoInstallUpdatesSuitesWriteCallback(pw))
}

func TestUpdaterInitIdleDownloadConfigEmptyRange(t *testing.T) {
	u := &Updater{
		manager:               &Manager{},
		idleDownloadConfigObj: idleDownloadConfig{IdleDownloadEnabled: true},
	}
	assert.Error(t, u.initIdleDownloadConfig())
}

func TestUpdaterApplyIdleDownloadConfigImmediatelyEmptyRange(t *testing.T) {
	u := &Updater{manager: &Manager{}}
	err := u.applyIdleDownloadConfigImmediately(idleDownloadConfig{IdleDownloadEnabled: true}, time.Now())
	assert.Error(t, err)
}

func TestUpdaterApplyIdleDownloadConfigBadTime(t *testing.T) {
	u := &Updater{systemdManager: newFailingSystemdManager(t)}
	err := u.applyIdleDownloadConfig(idleDownloadConfig{}, time.Now(), true)
	assert.Error(t, err)
}

func TestUpdaterEnableAndStartTimerUnitsFailingConn(t *testing.T) {
	u := &Updater{systemdManager: newFailingSystemdManager(t)}
	_, err := u.enableAndStartTimerUnits([]string{"foo.timer"})
	assert.Error(t, err)
}

func TestUpdaterDisableAndStopTimerUnitsFailingConn(t *testing.T) {
	u := &Updater{systemdManager: newFailingSystemdManager(t)}
	_, err := u.disableAndStopTimerUnits([]string{"foo.timer"})
	assert.Error(t, err)
}

func TestUpdaterGetP2PUnitFailingConn(t *testing.T) {
	u := &Updater{systemdManager: newFailingSystemdManager(t)}
	_, err := u.getP2PUnit()
	assert.Error(t, err)
}

func TestUpdaterDealSetP2PUpdateEnableUnsupported(t *testing.T) {
	u := &Updater{P2PUpdateSupport: false}
	assert.Error(t, u.dealSetP2PUpdateEnable(true))
}

// --- additional branch coverage ---

func TestUpdaterSetMirrorSourceSameID(t *testing.T) {
	u := &Updater{MirrorSource: "same"}
	assert.NoError(t, u.setMirrorSource("same"))
}

func TestUpdaterSetMirrorSourceNotFound(t *testing.T) {
	u := &Updater{MirrorSource: "other"}
	assert.Error(t, u.setMirrorSource("nonexistent-mirror-id"))
}

func TestUpdaterRefreshUpgradeDeliveryServiceRootConn(t *testing.T) {
	u := &Updater{service: newRootConnService(t), config: newTestConfig(t)}
	u.refreshUpgradeDeliveryService()
}

func TestUpdaterSetInstallUpdateTimeInvalid(t *testing.T) {
	u := newRootUpdater(t)
	assert.NotNil(t, u.SetInstallUpdateTime(ifcTestSender, "not-a-time"))
}

func TestUpdaterAutoInstallUpdatesWriteCallback(t *testing.T) {
	u := newRootUpdater(t)
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          true,
	}
	assert.Nil(t, u.autoInstallUpdatesWriteCallback(pw))
}

func TestUpdaterAutoInstallUpdatesSuitesWriteCallback(t *testing.T) {
	u := newRootUpdater(t)
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          uint64(system.SystemUpdate),
	}
	assert.Nil(t, u.autoInstallUpdatesSuitesWriteCallback(pw))
}

func TestUpdaterApplyIdleDownloadConfigImmediatelyDisabled(t *testing.T) {
	u := &Updater{manager: &Manager{ImmutableAutoRecovery: true}}
	assert.NoError(t, u.applyIdleDownloadConfigImmediately(idleDownloadConfig{IdleDownloadEnabled: false}, time.Now()))
}

func TestUpdaterApplyIdleDownloadConfigImmediatelyEnabled(t *testing.T) {
	u := &Updater{manager: &Manager{ImmutableAutoRecovery: true}}
	cfg := idleDownloadConfig{IdleDownloadEnabled: true, BeginTime: "00:00", EndTime: "23:59"}
	assert.NoError(t, u.applyIdleDownloadConfigImmediately(cfg, time.Now()))
}

func TestUpdaterDealSetP2PUpdateEnableNoop(t *testing.T) {
	u := &Updater{P2PUpdateSupport: true, P2PUpdateEnable: true}
	assert.NoError(t, u.dealSetP2PUpdateEnable(true))
}

func TestUpdaterDealSetP2PUpdateEnableFailingSystemd(t *testing.T) {
	u := &Updater{P2PUpdateSupport: true, systemdManager: newFailingSystemdManager(t)}
	assert.Error(t, u.dealSetP2PUpdateEnable(true))
}

func TestUpdaterApplyIdleDownloadConfigDisabled(t *testing.T) {
	orig := systemdTimerDir
	systemdTimerDir = t.TempDir()
	t.Cleanup(func() { systemdTimerDir = orig })

	u := &Updater{
		manager:        &Manager{ImmutableAutoRecovery: true},
		systemdManager: newFailingSystemdManager(t),
	}
	cfg := idleDownloadConfig{IdleDownloadEnabled: false, BeginTime: "10:00", EndTime: "11:00"}
	assert.Error(t, u.applyIdleDownloadConfig(cfg, time.Now(), false))
}
