// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

func TestSharedInhibitAcquireAndRelease(t *testing.T) {
	originalFn := inhibitorFn
	originalCloseFn := closeInhibitFd
	defer func() {
		inhibitorFn = originalFn
		closeInhibitFd = originalCloseFn
		sharedInhibitRef = 0
		sharedInhibitFd = -1
	}()

	var closedFds []int
	inhibitorFn = func(what, who, why string) (dbus.UnixFD, error) {
		return dbus.UnixFD(42), nil
	}
	closeInhibitFd = func(fd dbus.UnixFD) error {
		closedFds = append(closedFds, int(fd))
		return nil
	}

	fd, err := sharedInhibitAcquire("test reason")
	assert.NoError(t, err)
	assert.Equal(t, dbus.UnixFD(42), fd)
	assert.Equal(t, 1, sharedInhibitRef)

	fd2, err := sharedInhibitAcquire("test reason 2")
	assert.NoError(t, err)
	assert.Equal(t, dbus.UnixFD(42), fd2)
	assert.Equal(t, 2, sharedInhibitRef)

	err = sharedInhibitRelease()
	assert.NoError(t, err)
	assert.Equal(t, 1, sharedInhibitRef)

	err = sharedInhibitRelease()
	assert.NoError(t, err)
	assert.Equal(t, 0, sharedInhibitRef)
	assert.Equal(t, dbus.UnixFD(-1), sharedInhibitFd)
	assert.Equal(t, []int{42}, closedFds)
}

func TestSharedInhibitAcquireFailure(t *testing.T) {
	originalFn := inhibitorFn
	originalCloseFn := closeInhibitFd
	defer func() {
		inhibitorFn = originalFn
		closeInhibitFd = originalCloseFn
		sharedInhibitRef = 0
		sharedInhibitFd = -1
	}()

	inhibitorFn = func(what, who, why string) (dbus.UnixFD, error) {
		return 0, assert.AnError
	}

	fd, err := sharedInhibitAcquire("test")
	assert.Error(t, err)
	assert.Equal(t, dbus.UnixFD(0), fd)
	assert.Equal(t, 0, sharedInhibitRef)
}

func TestSharedInhibitReleaseNoRef(t *testing.T) {
	originalRef := sharedInhibitRef
	originalFd := sharedInhibitFd
	defer func() {
		sharedInhibitRef = originalRef
		sharedInhibitFd = originalFd
	}()

	sharedInhibitRef = 0
	sharedInhibitFd = -1
	err := sharedInhibitRelease()
	assert.NoError(t, err)
}

func TestUpdateSystemOnChanging(t *testing.T) {
	originalFn := inhibitorFn
	originalCloseFn := closeInhibitFd
	defer func() {
		inhibitorFn = originalFn
		closeInhibitFd = originalCloseFn
		sharedInhibitRef = 0
		sharedInhibitFd = -1
	}()

	var capturedWhy string
	var closedFd dbus.UnixFD
	inhibitorFn = func(what, who, why string) (dbus.UnixFD, error) {
		capturedWhy = why
		return dbus.UnixFD(42), nil
	}
	closeInhibitFd = func(fd dbus.UnixFD) error {
		closedFd = fd
		return nil
	}

	m := &Manager{inhibitFd: -1}

	// Acquire: install-like job selects the "Installing updates..." reason.
	m.updateSystemOnChanging(true, true)
	assert.Equal(t, dbus.UnixFD(42), m.inhibitFd)
	assert.Contains(t, capturedWhy, "Installing updates")

	// Repeat acquire while already inhibited is a no-op.
	capturedWhy = ""
	m.updateSystemOnChanging(true, false)
	assert.Empty(t, capturedWhy)
	assert.Equal(t, dbus.UnixFD(42), m.inhibitFd)

	// Release closes the fd and resets to -1.
	m.updateSystemOnChanging(false, false)
	assert.Equal(t, dbus.UnixFD(-1), m.inhibitFd)
	assert.Equal(t, dbus.UnixFD(42), closedFd)

	// Repeat release is a no-op.
	closedFd = -1
	m.updateSystemOnChanging(false, false)
	assert.Equal(t, dbus.UnixFD(-1), closedFd)
}

func TestUpdateSystemOnChangingTaskReason(t *testing.T) {
	originalFn := inhibitorFn
	originalCloseFn := closeInhibitFd
	defer func() {
		inhibitorFn = originalFn
		closeInhibitFd = originalCloseFn
		sharedInhibitRef = 0
		sharedInhibitFd = -1
	}()

	var capturedWhy string
	inhibitorFn = func(what, who, why string) (dbus.UnixFD, error) {
		capturedWhy = why
		return dbus.UnixFD(42), nil
	}
	closeInhibitFd = func(fd dbus.UnixFD) error { return nil }

	m := &Manager{inhibitFd: -1}
	m.updateSystemOnChanging(true, false)
	assert.Contains(t, capturedWhy, "Tasks are running")
}
