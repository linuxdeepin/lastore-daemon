// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
)

func TestRunNewSystemServiceError(t *testing.T) {
	old := newSystemServiceFn
	newSystemServiceFn = func() (*dbusutil.Service, error) {
		return nil, errors.New("no system bus")
	}
	t.Cleanup(func() { newSystemServiceFn = old })

	// run() prints the error and returns without panicking or blocking.
	run(false)
}

func TestRunSessionBus(t *testing.T) {
	old := newSystemServiceFn
	newSystemServiceFn = func() (*dbusutil.Service, error) {
		return dbusutil.NewSessionService()
	}
	t.Cleanup(func() { newSystemServiceFn = old })

	// Point newSmartMirror at an empty temp dir instead of the real state dir.
	oldState := stateDirectory
	stateDirectory = t.TempDir()
	t.Cleanup(func() { stateDirectory = oldState })

	// Non-empty value exercises the DBUS_STARTER_BUS_TYPE PATH adjustment.
	t.Setenv("DBUS_STARTER_BUS_TYPE", "session")

	// run() reaches service.Wait() and blocks there; run it in a goroutine and
	// only wait long enough to exercise the lines before the block.
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(false)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		// Blocked in service.Wait(); expected.
	}
}
