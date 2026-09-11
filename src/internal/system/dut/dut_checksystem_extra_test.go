// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckSystemUnknownTypeNilExtra(t *testing.T) {
	jobErr := CheckSystem(CheckType(99), nil, nil)
	require.NotNil(t, jobErr)
	assert.Equal(t, system.JobErrorType("INVALID_CHECK_TYPE"), jobErr.ErrType)
	assert.True(t, jobErr.IsCheckError)
}

func TestCheckSystemUnknownTypeIndicatorCalledExtra(t *testing.T) {
	var got []system.JobProgressInfo
	indicator := func(info system.JobProgressInfo) {
		got = append(got, info)
	}

	jobErr := CheckSystem(CheckType(99), map[string]string{"k": "v"}, indicator)
	require.NotNil(t, jobErr)

	require.NotEmpty(t, got, "indicator should have been called with the error log")
	assert.True(t, got[0].OnlyLog)
	assert.Contains(t, got[0].OriginalLog, "INVALID_CHECK_TYPE")
}

func TestCheckSystemDependsErrorOKExtra(t *testing.T) {
	installFakeAptGet(t, "0")
	var got []system.JobProgressInfo
	indicator := func(info system.JobProgressInfo) {
		got = append(got, info)
	}
	assert.NoError(t, checkSystemDependsError(indicator))
	require.NotEmpty(t, got)
	assert.True(t, got[0].OnlyLog)
}

func TestCheckSystemDependsErrorFailedExtra(t *testing.T) {
	installFakeAptGet(t, "1")
	var got []system.JobProgressInfo
	indicator := func(info system.JobProgressInfo) {
		got = append(got, info)
	}
	assert.Error(t, checkSystemDependsError(indicator))
}

func installFakeAptGet(t *testing.T, exitCode string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	script := "#!/bin/sh\nexit " + exitCode + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "apt-get"), []byte(script), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
