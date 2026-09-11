// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
)

// newFakeDutSystem 构造一个使用假二进制(exit 0)的 DutSystem。
func newFakeDutSystem(t *testing.T) *DutSystem {
	t.Helper()
	dir := t.TempDir()

	writeScript := func(name string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755))
		return path
	}

	oldApt := apt.AptGetBinPath
	oldImm := apt.DeepinImmutableCtlPath
	oldDpkg := apt.DpkgBinPath
	apt.AptGetBinPath = writeScript("apt-get")
	apt.DeepinImmutableCtlPath = writeScript("deepin-immutable-ctl")
	apt.DpkgBinPath = writeScript("dpkg")
	t.Cleanup(func() {
		apt.AptGetBinPath = oldApt
		apt.DeepinImmutableCtlPath = oldImm
		apt.DpkgBinPath = oldDpkg
	})

	// checkSystemDependsError / safeStart 使用相对路径 "apt-get",需 PATH 前置假命令。
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &DutSystem{
		APTSystem: apt.APTSystem{
			CmdSet:    make(map[string]*system.Command),
			Indicator: func(system.JobProgressInfo) {},
		},
	}
}

func TestDutUpdateSourceExtra(t *testing.T) {
	d := newFakeDutSystem(t)
	err := d.UpdateSource("job-1", map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestDutFixErrorExtra(t *testing.T) {
	d := newFakeDutSystem(t)
	err := d.FixError("job-1", string(system.ErrorDependenciesBroken), map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestDutOsBackupExtra(t *testing.T) {
	d := newFakeDutSystem(t)
	err := d.OsBackup("job-1")
	assert.NoError(t, err)
}

func TestDutDistUpgradeExtra(t *testing.T) {
	d := newFakeDutSystem(t)
	err := d.DistUpgrade("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestDutCheckSystemExtra(t *testing.T) {
	d := newFakeDutSystem(t)
	err := d.CheckSystem("job-1", "post-upgrade-check", map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}
