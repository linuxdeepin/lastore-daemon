// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
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

// --- fake in-memory D-Bus daemon --------------------------------------
//
// newRootConnService returns a dbusutil.Service whose connection is served by a
// fake bus daemon over a net.Pipe. The fake daemon reports every connection as
// uid 0 (root), so Manager.checkInvokePermission treats the caller as trusted
// and skips polkit — letting tests reach post-permission branches without a
// real bus or a polkit dialog. Unknown method calls get a D-Bus error so
// callers fail fast instead of blocking.

func newRootConnService(t *testing.T) *dbusutil.Service {
	t.Helper()
	server, client := net.Pipe()
	conn, err := dbus.NewConn(server)
	require.NoError(t, err)

	stop := make(chan struct{})
	go serveFakeBus(client, stop)

	require.NoError(t, conn.Auth(nil))
	require.NoError(t, conn.Hello())

	t.Cleanup(func() {
		close(stop)
		_ = conn.Close()
		_ = server.Close()
	})
	return dbusutil.NewService(conn)
}

// newRootManager returns a Manager whose checkInvokePermission passes for any
// sender (fake bus reports uid 0).
func newRootManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{service: newRootConnService(t)}
}

func readAuthLine(r *bufio.Reader) ([][]byte, error) {
	data, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSuffix(data, []byte("\r\n"))
	return bytes.Split(data, []byte{' '}), nil
}

// fakeBusMessageSerial reads the unexported serial field of a decoded message.
func fakeBusMessageSerial(msg *dbus.Message) uint32 {
	return uint32(reflect.ValueOf(msg).Elem().FieldByName("serial").Uint())
}

func fakeBusReply(serial uint32, body []interface{}) *dbus.Message {
	msg := &dbus.Message{
		Type: dbus.TypeMethodReply,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(serial),
		},
		Body: body,
	}
	if len(body) > 0 {
		msg.Headers[dbus.FieldSignature] = dbus.MakeVariant(dbus.SignatureOf(body...))
	}
	return msg
}

func fakeBusError(serial uint32) *dbus.Message {
	return &dbus.Message{
		Type: dbus.TypeError,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(serial),
			dbus.FieldErrorName:   dbus.MakeVariant("org.freedesktop.DBus.Error.UnknownMethod"),
		},
	}
}

func serveFakeBus(c net.Conn, stop <-chan struct{}) {
	defer c.Close()
	r := bufio.NewReader(c)

	// Auth handshake (server side): the client sends
	//   NUL "AUTH" -> "REJECTED EXTERNAL"
	//   "AUTH EXTERNAL <hexuid>" -> "OK <hexuid>"
	//   "BEGIN"
	var nul [1]byte
	if _, err := io.ReadFull(r, nul[:]); err != nil {
		return
	}
	line, err := readAuthLine(r)
	if err != nil || len(line) < 1 || !bytes.Equal(line[0], []byte("AUTH")) {
		return
	}
	if _, err := io.WriteString(c, "REJECTED EXTERNAL\r\n"); err != nil {
		return
	}
	line, err = readAuthLine(r)
	if err != nil || len(line) < 2 || !bytes.Equal(line[0], []byte("AUTH")) {
		return
	}
	// go-dbus discards the EXTERNAL mechanism's hex uid, so the client sends a
	// bare "AUTH EXTERNAL" line. Echo a fixed uuid; the client only stores it.
	if _, err := io.WriteString(c, "OK 0\r\n"); err != nil {
		return
	}
	line, err = readAuthLine(r)
	if err != nil || len(line) < 1 || !bytes.Equal(line[0], []byte("BEGIN")) {
		return
	}

	for {
		msg, err := dbus.DecodeMessage(r)
		if err != nil {
			return
		}
		if msg.Type != dbus.TypeMethodCall {
			continue
		}
		serial := fakeBusMessageSerial(msg)
		member, _ := msg.Headers[dbus.FieldMember].Value().(string)

		var reply *dbus.Message
		switch member {
		case "Hello":
			reply = fakeBusReply(serial, []interface{}{":1.0"})
		case "GetConnectionUnixUser":
			reply = fakeBusReply(serial, []interface{}{uint32(0)})
		case "GetConnectionUnixProcessID":
			reply = fakeBusReply(serial, []interface{}{uint32(os.Getpid())})
		case "NameHasOwner":
			reply = fakeBusReply(serial, []interface{}{false})
		case "GetNameOwner":
			reply = fakeBusReply(serial, []interface{}{""})
		default:
			reply = fakeBusError(serial)
		}
		_ = reply.EncodeTo(c, binary.LittleEndian)
	}
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

// --- post-permission branches (fake bus reports uid 0 -> trusted) ----------

func TestManagerIfcCleanArchives(t *testing.T) {
	m := &Manager{
		service:    newRootConnService(t),
		jobManager: NewJobManager(newFailingConnService(t), apt.NewSystem(nil, nil, false), nil, nil),
		config:     newTestConfig(t),
	}
	job, busErr := m.CleanArchives(ifcTestSender)
	assert.Nil(t, busErr)
	assert.NotEqual(t, dbus.ObjectPath("/"), job)
}

func TestManagerIfcFixErrorInvalidType(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		userAgents:       newUserAgentMap(),
		updateSourceOnce: true,
	}
	job, busErr := m.FixError(ifcTestSender, "not-a-real-error")
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcFixErrorValidType(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		userAgents:       newUserAgentMap(),
		updateSourceOnce: true,
		jobManager:       NewJobManager(newFailingConnService(t), apt.NewSystem(nil, nil, false), nil, nil),
	}
	job, busErr := m.FixError(ifcTestSender, string(system.ErrorDpkgInterrupted))
	assert.Nil(t, busErr)
	assert.NotEqual(t, dbus.ObjectPath("/"), job)
}

// writeJSONBin writes an executable shell script that prints the given JSON to
// stdout and returns its path. It stubs lastore-apt-clean so GetArchivesInfo
// can be exercised without the real binary installed.
func writeJSONBin(t *testing.T, json string) string {
	t.Helper()
	script := "#!/bin/sh\ncat <<'EOF'\n" + json + "\nEOF\n"
	path := filepath.Join(t.TempDir(), "lastore-apt-clean")
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path
}

func TestManagerIfcGetArchivesInfo(t *testing.T) {
	oldClean := apt.LastoreAptCleanBinPath
	t.Cleanup(func() { apt.LastoreAptCleanBinPath = oldClean })
	apt.LastoreAptCleanBinPath = writeJSONBin(t, `{"total": "1024"}`)

	m := &Manager{service: newRootConnService(t)}
	info, busErr := m.GetArchivesInfo(ifcTestSender)
	assert.Nil(t, busErr)
	assert.NotEmpty(t, info)
}

func TestManagerIfcInstallPackageInvalidPkgs(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	job, busErr := m.InstallPackage(ifcTestSender, "job", "InvalidPackage!")
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcInstallPackageFromRepoInvalidPath(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	job, busErr := m.InstallPackageFromRepo(ifcTestSender, "job", "/src", "/nonexistent-repo", "/cache", []string{"pkg"})
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcGetUpdateLogsUnknownType(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	logs, busErr := m.GetUpdateLogs(ifcTestSender, 0)
	assert.Equal(t, "", logs)
	assert.NotNil(t, busErr)
}

func TestManagerIfcGetUpdateLogsSystem(t *testing.T) {
	m := &Manager{
		service:        newRootConnService(t),
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	logs, busErr := m.GetUpdateLogs(ifcTestSender, system.SystemUpdate)
	assert.Nil(t, busErr)
	assert.NotEmpty(t, logs)
}

func TestManagerIfcGetHistoryLogs(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	logs, busErr := m.GetHistoryLogs(ifcTestSender)
	assert.Nil(t, busErr)
	// upgradeRecordPath may not exist on the host; the branch still runs.
	_ = logs
}

func TestManagerIfcPackagesSizeEmpty(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		updateSourceOnce: true,
		UpdateMode:       0,
	}
	size, busErr := m.PackagesSize(ifcTestSender, nil)
	assert.NotNil(t, busErr)
	_ = size
}

func TestManagerIfcPackagesDownloadSizeEmpty(t *testing.T) {
	m := &Manager{
		service:          newRootConnService(t),
		updateSourceOnce: true,
		UpdateMode:       0,
		config:           newTestConfig(t),
	}
	size, busErr := m.PackagesDownloadSize(ifcTestSender, nil)
	assert.NotNil(t, busErr)
	_ = size
}

func TestManagerIfcPauseJob(t *testing.T) {
	m := &Manager{service: newRootConnService(t), jobManager: newTestJobManager()}
	assert.NotNil(t, m.PauseJob(ifcTestSender, "nonexistent"))
}

func TestManagerIfcStartJob(t *testing.T) {
	m := &Manager{service: newRootConnService(t), jobManager: newTestJobManager()}
	assert.NotNil(t, m.StartJob(ifcTestSender, "nonexistent"))
}

func TestManagerIfcRemovePackageInvalidPkgs(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	job, busErr := m.RemovePackage(ifcTestSender, "job", "InvalidPackage!")
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcSetAutoClean(t *testing.T) {
	m := &Manager{
		service:   newRootConnService(t),
		config:    newTestConfig(t),
		AutoClean: false,
	}
	assert.Nil(t, m.SetAutoClean(ifcTestSender, true))
	assert.True(t, m.AutoClean)
}

func TestManagerIfcPrepareFullScreenUpgradeInvalidJSON(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	assert.NotNil(t, m.PrepareFullScreenUpgrade(ifcTestSender, "not-json"))
}

func TestManagerIfcQueryAllSizeWithSourceEmptyMode(t *testing.T) {
	m := &Manager{service: newRootConnService(t)}
	size, busErr := m.QueryAllSizeWithSource(ifcTestSender, 0)
	assert.NotNil(t, busErr)
	_ = size
}

func TestManagerIfcPrepareDistUpgradePartlyImmutable(t *testing.T) {
	m := &Manager{service: newRootConnService(t), ImmutableAutoRecovery: true}
	job, busErr := m.PrepareDistUpgradePartly(ifcTestSender, system.SystemUpdate)
	assert.Equal(t, dbus.ObjectPath("/"), job)
	assert.NotNil(t, busErr)
}

func TestManagerIfcSetUpdateSourcesInvalidRepoType(t *testing.T) {
	m := &Manager{service: newRootConnService(t), config: newTestConfig(t)}
	assert.NotNil(t, m.SetUpdateSources(ifcTestSender, system.SystemUpdate, config.RepoType("bogus"), nil, false))
}

func TestManagerIfcSetUpdateSourcesCustomEmpty(t *testing.T) {
	m := &Manager{service: newRootConnService(t), config: newTestConfig(t)}
	assert.NotNil(t, m.SetUpdateSources(ifcTestSender, system.SystemUpdate, config.CustomRepo, []string{}, false))
}

func TestManagerIfcSetUpdateSourcesUnsupportedType(t *testing.T) {
	m := &Manager{service: newRootConnService(t), config: newTestConfig(t)}
	assert.NotNil(t, m.SetUpdateSources(ifcTestSender, system.UpdateType(999), config.OSDefaultRepo, nil, false))
}

func TestManagerIfcConfirmRollbackTrue(t *testing.T) {
	installFakeImmutableCtl(t)
	m := &Manager{
		service:          newRootConnService(t),
		immutableManager: newImmutableManager(func(info system.JobProgressInfo) {}),
	}
	assert.Nil(t, m.ConfirmRollback(ifcTestSender, true))
}

func TestManagerIfcCanRollback(t *testing.T) {
	installFakeImmutableCtl(t)
	m := &Manager{
		service:          newRootConnService(t),
		immutableManager: newImmutableManager(func(info system.JobProgressInfo) {}),
	}
	can, info, busErr := m.CanRollback(ifcTestSender)
	assert.True(t, can)
	assert.NotEmpty(t, info)
	assert.Nil(t, busErr)
}

func TestManagerIfcGetUpdateDetailsRealtime(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "fd-*")
	require.NoError(t, err)
	defer tmp.Close()

	m := &Manager{service: newRootConnService(t)}
	assert.Nil(t, m.GetUpdateDetails(ifcTestSender, dbus.UnixFD(tmp.Fd()), true))
	assert.Len(t, m.logFds, 1)
}

func TestManagerIfcGetUpdateDetailsNotRealtime(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "fd-*")
	require.NoError(t, err)
	defer tmp.Close()

	m := &Manager{service: newRootConnService(t)}
	assert.NotNil(t, m.GetUpdateDetails(ifcTestSender, dbus.UnixFD(tmp.Fd()), false))
}
