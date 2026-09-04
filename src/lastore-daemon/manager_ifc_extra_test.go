// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"net"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFailingConnService returns a dbusutil.Service backed by a connection whose
// peer has already been closed. Every bus call (e.g. GetConnUID) therefore
// fails fast with an io error, which lets permission-gated Manager methods hit
// their "permission denied" early-return path without a real bus or polkit.
func newFailingConnService(t *testing.T) *dbusutil.Service {
	t.Helper()
	server, client := net.Pipe()
	conn, err := dbus.NewConn(server)
	require.NoError(t, err)
	require.NoError(t, client.Close())
	t.Cleanup(func() {
		_ = conn.Close()
		_ = server.Close()
	})
	return dbusutil.NewService(conn)
}

func newPermissionGatedManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{service: newFailingConnService(t)}
}

const ifcTestSender = dbus.Sender(":1.42")

func TestManagerIfcCleanArchivesPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.CleanArchives(ifcTestSender)
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcCleanJobPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.CleanJob(ifcTestSender, "job1"))
}

func TestManagerIfcFixErrorPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.FixError(ifcTestSender, "dpkgError")
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcGetArchivesInfoPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	info, busErr := m.GetArchivesInfo(ifcTestSender)
	assert.Equal(t, "", info)
	assert.NotNil(t, busErr)
}

func TestManagerIfcHandleSystemEventGetConnUIDFails(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.HandleSystemEvent(ifcTestSender, "AutoCheck"))
}

func TestManagerIfcInstallPackagePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.InstallPackage(ifcTestSender, "job", "foo bar")
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcInstallPackageFromRepoPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.InstallPackageFromRepo(ifcTestSender, "job", "/src", "/repo", "/cache", []string{"foo"})
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPackageExistsPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	exist, busErr := m.PackageExists(ifcTestSender, "foo")
	assert.False(t, exist)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPackageInstallablePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	installable, busErr := m.PackageInstallable(ifcTestSender, "foo")
	assert.False(t, installable)
	assert.NotNil(t, busErr)
}

func TestManagerIfcGetUpdateLogsPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	logs, busErr := m.GetUpdateLogs(ifcTestSender, system.SystemUpdate)
	assert.Equal(t, "", logs)
	assert.NotNil(t, busErr)
}

func TestManagerIfcGetHistoryLogsPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	logs, busErr := m.GetHistoryLogs(ifcTestSender)
	assert.Equal(t, "", logs)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPackagesSizePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	size, busErr := m.PackagesSize(ifcTestSender, []string{"foo"})
	assert.Equal(t, int64(0), size)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPackagesDownloadSizePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	size, busErr := m.PackagesDownloadSize(ifcTestSender, []string{"foo"})
	assert.Equal(t, int64(0), size)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPauseJobPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.PauseJob(ifcTestSender, "job1"))
}

func TestManagerIfcRegisterAgentGetConnUIDFails(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.RegisterAgent(ifcTestSender, "/org/deepin/dde/Lastore1/Agent1"))
}

func TestManagerIfcRemovePackagePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.RemovePackage(ifcTestSender, "job", "foo")
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcSetAutoCleanPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.SetAutoClean(ifcTestSender, true))
}

func TestManagerIfcStartJobPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.StartJob(ifcTestSender, "job1"))
}

func TestManagerIfcUnRegisterAgentGetConnUIDFails(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.UnRegisterAgent(ifcTestSender, "/org/deepin/dde/Lastore1/Agent1"))
}

func TestManagerIfcUpdateSourcePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.UpdateSource(ifcTestSender)
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcDistUpgradePartlyPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.DistUpgradePartly(ifcTestSender, system.SystemUpdate, false)
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPrepareFullScreenUpgradePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.PrepareFullScreenUpgrade(ifcTestSender, `{"DoUpgrade":true}`))
}

func TestManagerIfcQueryAllSizeWithSourcePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	size, busErr := m.QueryAllSizeWithSource(ifcTestSender, system.SystemUpdate)
	assert.Equal(t, int64(0), size)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPrepareDistUpgradePartlyPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.PrepareDistUpgradePartly(ifcTestSender, system.SystemUpdate)
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcCheckUpgradePermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	job, busErr := m.CheckUpgrade(ifcTestSender, system.SystemUpdate, uint32(firstCheck))
	// CheckUpgrade returns empty string (not "/") on the permission-denied path.
	assert.Equal(t, dbus.ObjectPath(""), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcPowerOffPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.PowerOff(ifcTestSender, false))
}

func TestManagerIfcSetUpdateSourcesPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.SetUpdateSources(ifcTestSender, system.SystemUpdate, config.OSDefaultRepo, nil, false))
}

func TestManagerIfcConfirmRollbackPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.ConfirmRollback(ifcTestSender, true))
}

func TestManagerIfcCanRollbackPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	can, info, busErr := m.CanRollback(ifcTestSender)
	assert.False(t, can)
	assert.Equal(t, "", info)
	assert.NotNil(t, busErr)
}

func TestManagerIfcGetUpdateDetailsPermissionDenied(t *testing.T) {
	m := newPermissionGatedManager(t)
	assert.NotNil(t, m.GetUpdateDetails(ifcTestSender, dbus.UnixFD(1), false))
}
