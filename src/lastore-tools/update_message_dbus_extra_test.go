// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFakeJobStatusConn returns a *dbus.Conn whose peer is an in-memory fake bus
// daemon (net.Pipe). The fake daemon answers the auth/Hello handshake and, for
// the org.freedesktop.DBus.Properties.Get call issued by
// getUpdateJosStatusProperty, replies with either a string variant or a D-Bus
// error. No real system/session bus is touched.
func newFakeJobStatusConn(t *testing.T, status string, withError bool) *dbus.Conn {
	t.Helper()
	server, client := net.Pipe()
	conn, err := dbus.NewConn(server)
	require.NoError(t, err)

	go serveJobStatusFakeBus(client, status, withError)

	require.NoError(t, conn.Auth(nil))
	require.NoError(t, conn.Hello())

	t.Cleanup(func() {
		_ = conn.Close()
		_ = server.Close()
	})
	return conn
}

func readAuthLine(r *bufio.Reader) ([][]byte, error) {
	data, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSuffix(data, []byte("\r\n"))
	return bytes.Split(data, []byte{' '}), nil
}

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

func serveJobStatusFakeBus(c net.Conn, status string, withError bool) {
	defer c.Close()
	r := bufio.NewReader(c)

	// Auth handshake (server side).
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
		case "Get":
			if withError {
				reply = fakeBusError(serial)
			} else {
				reply = fakeBusReply(serial, []interface{}{dbus.MakeVariant(status)})
			}
		default:
			reply = fakeBusError(serial)
		}
		_ = reply.EncodeTo(c, binary.LittleEndian)
	}
}

func TestGetUpdateJosStatusPropertySuccess(t *testing.T) {
	conn := newFakeJobStatusConn(t, "succeed", false)
	assert.Equal(t, "succeed", getUpdateJosStatusProperty(conn, "/org/deepin/dde/Lastore1/Job1"))
}

func TestGetUpdateJosStatusPropertyError(t *testing.T) {
	conn := newFakeJobStatusConn(t, "", true)
	assert.Equal(t, "", getUpdateJosStatusProperty(conn, "/org/deepin/dde/Lastore1/Job1"))
}
