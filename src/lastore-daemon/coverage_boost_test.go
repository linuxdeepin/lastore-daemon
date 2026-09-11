// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// controllableSystemdManager embeds the generated systemd1.Manager interface and
// overrides the unit-file/unit-lifecycle methods so enableAndStartTimerUnits,
// disableAndStopTimerUnits and dealSetP2PUpdateEnable can be driven through
// their success and failure paths without a real systemd bus (which would need
// polkit auth).
type controllableSystemdManager struct {
	systemd1.Manager
	enableUnitFilesErr  error
	disableUnitFilesErr error
	startUnitErr        error
	stopUnitErr         error
	restartUnitErr      error
	getUnitErr          error
	reloadErr           error
	changes             []systemd1.UnitFileChange
}

func (f *controllableSystemdManager) EnableUnitFiles(flags dbus.Flags, files []string, runtime bool, force bool) (bool, []systemd1.UnitFileChange, error) {
	if f.enableUnitFilesErr != nil {
		return false, nil, f.enableUnitFilesErr
	}
	return true, f.changes, nil
}

func (f *controllableSystemdManager) DisableUnitFiles(flags dbus.Flags, files []string, runtime bool) ([]systemd1.UnitFileChange, error) {
	if f.disableUnitFilesErr != nil {
		return nil, f.disableUnitFilesErr
	}
	return f.changes, nil
}

func (f *controllableSystemdManager) StartUnit(flags dbus.Flags, name string, mode string) (dbus.ObjectPath, error) {
	if f.startUnitErr != nil {
		return dbus.ObjectPath(""), f.startUnitErr
	}
	return dbus.ObjectPath("/org/freedesktop/systemd1/unit/fake"), nil
}

func (f *controllableSystemdManager) StopUnit(flags dbus.Flags, name string, mode string) (dbus.ObjectPath, error) {
	if f.stopUnitErr != nil {
		return dbus.ObjectPath(""), f.stopUnitErr
	}
	return dbus.ObjectPath(""), nil
}

func (f *controllableSystemdManager) RestartUnit(flags dbus.Flags, name string, mode string) (dbus.ObjectPath, error) {
	if f.restartUnitErr != nil {
		return dbus.ObjectPath(""), f.restartUnitErr
	}
	return dbus.ObjectPath("/org/freedesktop/systemd1/unit/fake"), nil
}

func (f *controllableSystemdManager) GetUnit(flags dbus.Flags, name string) (dbus.ObjectPath, error) {
	if f.getUnitErr != nil {
		return dbus.ObjectPath(""), f.getUnitErr
	}
	return dbus.ObjectPath("/org/freedesktop/systemd1/unit/fake"), nil
}

func (f *controllableSystemdManager) Reload(flags dbus.Flags) error {
	return f.reloadErr
}

func TestHandleDownloadLimitChangedEnabled(t *testing.T) {
	m := &Manager{updater: &Updater{downloadSpeedLimitConfigObj: downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: true,
		LimitSpeed:                "2048",
	}}}
	job := &Job{option: map[string]string{}}
	m.handleDownloadLimitChanged(job)
	assert.Equal(t, "2048", job.option[aptHttpLimitKey])
	assert.Equal(t, "2048", job.option[aptDeliveryLimitKey])
}

func TestHandleDownloadLimitChangedOnline(t *testing.T) {
	m := &Manager{updater: &Updater{downloadSpeedLimitConfigObj: downloadSpeedLimitConfig{
		IsOnlineSpeedLimit: true,
		LimitSpeed:         "512",
	}}}
	job := &Job{}
	m.handleDownloadLimitChanged(job)
	assert.Equal(t, "512", job.option[aptHttpLimitKey])
	assert.Equal(t, "512", job.option[aptDeliveryLimitKey])
}

func TestHandleDownloadLimitChangedRemovesOptions(t *testing.T) {
	m := &Manager{updater: &Updater{}}
	job := &Job{option: map[string]string{
		aptHttpLimitKey:     "1024",
		aptDeliveryLimitKey: "1024",
	}}
	m.handleDownloadLimitChanged(job)
	_, hasHTTP := job.option[aptHttpLimitKey]
	_, hasDelivery := job.option[aptDeliveryLimitKey]
	assert.False(t, hasHTTP)
	assert.False(t, hasDelivery)
}

func TestEnableAndStartTimerUnitsSuccess(t *testing.T) {
	u := &Updater{systemdManager: &controllableSystemdManager{
		changes: []systemd1.UnitFileChange{{}},
	}}
	changed, err := u.enableAndStartTimerUnits([]string{"lastore-idle-download.timer"})
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestEnableAndStartTimerUnitsEnableError(t *testing.T) {
	u := &Updater{systemdManager: &controllableSystemdManager{
		enableUnitFilesErr: assert.AnError,
	}}
	changed, err := u.enableAndStartTimerUnits([]string{"lastore-idle-download.timer"})
	require.Error(t, err)
	assert.False(t, changed)
}

func TestDisableAndStopTimerUnitsSuccess(t *testing.T) {
	u := &Updater{systemdManager: &controllableSystemdManager{
		changes: []systemd1.UnitFileChange{{}},
	}}
	changed, err := u.disableAndStopTimerUnits([]string{"lastore-idle-download.timer"})
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestDisableAndStopTimerUnitsDisableError(t *testing.T) {
	u := &Updater{systemdManager: &controllableSystemdManager{
		disableUnitFilesErr: assert.AnError,
	}}
	changed, err := u.disableAndStopTimerUnits([]string{"lastore-idle-download.timer"})
	require.Error(t, err)
	assert.False(t, changed)
}

func TestDealSetP2PUpdateEnableEnableSuccess(t *testing.T) {
	u := &Updater{
		service:          newTestService(),
		systemdManager:   &controllableSystemdManager{},
		P2PUpdateSupport: true,
		P2PUpdateEnable:  false,
	}
	require.NoError(t, u.dealSetP2PUpdateEnable(true))
	assert.True(t, u.P2PUpdateEnable)
}

func TestDealSetP2PUpdateEnableDisableSuccess(t *testing.T) {
	u := &Updater{
		service:          newTestService(),
		systemdManager:   &controllableSystemdManager{},
		P2PUpdateSupport: true,
		P2PUpdateEnable:  true,
	}
	require.NoError(t, u.dealSetP2PUpdateEnable(false))
	assert.False(t, u.P2PUpdateEnable)
}

func TestGetUpgradeUrlsFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sources.list")
	content := "deb http://example.com beige main\ndeb https://secure.example.com beige-security main contrib\n"
	require.NoError(t, os.WriteFile(src, []byte(content), 0644))

	urls := getUpgradeUrls(src)
	assert.Equal(t, []string{"http://example.com", "https://secure.example.com"}, urls)
}

func TestGetUpgradeUrlsDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.list"), []byte("deb https://a.example.com beige main\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.list"), []byte("deb ftp://b.example.com beige main\n"), 0644))
	// 屏蔽的仓库地址(被 # 注释)不应被收集
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.list"), []byte("# deb http://disabled.example.com beige main\n"), 0644))

	urls := getUpgradeUrls(dir)
	assert.ElementsMatch(t, []string{"https://a.example.com", "ftp://b.example.com"}, urls)
}

func TestGetUpgradeUrlsMissingPath(t *testing.T) {
	assert.Nil(t, getUpgradeUrls(filepath.Join(t.TempDir(), "nonexistent")))
}

func TestUpdateSecurityConfigFileCreate(t *testing.T) {
	dir := t.TempDir()
	old := aptConfDir
	aptConfDir = dir
	t.Cleanup(func() { aptConfDir = old })

	require.NoError(t, updateSecurityConfigFile(true))
	data, err := os.ReadFile(filepath.Join(dir, securityConfFileName))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Dir::Etc::SourceParts")
	assert.Contains(t, string(data), system.SecurityList)
}

func TestUpdateSecurityConfigFileCreateAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	old := aptConfDir
	aptConfDir = dir
	t.Cleanup(func() { aptConfDir = old })

	require.NoError(t, os.WriteFile(filepath.Join(dir, securityConfFileName), []byte("existing"), 0644))
	require.NoError(t, updateSecurityConfigFile(true))

	data, err := os.ReadFile(filepath.Join(dir, securityConfFileName))
	require.NoError(t, err)
	assert.Equal(t, "existing", string(data))
}

func TestUpdateSecurityConfigFileRemove(t *testing.T) {
	dir := t.TempDir()
	old := aptConfDir
	aptConfDir = dir
	t.Cleanup(func() { aptConfDir = old })

	require.NoError(t, os.WriteFile(filepath.Join(dir, securityConfFileName), []byte("x"), 0644))
	require.NoError(t, updateSecurityConfigFile(false))

	_, err := os.Stat(filepath.Join(dir, securityConfFileName))
	assert.True(t, os.IsNotExist(err))
}

func TestMakeEnvironWithSenderProxyFiltering(t *testing.T) {
	agent := newMockLastoreAgent()
	agent.MockInterfaceAgent.On("GetManualProxy", mock.Anything).Return(map[string]string{
		"http_proxy":  "http://proxy.example.com:8080",
		"https_proxy": "https://proxy.example.com:8080",
		"LD_PRELOAD":  "/tmp/evil.so",
	}, nil)

	// GetConnPID fails on a failing conn, so makeEnvironWithSender returns an
	// error; the proxy-filtering loop above it is still exercised (and would
	// drop the non-whitelisted LD_PRELOAD key before reaching the bus call).
	m := &Manager{userAgents: newUserAgentMapWithAgent(agent), service: newFailingConnService(t)}
	_, err := makeEnvironWithSender(m, ifcTestSender)
	assert.Error(t, err)
}

func TestPrepareFullScreenUpgradeEmptyOptionRestartSuccess(t *testing.T) {
	m := &Manager{
		service:      newRootConnService(t),
		loginManager: login1.NewManager(newRootConnService(t).Conn()),
		systemd:      &controllableSystemdManager{},
	}
	assert.Nil(t, m.PrepareFullScreenUpgrade(ifcTestSender, ""))
}

func TestPrepareFullScreenUpgradeEmptyOptionRestartError(t *testing.T) {
	m := &Manager{
		service:      newRootConnService(t),
		loginManager: login1.NewManager(newRootConnService(t).Conn()),
		systemd:      &controllableSystemdManager{restartUnitErr: assert.AnError},
	}
	assert.NotNil(t, m.PrepareFullScreenUpgrade(ifcTestSender, ""))
}

func TestUpdaterSetDownloadSpeedLimitSuccess(t *testing.T) {
	u := newRootUpdater(t)
	u.manager.jobManager = newTestJobManager()
	assert.Nil(t, u.SetDownloadSpeedLimit(ifcTestSender, `{"LimitSpeed":"2048","DownloadSpeedLimitEnabled":true}`))
	assert.Equal(t, "2048", u.downloadSpeedLimitConfigObj.LimitSpeed)
	if u.setDownloadSpeedLimitTimer != nil {
		u.setDownloadSpeedLimitTimer.Stop()
	}
}

func TestUpdaterSetDownloadSpeedLimitOnlineSkip(t *testing.T) {
	u := newRootUpdater(t)
	u.manager.jobManager = newTestJobManager()
	u.downloadSpeedLimitConfigObj.IsOnlineSpeedLimit = true
	assert.Nil(t, u.SetDownloadSpeedLimit(ifcTestSender, `{"LimitSpeed":"9999"}`))
	assert.NotEqual(t, "9999", u.downloadSpeedLimitConfigObj.LimitSpeed)
}

func TestUpdaterSetIdleDownloadConfigTimerReset(t *testing.T) {
	u := &Updater{service: newTestService(), config: newTestConfig(t)}
	require.NoError(t, u.setIdleDownloadConfig(`{"BeginTime":"10:00","EndTime":"11:00"}`))
	require.NoError(t, u.setIdleDownloadConfig(`{"BeginTime":"10:00","EndTime":"11:00"}`))
	if u.setIdleDownloadConfigTimer != nil {
		u.setIdleDownloadConfigTimer.Stop()
	}
}

func TestOsTreeRefreshFullMerge(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})
	assert.NoError(t, mgr.osTreeRefresh(true))
}

func TestOsTreeRefreshError(t *testing.T) {
	installFakeImmutableCtlScript(t, "#!/bin/sh\nexit 1\n")
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})
	assert.Error(t, mgr.osTreeRefresh(false))
}

func TestOsTreeFinalizeError(t *testing.T) {
	installFakeImmutableCtlScript(t, "#!/bin/sh\nexit 1\n")
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})
	assert.Error(t, mgr.osTreeFinalize())
}

func TestOsTreeRollbackError(t *testing.T) {
	installFakeImmutableCtlScript(t, "#!/bin/sh\nexit 1\n")
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})
	assert.Error(t, mgr.osTreeRollback())
}

func TestSendNotifyNoAgentListUsersError(t *testing.T) {
	m := &Manager{
		updater:      &Updater{UpdateNotify: true},
		config:       newTestConfig(t),
		userAgents:   newUserAgentMap(),
		loginManager: login1.NewManager(newFailingConnService(t).Conn()),
	}
	assert.Equal(t, uint32(0), m.sendNotify("app", 0, "icon", "summary", "body", nil, nil, -1))
}

func TestSendNotifyNoAgentIntranetIcon(t *testing.T) {
	m := &Manager{
		updater:      &Updater{UpdateNotify: true},
		config:       newTestConfig(t),
		userAgents:   newUserAgentMap(),
		loginManager: login1.NewManager(newFailingConnService(t).Conn()),
	}
	m.config.IntranetUpdate = true
	assert.Equal(t, uint32(0), m.sendNotify("app", 0, "icon", "summary", "body", nil, nil, -1))
}

func TestManagerIfcSetUpdateSourcesSystem(t *testing.T) {
	m := newRootManager(t)
	m.config = newTestConfig(t)
	m.systemSourceConfig = make(UpdateSourceConfig)
	m.securitySourceConfig = make(UpdateSourceConfig)
	assert.Nil(t, m.SetUpdateSources(ifcTestSender, system.SystemUpdate, config.OSDefaultRepo, nil, false))
}

func TestManagerIfcSetUpdateSourcesSecurity(t *testing.T) {
	m := newRootManager(t)
	m.config = newTestConfig(t)
	m.systemSourceConfig = make(UpdateSourceConfig)
	m.securitySourceConfig = make(UpdateSourceConfig)
	assert.Nil(t, m.SetUpdateSources(ifcTestSender, system.SecurityUpdate, config.OSDefaultRepo, nil, false))
}

func TestUserAgentMapRecoverLastoreAgentsListSessionsError(t *testing.T) {
	record := `{"ActiveUid":"1000","UidInfoMap":{"1000":{"Sessions":["/org/freedesktop/login1/session/_31"],"Agents":{"/org/deepin/dde/Lastore1/Agent":":1.42"},"Lang":"zh_CN.UTF-8"}}}`
	f := filepath.Join(t.TempDir(), "agentCache.json")
	require.NoError(t, os.WriteFile(f, []byte(record), 0644))
	old := userAgentRecordPath
	userAgentRecordPath = f
	t.Cleanup(func() { userAgentRecordPath = old })

	m := newUserAgentMap()
	m.recoverLastoreAgents(newFailingConnService(t), func(sessionId string, sessionPath dbus.ObjectPath) {
		t.Fatal("sessionNew should not be called when ListSessions fails")
	})
}

func TestManagerIfcRegisterAgentListSessionsError(t *testing.T) {
	m := &Manager{
		service:      newRootConnService(t),
		userAgents:   newUserAgentMap(),
		loginManager: login1.NewManager(newFailingConnService(t).Conn()),
	}
	assert.NotNil(t, m.RegisterAgent(ifcTestSender, lastoreAgentPath))
}

func TestManagerIfcUnRegisterAgentRemoveError(t *testing.T) {
	m := &Manager{service: newRootConnService(t), userAgents: newUserAgentMap()}
	assert.NotNil(t, m.UnRegisterAgent(ifcTestSender, lastoreAgentPath))
}

func TestManagerIfcUnRegisterAgentSuccess(t *testing.T) {
	m := &Manager{service: newRootConnService(t), userAgents: newUserAgentMap()}
	agent := newMockLastoreAgent()
	m.userAgents.addAgent("0", agent)
	assert.Nil(t, m.UnRegisterAgent(ifcTestSender, lastoreAgentPath))
}

func TestReportLogWithAgent(t *testing.T) {
	agent := newMockLastoreAgent()
	agent.MockInterfaceAgent.On("ReportLog", mock.Anything, mock.Anything).Return(nil)
	m := &Manager{userAgents: newUserAgentMapWithAgent(agent)}

	m.reportLog(updateStatusReport, true, "desc")
	m.reportLog(downloadStatusReport, true, "desc")
	m.reportLog(upgradeStatusReport, true, "desc")
}

func TestReportLogNoAgent(t *testing.T) {
	m := &Manager{userAgents: newUserAgentMap()}
	m.reportLog(updateStatusReport, true, "desc")
}

func TestUpdateModeWriteCallbackSuccess(t *testing.T) {
	m := newRootManager(t)
	m.statusManager = newTestStatusManager(t)
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          uint64(system.SystemUpdate),
	}
	assert.Nil(t, m.updateModeWriteCallback(pw))
}

func TestCheckUpdateModeWriteCallbackSuccess(t *testing.T) {
	m := newRootManager(t)
	m.statusManager = newTestStatusManager(t)
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          uint64(system.SystemUpdate),
	}
	assert.Nil(t, m.checkUpdateModeWriteCallback(pw))
}

func TestHandleAutoCleanEventDisabled(t *testing.T) {
	m := &Manager{config: newTestConfig(t), AutoClean: false}
	assert.NoError(t, m.handleAutoCleanEvent())
}

func TestUpdateAutoRecoveryStatusOverlay(t *testing.T) {
	f := filepath.Join(t.TempDir(), "booted")
	require.NoError(t, os.WriteFile(f, []byte("overlay"), 0644))
	old := deepinImmutableBootedFile
	deepinImmutableBootedFile = f
	t.Cleanup(func() { deepinImmutableBootedFile = old })

	m := &Manager{service: newTestService(), systemd: &controllableSystemdManager{}}
	m.updateAutoRecoveryStatus()
	assert.True(t, m.ImmutableAutoRecovery)
}

func TestUpdateAutoRecoveryStatusDisabled(t *testing.T) {
	f := filepath.Join(t.TempDir(), "booted")
	require.NoError(t, os.WriteFile(f, []byte("plain"), 0644))
	old := deepinImmutableBootedFile
	deepinImmutableBootedFile = f
	t.Cleanup(func() { deepinImmutableBootedFile = old })

	m := &Manager{service: newTestService()}
	m.updateAutoRecoveryStatus()
	assert.False(t, m.ImmutableAutoRecovery)
}

func TestDelHandleSystemEventInvalidType(t *testing.T) {
	m := newRootManager(t)
	assert.NotNil(t, m.delHandleSystemEvent(ifcTestSender, "invalid-event"))
}

func TestDelHandleSystemEventAutoDownloadDisabled(t *testing.T) {
	m := newRootManager(t)
	m.updater = &Updater{}
	assert.Nil(t, m.delHandleSystemEvent(ifcTestSender, "AutoDownload"))
}

func TestDelHandleSystemEventAbortAutoDownloadDisabled(t *testing.T) {
	m := newRootManager(t)
	m.updater = &Updater{}
	assert.Nil(t, m.delHandleSystemEvent(ifcTestSender, "AbortAutoDownload"))
}

func TestCloseNotifyWithAgent(t *testing.T) {
	agent := newMockLastoreAgent()
	agent.MockInterfaceAgent.On("CloseNotification", mock.Anything, mock.Anything).Return(nil)
	m := &Manager{userAgents: newUserAgentMapWithAgent(agent)}
	assert.NoError(t, m.closeNotify(42))
}

func TestCloseNotifyWithAgentError(t *testing.T) {
	agent := newMockLastoreAgent()
	agent.MockInterfaceAgent.On("CloseNotification", mock.Anything, mock.Anything).Return(assert.AnError)
	m := &Manager{userAgents: newUserAgentMapWithAgent(agent)}
	assert.NoError(t, m.closeNotify(42))
}

func TestHandleAutoCheckEventImmutable(t *testing.T) {
	m := &Manager{ImmutableAutoRecovery: true}
	assert.NoError(t, m.handleAutoCheckEvent())
}

func TestUpdatableAppsChanged(t *testing.T) {
	m := &Manager{service: newTestService()}
	m.UpgradableApps = []string{"app-a"}
	m.updatableApps([]string{"app-b"})
	assert.Equal(t, []string{"app-b"}, m.UpgradableApps)
}

func TestUpdatableAppsSameLengthDifferentContent(t *testing.T) {
	m := &Manager{service: newTestService()}
	m.UpgradableApps = []string{"app-a"}
	m.updatableApps([]string{"app-c"})
	assert.Equal(t, []string{"app-c"}, m.UpgradableApps)
}

func TestUpdaterGetP2PUnitSuccess(t *testing.T) {
	u := &Updater{service: newTestService(), systemdManager: &controllableSystemdManager{}}
	unit, err := u.getP2PUnit()
	assert.NoError(t, err)
	assert.NotNil(t, unit)
}

func TestApplyIdleDownloadConfigEnabled(t *testing.T) {
	old := systemdTimerDir
	systemdTimerDir = t.TempDir()
	t.Cleanup(func() { systemdTimerDir = old })

	u := &Updater{systemdManager: &controllableSystemdManager{changes: []systemd1.UnitFileChange{{}}}}
	err := u.applyIdleDownloadConfig(idleDownloadConfig{
		IdleDownloadEnabled: true,
		BeginTime:           "10:00",
		EndTime:             "11:00",
	}, time.Now(), true)
	assert.NoError(t, err)
}

func TestApplyIdleDownloadConfigDisabled(t *testing.T) {
	old := systemdTimerDir
	systemdTimerDir = t.TempDir()
	t.Cleanup(func() { systemdTimerDir = old })

	u := &Updater{systemdManager: &controllableSystemdManager{changes: []systemd1.UnitFileChange{{}}}}
	err := u.applyIdleDownloadConfig(idleDownloadConfig{
		IdleDownloadEnabled: false,
		BeginTime:           "10:00",
		EndTime:             "11:00",
	}, time.Now(), true)
	assert.NoError(t, err)
}

func TestDelUpdatePackageSuccess(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		userAgents:       newUserAgentMap(),
		jobManager:       newTestJobManager(),
		updateSourceOnce: true,
	}
	job, err := m.delUpdatePackage(ifcTestSender, "update", "pkg-a pkg-b")
	assert.NoError(t, err)
	assert.NotNil(t, job)
}

func TestRemovePackageSuccess(t *testing.T) {
	m := &Manager{
		service:    newRootConnService(t),
		userAgents: newUserAgentMap(),
		jobManager: newTestJobManager(),
	}
	job, err := m.removePackage(ifcTestSender, "remove", "pkg-a")
	assert.NoError(t, err)
	assert.NotNil(t, job)
}

func TestInstallPackageSuccess(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		userAgents:       newUserAgentMap(),
		jobManager:       newTestJobManager(),
		updateSourceOnce: true,
	}
	job, err := m.installPackage(ifcTestSender, "install", "pkg-a")
	assert.NoError(t, err)
	assert.NotNil(t, job)
}

func TestDelFixErrorSuccess(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		userAgents:       newUserAgentMap(),
		jobManager:       newTestJobManager(),
		updateSourceOnce: true,
	}
	job, err := m.delFixError(ifcTestSender, "dpkgInterrupted")
	assert.NoError(t, err)
	assert.NotNil(t, job)
}
