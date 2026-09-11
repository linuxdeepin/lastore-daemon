// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

// exitPanic is a sentinel used by the stubbed osExit to halt execution once a
// code path calls osExit, instead of terminating the test process.
type exitPanic int

// stubExit replaces the osExit seam with a recorder that stores the exit code
// and panics. The returned restore func MUST be deferred.
func stubExit() (restore func(), exitCode *int) {
	old := osExit
	code := 0
	osExit = func(c int) {
		code = c
		panic(exitPanic(c))
	}
	return func() { osExit = old }, &code
}

func TestMainUsageExit(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// Only 3 args (plus the program name) → the usage branch exits -1.
	os.Args = []string{"smartmirror", "http://example.com/a.deb", "official.example.com"}

	restore, code := stubExit()
	defer restore()

	var caught any
	func() {
		defer func() { caught = recover() }()
		main()
	}()
	p, ok := caught.(exitPanic)
	assert.True(t, ok, "expected osExit panic, got %v", caught)
	assert.Equal(t, -1, int(p))
	assert.Equal(t, -1, *code)
}

func TestMainSystemBusError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"smartmirror", "http://example.com/a.deb", "official.example.com", "mirror.example.com"}

	oldBus := systemBusFn
	systemBusFn = func() (*dbus.Conn, error) {
		return nil, errors.New("no system bus")
	}
	defer func() { systemBusFn = oldBus }()

	// Should print the raw URL and return without panicking or exiting.
	assert.NotPanics(t, main)
}
