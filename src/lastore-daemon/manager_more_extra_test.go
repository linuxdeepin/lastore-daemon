// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	lastoreAgent "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.lastore1.agent"
	power "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMainEarlyExit covers main(). It only runs when the host lastore-daemon
// already owns org.deepin.dde.Lastore1, so main() hits the
// "another lastore-daemon running" early return instead of grabbing the name.
func TestMainEarlyExit(t *testing.T) {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		t.Skipf("system bus not available: %v", err)
	}

	hasOwner, err := service.NameHasOwner(dbusServiceName)
	require.NoError(t, err)
	if !hasOwner {
		t.Skip("host lastore-daemon not running; skipping to avoid grabbing the bus name")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("main() did not return; it unexpectedly grabbed the bus name")
	}
}

func TestInitLastoreInhibitHint(t *testing.T) {
	initLastoreInhibitHint(newFailingConnService(t))
}

// TestNewManagerArchDetectionFailure covers NewManager's arch-detection error
// path without triggering the full daemon init (which would add real bus
// matches and recover real agents).
func TestNewManagerArchDetectionFailure(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	m := NewManager(newFailingConnService(t), apt.NewSystem(nil, nil, false), newTestConfig(t))
	assert.Nil(t, m)
}

func TestInitDbusSignalListen(t *testing.T) {
	conn, err := dbus.SystemBus()
	if err != nil {
		t.Skipf("system bus not available: %v", err)
	}

	m := &Manager{
		loginManager:  login1.NewManager(conn),
		sysDBusDaemon: ofdbus.NewDBus(conn),
		sysPower:      power.NewPower(conn),
		signalLoop:    dbusutil.NewSignalLoop(conn, 10),
		userAgents:    newUserAgentMap(),
	}
	m.initDbusSignalListen()
}

func TestSyncHardwareRelatedData(t *testing.T) {
	m := &Manager{service: newTestService(), config: newTestConfig(t)}
	m.syncHardwareRelatedData()
}

func TestInitDSettingsChangedHandle(t *testing.T) {
	m := &Manager{config: newTestConfig(t)}
	m.initDSettingsChangedHandle()
}

func TestUpdateIncrementalUpdate(t *testing.T) {
	m := &Manager{
		updateApi:  apt.NewSystem(nil, nil, false),
		jobManager: newTestJobManager(),
	}
	m.UpdateIncrementalUpdate(true)
}

func TestInitStatusManager(t *testing.T) {
	// InitModifyData fires updateModeChangedCallback/checkModeChangedCallback
	// which call setPropUpdateMode/setPropCheckUpdateMode -> emitPropChanged* ->
	// v.service.EmitPropertyChanged, so service must be non-nil.
	m := &Manager{config: newTestConfig(t), service: newFailingConnService(t)}
	m.initStatusManager()
	assert.NotNil(t, m.statusManager)
}

func TestInitAgent(t *testing.T) {
	svc := newFailingConnService(t)
	m := &Manager{service: svc, loginManager: login1.NewManager(svc.Conn())}
	m.initAgent()
	assert.NotNil(t, m.userAgents)
}

func TestInitPlatformManager(t *testing.T) {
	m := &Manager{config: newTestConfig(t)}
	m.initPlatformManager()
	assert.NotNil(t, m.updatePlatform)
}

func TestTryToStartAutoCheck(t *testing.T) {
	// ImmutableAutoRecovery makes updateAutoCheckSystemUnit take the
	// stopTimerUnit path (failing conn -> error) instead of exec'ing
	// systemd-run, which avoids creating a transient timer.
	m := &Manager{
		config:                newTestConfig(t),
		systemd:               systemd1.NewManager(newFailingConnService(t).Conn()),
		ImmutableAutoRecovery: true,
	}
	m.TryToStartAutoCheck()
}

func TestDelUpdatePackageMakeEnvironError(t *testing.T) {
	m := &Manager{service: newFailingConnService(t), userAgents: newUserAgentMap(), updateSourceOnce: true}
	job, err := m.delUpdatePackage(ifcTestSender, "test", "pkg")
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestInstallPackageMakeEnvironError(t *testing.T) {
	m := &Manager{service: newFailingConnService(t), userAgents: newUserAgentMap(), updateSourceOnce: true}
	job, err := m.installPackage(ifcTestSender, "test", "pkg")
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestDelInstallPackageFromRepoInvalid(t *testing.T) {
	m := &Manager{}
	job, err := m.delInstallPackageFromRepo(ifcTestSender, "test", "/src", "/nonexistent-repo", "/cache", []string{"pkg"})
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestDelInstallPackageFromRepoInvalidCachePath(t *testing.T) {
	m := &Manager{}
	job, err := m.delInstallPackageFromRepo(ifcTestSender, "test", "/src", t.TempDir(), "/nonexistent-cache", []string{"pkg"})
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestDelInstallPackageFromRepoMakeEnvironError(t *testing.T) {
	m := &Manager{
		service:    newFailingConnService(t),
		userAgents: newUserAgentMap(),
	}
	job, err := m.delInstallPackageFromRepo(ifcTestSender, "test", "/src", t.TempDir(), t.TempDir(), []string{"pkg"})
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestDelInstallPackageFromRepoCreateJob(t *testing.T) {
	// As a non-root user, system.CheckLock cannot open /var/lib/dpkg/lock and
	// returns ("", false), so the function deterministically proceeds to create
	// and queue an OnlyInstall job. NotUseDBus keeps addJob from running apt.
	srcDir := t.TempDir()
	repoDir := t.TempDir()
	cacheDir := t.TempDir()
	m := &Manager{
		service:    newRootConnService(t),
		userAgents: newUserAgentMap(),
		jobManager: newTestJobManager(),
	}
	job, err := m.delInstallPackageFromRepo(ifcTestSender, "test", srcDir, repoDir, cacheDir, []string{"pkg"})
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "/dev/null", job.option["Dir::Etc::SourceList"])
	assert.Equal(t, srcDir, job.option["Dir::Etc::SourceParts"])
	assert.Equal(t, repoDir, job.option["Dir::State::lists"])
	assert.Equal(t, cacheDir, job.option["Dir::Cache::archives"])
}

func TestInstallPkg(t *testing.T) {
	m := &Manager{
		jobManager: NewJobManager(newFailingConnService(t), apt.NewSystem(nil, nil, false), nil, nil),
	}
	job, err := m.installPkg("test", "pkg-a", map[string]string{})
	assert.NotNil(t, job)
	assert.NoError(t, err)
}

func TestRemovePackageMakeEnvironError(t *testing.T) {
	m := &Manager{service: newFailingConnService(t), userAgents: newUserAgentMap()}
	job, err := m.removePackage(ifcTestSender, "test", "pkg")
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestCleanJobNotFound(t *testing.T) {
	m := &Manager{jobManager: newTestJobManager()}
	assert.Error(t, m.cleanJob("nonexistent"))
}

func TestCleanArchives(t *testing.T) {
	m := &Manager{
		jobManager: NewJobManager(newFailingConnService(t), apt.NewSystem(nil, nil, false), nil, nil),
		config:     newTestConfig(t),
	}
	job, err := m.cleanArchives(true)
	assert.NotNil(t, job)
	assert.NoError(t, err)
}

func TestDelFixErrorMakeEnvironError(t *testing.T) {
	m := &Manager{service: newFailingConnService(t), userAgents: newUserAgentMap(), updateSourceOnce: true}
	job, err := m.delFixError(ifcTestSender, "dpkg-interrupted")
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestUpdateModeWriteCallbackPermissionDenied(t *testing.T) {
	m := &Manager{service: newFailingConnService(t)}
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          uint64(0),
	}
	assert.NotNil(t, m.updateModeWriteCallback(pw))
}

func TestSyncThirdPartyDconfig(t *testing.T) {
	m := &Manager{service: newFailingConnService(t)}
	m.syncThirdPartyDconfig()
}

func TestCheckUpdateModeWriteCallbackPermissionDenied(t *testing.T) {
	m := &Manager{service: newFailingConnService(t)}
	pw := &dbusutil.PropertyWrite{
		PropertyAccess: dbusutil.PropertyAccess{Sender: ifcTestSender},
		Value:          uint64(0),
	}
	assert.NotNil(t, m.checkUpdateModeWriteCallback(pw))
}

func TestHandleAutoCheckRegularlyEvent(t *testing.T) {
	conn, err := dbus.SystemBus()
	if err != nil {
		t.Skipf("system bus not available: %v", err)
	}

	sm := newTestStatusManager(t)
	// Force every status to NotDownload so GetCanDistUpgradeMode returns 0 and
	// distUpgradePartly bails out before starting any real upgrade.
	for _, typ := range system.AllInstallUpdateType() {
		sm.updateModeStatusObj[typ.JobType()] = system.NotDownload
	}

	m := &Manager{
		service:        dbusutil.NewService(conn),
		statusManager:  sm,
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	_ = m.handleAutoCheckRegularlyEvent()
}

func TestWatchSession(t *testing.T) {
	conn, err := dbus.SystemBus()
	if err != nil {
		t.Skipf("system bus not available: %v", err)
	}

	sess, err := login1.NewSession(conn, "/org/freedesktop/login1/session/_31")
	require.NoError(t, err)

	m := &Manager{
		service:    dbusutil.NewService(conn),
		signalLoop: dbusutil.NewSignalLoop(conn, 10),
		userAgents: newUserAgentMap(),
	}
	m.watchSession("1000", sess)
}

func TestHandleSessionNew(t *testing.T) {
	m := &Manager{service: newFailingConnService(t)}
	m.handleSessionNew("_31", "/org/freedesktop/login1/session/_31")
}

func TestHandleSessionRemoved(t *testing.T) {
	m := &Manager{userAgents: newUserAgentMap()}
	m.handleSessionRemoved("_31", "/org/freedesktop/login1/session/_31")
}

func TestUpdateLocaleByUser(t *testing.T) {
	m := &Manager{service: newFailingConnService(t)}
	m.updateLocaleByUser("1000")
}

func TestHandleUserRemoved(t *testing.T) {
	m := &Manager{userAgents: newUserAgentMap()}
	m.handleUserRemoved(1000, "/org/freedesktop/Accounts/User1000")
}

func TestCloseNotify(t *testing.T) {
	m := &Manager{
		userAgents:   newUserAgentMap(),
		loginManager: login1.NewManager(newFailingConnService(t).Conn()),
	}
	assert.NoError(t, m.closeNotify(1))
}

func newMockLastoreAgent() *lastoreAgent.MockAgent {
	a := &lastoreAgent.MockAgent{}
	a.MockObject.On("Path_").Return(dbus.ObjectPath(lastoreAgentPath))
	return a
}

func newUserAgentMapWithAgent(a lastoreAgent.Agent) *userAgentMap {
	m := newUserAgentMap()
	m.setActiveUID("1000")
	m.addAgent("1000", a)
	return m
}

func TestSendNotifyDisabled(t *testing.T) {
	m := &Manager{
		updater: &Updater{UpdateNotify: false},
		config:  newTestConfig(t),
	}
	assert.Equal(t, uint32(0), m.sendNotify("app", 0, "icon", "summary", "body", nil, nil, -1))
}

func TestSendNotifyAgentSuccess(t *testing.T) {
	agent := newMockLastoreAgent()
	agent.MockInterfaceAgent.On("SendNotify",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint32(42), nil)

	m := &Manager{
		updater:    &Updater{UpdateNotify: true},
		config:     newTestConfig(t),
		userAgents: newUserAgentMapWithAgent(agent),
	}
	assert.Equal(t, uint32(42), m.sendNotify("app", 0, "icon", "summary", "body", nil, nil, -1))
}

func TestSendNotifyAgentError(t *testing.T) {
	agent := newMockLastoreAgent()
	agent.MockInterfaceAgent.On("SendNotify",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint32(0), errors.New("notify failed"))

	m := &Manager{
		updater:    &Updater{UpdateNotify: true},
		config:     newTestConfig(t),
		userAgents: newUserAgentMapWithAgent(agent),
	}
	assert.Equal(t, uint32(0), m.sendNotify("app", 0, "icon", "summary", "body", nil, nil, -1))
}

func TestChangePrepareDistUpgradeJobOption(t *testing.T) {
	m := &Manager{jobManager: newTestJobManager()}
	m.ChangePrepareDistUpgradeJobOption()
}

func TestAfterUpdateModeChanged(t *testing.T) {
	svc := newFailingConnService(t)
	m := &Manager{service: svc, updater: &Updater{service: svc}}
	m.afterUpdateModeChanged(nil)
}

func TestHandleDownloadLimitChanged(t *testing.T) {
	m := &Manager{updater: &Updater{}}
	m.handleDownloadLimitChanged(&Job{})
}

func TestInstallSpecialPackageSync(t *testing.T) {
	// The condition (updatable or installable) must stay false to avoid the
	// wg.Wait() deadlock, since preHooks never fire without a running job.
	m := &Manager{updater: &Updater{}}
	m.installSpecialPackageSync("zzz-nonexistent-pkg-123", nil, nil)
}

func TestReloadOemConfig(t *testing.T) {
	m := &Manager{
		config:               newTestConfig(t),
		systemSourceConfig:   make(UpdateSourceConfig),
		securitySourceConfig: make(UpdateSourceConfig),
	}
	m.reloadOemConfig(false)
}

func TestUpdateAutoRecoveryStatus(t *testing.T) {
	// /run/deepin-immutable-writable/booted does not exist on this host, so the
	// function returns early before touching any fields.
	m := &Manager{}
	m.updateAutoRecoveryStatus()
}
