// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemdCheckerIsUnitActive_GetUnitError(t *testing.T) {
	mgr := &systemd1.MockManager{}
	mgr.MockInterfaceManager.On("GetUnit", dbus.Flags(0), "missing.service").
		Return(dbus.ObjectPath(""), errors.New("unit not found"))

	c := &SystemdChecker{manager: mgr}
	active, err := c.IsUnitActive("missing.service")

	assert.False(t, active)
	assert.NoError(t, err)
	mgr.MockInterfaceManager.AssertExpectations(t)
}

func TestNewSystemdChecker(t *testing.T) {
	checker, err := NewSystemdChecker()
	if err != nil {
		t.Skipf("system bus not available: %v", err)
	}
	require.NotNil(t, checker)
	require.NotNil(t, checker.conn)
}

func TestCheckImportantService(t *testing.T) {
	// Stage1/Stage2 both check display-manager.service; the result depends on
	// whether the unit is active on the host, so only assert the error shape.
	err := CheckImportantService(Stage1)
	if err != nil {
		var jobErr *system.JobError
		assert.ErrorAs(t, err, &jobErr)
	}

	// An unknown stage parameter is rejected with a JobError.
	err = CheckImportantService("invalid-stage")
	var jobErr *system.JobError
	assert.ErrorAs(t, err, &jobErr)
}

func TestCheckImportantProcessPidofError(t *testing.T) {
	old := pidofRunner
	pidofRunner = func(int, string, ...string) (string, error) {
		return "", errors.New("pidof failed")
	}
	t.Cleanup(func() { pidofRunner = old })

	err := CheckImportantProcess(Stage1)
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckProgramFailed, jobErr.ErrType)
}

func TestCheckImportantProcessNotRunning(t *testing.T) {
	old := pidofRunner
	pidofRunner = func(int, string, ...string) (string, error) {
		return "", nil
	}
	t.Cleanup(func() { pidofRunner = old })

	err := CheckImportantProcess(Stage2)
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckProcessNotRunning, jobErr.ErrType)
}

func TestCheckImportantProcessRunning(t *testing.T) {
	old := pidofRunner
	pidofRunner = func(int, string, ...string) (string, error) {
		return "1234", nil
	}
	t.Cleanup(func() { pidofRunner = old })

	err := CheckImportantProcess(Stage1)
	assert.NoError(t, err)
}
