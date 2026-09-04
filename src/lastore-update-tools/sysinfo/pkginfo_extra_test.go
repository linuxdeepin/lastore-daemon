// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"os"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAppIsExist(t *testing.T) {
	// existing file
	exists, err := CheckAppIsExist("/usr/bin/dpkg")
	assert.NoError(t, err)
	assert.True(t, exists)

	// non-existing file
	exists, err = CheckAppIsExist("/nonexistent/path/to/file")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestGetCurrInstPkgStat(t *testing.T) {
	pkgs := map[string]*cache.AppTinyInfo{
		"old-pkg": {Name: "old-pkg", Version: "0.0.1"},
	}

	err := GetCurrInstPkgStat(pkgs)
	require.NoError(t, err)

	// old entry should be cleared
	_, ok := pkgs["old-pkg"]
	assert.False(t, ok, "old entries should be cleared")

	// dpkg should be in the result since it's installed
	dpkgInfo, ok := pkgs["dpkg"]
	if ok {
		assert.Equal(t, "dpkg", dpkgInfo.Name)
		assert.NotEmpty(t, dpkgInfo.Version)
	}

	// write a temp file to verify CheckAppIsExist via fs
	tmpFile, err := os.CreateTemp("", "test*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	exists, err := CheckAppIsExist(tmpFile.Name())
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestGetSysPkgStateAndVersionNotFound(t *testing.T) {
	_, _, err := GetSysPkgStateAndVersion("__nonexistent_pkg_xyz_123__")
	assert.Error(t, err)
}
