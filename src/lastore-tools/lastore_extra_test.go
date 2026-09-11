// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/go-ini/ini"
	"github.com/godbus/dbus/v5"
	lastore "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.lastore1"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/lastore-daemon/src/internal/dstore"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network disabled in test")
}

// newMockJob builds a lastore.Job mock whose Id/Type/Status/Progress/Description
// property getters return fixed values, enough to drive showLine and waitJob.
func newMockJob(status string, progress float64) *lastore.MockJob {
	j := &lastore.MockJob{}

	strProp := func(val string) *proxy.MockPropString {
		p := &proxy.MockPropString{}
		p.On("Get", dbus.Flags(0)).Return(val, nil)
		return p
	}

	j.MockInterfaceJob.On("Id").Return(strProp("job1"))
	j.MockInterfaceJob.On("Type").Return(strProp("install"))
	j.MockInterfaceJob.On("Status").Return(strProp(status))
	j.MockInterfaceJob.On("Description").Return(strProp("desc"))

	progressProp := &proxy.MockPropDouble{}
	progressProp.On("Get", dbus.Flags(0)).Return(progress, nil)
	j.MockInterfaceJob.On("Progress").Return(progressProp)

	return j
}

func TestFormatJobLine(t *testing.T) {
	got := formatJobLine("job1", "install", "running", 0.5, "desc")
	assert.Equal(t, "id:job1(install)\tProgress:running:50%\tDesc:\"desc\"", got)
}

func TestLastoreSearch(t *testing.T) {
	old := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: failingTransport{}}
	defer func() { http.DefaultClient = old }()

	assert.Error(t, LastoreSearch("", "", false))
}

func TestMainTester(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("job", "", "")
	app := &cli.App{
		Writer:   io.Discard,
		Commands: []cli.Command{CMDTester},
	}
	c := cli.NewContext(app, set, nil)
	assert.NoError(t, MainTester(c))
}

func TestShowLine(t *testing.T) {
	j := newMockJob("running", 0.5)
	assert.Equal(t, "id:job1(install)\tProgress:running:50%\tDesc:\"desc\"", showLine(j))
}

func TestWaitJobSystemBusError(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	systemBusFn = func() (*dbus.Conn, error) { return nil, errors.New("no bus") }

	assert.EqualError(t, waitJob("/org/deepin/dde/Lastore1/Job1"), "no bus")
}

func TestWaitJobNewJobError(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	systemBusFn = func() (*dbus.Conn, error) { return nil, nil }
	newJobFn = func(*dbus.Conn, dbus.ObjectPath) (lastore.Job, error) {
		return nil, errors.New("bad path")
	}

	assert.EqualError(t, waitJob("/org/deepin/dde/Lastore1/Job1"), "bad path")
}

func TestWaitJobEmptyStatus(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	systemBusFn = func() (*dbus.Conn, error) { return nil, nil }
	newJobFn = func(*dbus.Conn, dbus.ObjectPath) (lastore.Job, error) {
		return newMockJob("", 0), nil
	}

	assert.NoError(t, waitJob("/org/deepin/dde/Lastore1/Job1"))
}

func TestWaitJobSucceed(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	systemBusFn = func() (*dbus.Conn, error) { return nil, nil }
	newJobFn = func(*dbus.Conn, dbus.ObjectPath) (lastore.Job, error) {
		return newMockJob(string(system.SucceedStatus), 1), nil
	}

	assert.NoError(t, waitJob("/org/deepin/dde/Lastore1/Job1"))
}

func TestWaitJobFailed(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	systemBusFn = func() (*dbus.Conn, error) { return nil, nil }
	newJobFn = func(*dbus.Conn, dbus.ObjectPath) (lastore.Job, error) {
		return newMockJob(string(system.FailedStatus), 0), nil
	}

	assert.Error(t, waitJob("/org/deepin/dde/Lastore1/Job1"))
}

func TestWaitJobPaused(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	systemBusFn = func() (*dbus.Conn, error) { return nil, nil }
	newJobFn = func(*dbus.Conn, dbus.ObjectPath) (lastore.Job, error) {
		return newMockJob(string(system.PausedStatus), 0), nil
	}

	assert.EqualError(t, waitJob("/org/deepin/dde/Lastore1/Job1"), "job be paused")
}

func TestWaitJobRunningThenSucceed(t *testing.T) {
	oldBus, oldJob := systemBusFn, newJobFn
	t.Cleanup(func() { systemBusFn = oldBus; newJobFn = oldJob })

	// First status poll returns "running" (Ready/Running branch), then every
	// subsequent poll reports "succeed" so the loop terminates.
	j := &lastore.MockJob{}
	strProp := func(val string) *proxy.MockPropString {
		p := &proxy.MockPropString{}
		p.On("Get", dbus.Flags(0)).Return(val, nil)
		return p
	}
	j.MockInterfaceJob.On("Id").Return(strProp("job1"))
	j.MockInterfaceJob.On("Type").Return(strProp("install"))
	j.MockInterfaceJob.On("Description").Return(strProp("desc"))

	statusProp := &proxy.MockPropString{}
	statusProp.On("Get", dbus.Flags(0)).Return("running", nil).Once()
	statusProp.On("Get", dbus.Flags(0)).Return("succeed", nil)
	j.MockInterfaceJob.On("Status").Return(statusProp)

	progressProp := &proxy.MockPropDouble{}
	progressProp.On("Get", dbus.Flags(0)).Return(0.5, nil)
	j.MockInterfaceJob.On("Progress").Return(progressProp)

	systemBusFn = func() (*dbus.Conn, error) { return nil, nil }
	newJobFn = func(*dbus.Conn, dbus.ObjectPath) (lastore.Job, error) { return j, nil }

	assert.NoError(t, waitJob("/org/deepin/dde/Lastore1/Job1"))
}

func TestGetLastorePanicsOnBusError(t *testing.T) {
	old := systemBusFn
	t.Cleanup(func() { systemBusFn = old })

	systemBusFn = func() (*dbus.Conn, error) { return nil, errors.New("no bus") }

	assert.Panics(t, func() { getLastore() })
}

func TestGetLastoreSuccess(t *testing.T) {
	old := systemBusFn
	t.Cleanup(func() { systemBusFn = old })

	c1, c2 := net.Pipe()
	t.Cleanup(func() { _ = c1.Close(); _ = c2.Close() })
	systemBusFn = func() (*dbus.Conn, error) {
		return dbus.NewConn(c1)
	}

	assert.NotNil(t, getLastore())
}

func TestLastoreSearchHappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"dpk://deb/foo":{"name":"foo"},"dpk://deb/bar":{"name":"bar"}}`)
	}))
	defer server.Close()

	cfg := ini.Empty()
	_, err := cfg.Section("General").NewKey("Server", server.URL)
	require.NoError(t, err)

	oldStore, oldPath := lastoreNewStore, lastoreAppsPath
	t.Cleanup(func() {
		lastoreNewStore = oldStore
		lastoreAppsPath = oldPath
	})
	lastoreNewStore = func() *dstore.Store { return dstore.NewStoreWithConfig(cfg) }
	lastoreAppsPath = filepath.Join(t.TempDir(), "apps.json")

	// No filter: lists every package.
	require.NoError(t, LastoreSearch("", "", false))

	// Filter matching one package (debug branch).
	require.NoError(t, LastoreSearch("", "foo", true))

	// Filter matching nothing.
	require.NoError(t, LastoreSearch("", "nomatch", false))
}

func TestMainTesterSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"dpk://deb/foo":{"name":"foo"}}`)
	}))
	defer server.Close()

	cfg := ini.Empty()
	_, err := cfg.Section("General").NewKey("Server", server.URL)
	require.NoError(t, err)

	oldStore, oldPath := lastoreNewStore, lastoreAppsPath
	t.Cleanup(func() {
		lastoreNewStore = oldStore
		lastoreAppsPath = oldPath
	})
	lastoreNewStore = func() *dstore.Store { return dstore.NewStoreWithConfig(cfg) }
	lastoreAppsPath = filepath.Join(t.TempDir(), "apps.json")

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("job", "search", "")
	require.NoError(t, set.Parse([]string{"foo"}))
	app := &cli.App{
		Writer:   io.Discard,
		Commands: []cli.Command{CMDTester},
	}
	c := cli.NewContext(app, set, nil)

	assert.NoError(t, MainTester(c))
}
