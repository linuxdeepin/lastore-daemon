// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFakeActiveStateConn returns a *dbus.Conn served by an in-memory fake bus
// that answers the org.freedesktop.DBus.Properties.Get call for ActiveState
// with the given string. It lets IsUnitActive reach its success branch without
// a real system bus.
func newFakeActiveStateConn(t *testing.T, activeState string) *dbus.Conn {
	t.Helper()
	server, client := net.Pipe()
	conn, err := dbus.NewConn(server)
	require.NoError(t, err)
	go serveActiveStateFakeBus(client, activeState, false)
	require.NoError(t, conn.Auth(nil))
	require.NoError(t, conn.Hello())
	t.Cleanup(func() {
		_ = conn.Close()
		_ = server.Close()
	})
	return conn
}

func readFakeBusAuthLine(r *bufio.Reader) ([][]byte, error) {
	data, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSuffix(data, []byte("\r\n"))
	return bytes.Split(data, []byte{' '}), nil
}

func fakeBusSerial(msg *dbus.Message) uint32 {
	return uint32(reflect.ValueOf(msg).Elem().FieldByName("serial").Uint())
}

func fakeBusMethodReply(serial uint32, body []interface{}) *dbus.Message {
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

func serveActiveStateFakeBus(c net.Conn, activeState string, withError bool) {
	defer c.Close()
	r := bufio.NewReader(c)

	var nul [1]byte
	if _, err := io.ReadFull(r, nul[:]); err != nil {
		return
	}
	line, err := readFakeBusAuthLine(r)
	if err != nil || len(line) < 1 || !bytes.Equal(line[0], []byte("AUTH")) {
		return
	}
	if _, err := io.WriteString(c, "REJECTED EXTERNAL\r\n"); err != nil {
		return
	}
	line, err = readFakeBusAuthLine(r)
	if err != nil || len(line) < 2 || !bytes.Equal(line[0], []byte("AUTH")) {
		return
	}
	if _, err := io.WriteString(c, "OK 0\r\n"); err != nil {
		return
	}
	line, err = readFakeBusAuthLine(r)
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
		serial := fakeBusSerial(msg)
		member, _ := msg.Headers[dbus.FieldMember].Value().(string)

		var reply *dbus.Message
		switch member {
		case "Hello":
			reply = fakeBusMethodReply(serial, []interface{}{":1.0"})
		case "Get":
			if withError {
				reply = &dbus.Message{
					Type: dbus.TypeError,
					Headers: map[dbus.HeaderField]dbus.Variant{
						dbus.FieldReplySerial: dbus.MakeVariant(serial),
						dbus.FieldErrorName:   dbus.MakeVariant("org.freedesktop.DBus.Error.Failed"),
					},
				}
			} else {
				reply = fakeBusMethodReply(serial, []interface{}{dbus.MakeVariant(activeState)})
			}
		default:
			reply = &dbus.Message{
				Type: dbus.TypeError,
				Headers: map[dbus.HeaderField]dbus.Variant{
					dbus.FieldReplySerial: dbus.MakeVariant(serial),
					dbus.FieldErrorName:   dbus.MakeVariant("org.freedesktop.DBus.Error.UnknownMethod"),
				},
			}
		}
		_ = reply.EncodeTo(c, binary.LittleEndian)
	}
}

func TestSystemdCheckerIsUnitActive_NewUnitError(t *testing.T) {
	mgr := &systemd1.MockManager{}
	mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "bad.service").
		Return(dbus.ObjectPath(""), nil)

	c := &SystemdChecker{manager: mgr}
	active, err := c.IsUnitActive("bad.service")
	assert.False(t, active)
	assert.Error(t, err)
}

func TestSystemdCheckerIsUnitActive_Active(t *testing.T) {
	conn := newFakeActiveStateConn(t, "active")
	mgr := &systemd1.MockManager{}
	mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "display-manager.service").
		Return(dbus.ObjectPath("/org/freedesktop/systemd1/unit/display_2dmanager_2eservice"), nil)

	c := &SystemdChecker{conn: conn, manager: mgr}
	active, err := c.IsUnitActive("display-manager.service")
	assert.NoError(t, err)
	assert.True(t, active)
}

func TestSystemdCheckerIsUnitActive_Inactive(t *testing.T) {
	conn := newFakeActiveStateConn(t, "inactive")
	mgr := &systemd1.MockManager{}
	mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "display-manager.service").
		Return(dbus.ObjectPath("/org/freedesktop/systemd1/unit/display_2dmanager_2eservice"), nil)

	c := &SystemdChecker{conn: conn, manager: mgr}
	active, err := c.IsUnitActive("display-manager.service")
	assert.NoError(t, err)
	assert.False(t, active)
}

func TestSystemdCheckerIsUnitActive_ActiveStateError(t *testing.T) {
	server, client := net.Pipe()
	conn, err := dbus.NewConn(server)
	require.NoError(t, err)
	go serveActiveStateFakeBus(client, "", true)
	require.NoError(t, conn.Auth(nil))
	require.NoError(t, conn.Hello())
	t.Cleanup(func() {
		_ = conn.Close()
		_ = server.Close()
	})

	mgr := &systemd1.MockManager{}
	mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "display-manager.service").
		Return(dbus.ObjectPath("/org/freedesktop/systemd1/unit/display_2dmanager_2eservice"), nil)

	c := &SystemdChecker{conn: conn, manager: mgr}
	active, err := c.IsUnitActive("display-manager.service")
	assert.False(t, active)
	assert.Error(t, err)
}

func TestCheckImportantServiceActive(t *testing.T) {
	conn := newFakeActiveStateConn(t, "active")
	old := newSystemdCheckerFn
	t.Cleanup(func() { newSystemdCheckerFn = old })
	newSystemdCheckerFn = func() (*SystemdChecker, error) {
		mgr := &systemd1.MockManager{}
		mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "display-manager.service").
			Return(dbus.ObjectPath("/org/freedesktop/systemd1/unit/display_2dmanager_2eservice"), nil)
		return &SystemdChecker{conn: conn, manager: mgr}, nil
	}

	assert.NoError(t, CheckImportantService(Stage1))
}

func TestCheckImportantServiceInactive(t *testing.T) {
	conn := newFakeActiveStateConn(t, "inactive")
	old := newSystemdCheckerFn
	t.Cleanup(func() { newSystemdCheckerFn = old })
	newSystemdCheckerFn = func() (*SystemdChecker, error) {
		mgr := &systemd1.MockManager{}
		mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "display-manager.service").
			Return(dbus.ObjectPath("/org/freedesktop/systemd1/unit/display_2dmanager_2eservice"), nil)
		return &SystemdChecker{conn: conn, manager: mgr}, nil
	}

	err := CheckImportantService(Stage1)
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckServiceFailed, jobErr.ErrType)
}

func TestCheckImportantServiceCheckerError(t *testing.T) {
	old := newSystemdCheckerFn
	t.Cleanup(func() { newSystemdCheckerFn = old })
	newSystemdCheckerFn = func() (*SystemdChecker, error) {
		return nil, errors.New("no system bus")
	}

	err := CheckImportantService(Stage1)
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckServiceFailed, jobErr.ErrType)
}

func TestCheckImportantServiceUnitCheckError(t *testing.T) {
	server, client := net.Pipe()
	conn, err := dbus.NewConn(server)
	require.NoError(t, err)
	go serveActiveStateFakeBus(client, "", true)
	require.NoError(t, conn.Auth(nil))
	require.NoError(t, conn.Hello())
	t.Cleanup(func() {
		_ = conn.Close()
		_ = server.Close()
	})

	old := newSystemdCheckerFn
	t.Cleanup(func() { newSystemdCheckerFn = old })
	newSystemdCheckerFn = func() (*SystemdChecker, error) {
		mgr := &systemd1.MockManager{}
		mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "display-manager.service").
			Return(dbus.ObjectPath("/org/freedesktop/systemd1/unit/display_2dmanager_2eservice"), nil)
		return &SystemdChecker{conn: conn, manager: mgr}, nil
	}

	err = CheckImportantService(Stage1)
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckServiceFailed, jobErr.ErrType)
}
