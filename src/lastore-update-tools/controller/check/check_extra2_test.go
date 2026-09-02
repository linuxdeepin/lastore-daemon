// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAPTAndDPKGState(t *testing.T) {
	// apt and dpkg should exist in the test environment
	err := CheckAPTAndDPKGState()
	assert.NoError(t, err)
}

func TestLoadSysPkgInfo(t *testing.T) {
	pkgs := map[string]*cache.AppTinyInfo{
		"old-pkg": {Name: "old-pkg", Version: "0.0.1"},
	}

	err := LoadSysPkgInfo(pkgs)
	require.NoError(t, err)

	// old entry should be cleared
	_, ok := pkgs["old-pkg"]
	assert.False(t, ok, "old entries should be cleared")

	// dpkg should be present
	_, ok = pkgs["dpkg"]
	assert.True(t, ok, "dpkg should be in the result")
}

func TestCheckRootDiskFreeSpaceSmall(t *testing.T) {
	// requesting 0 bytes should always succeed
	err := CheckRootDiskFreeSpace(0)
	assert.NoError(t, err)
}

func TestCheckRootDiskFreeSpaceHuge(t *testing.T) {
	// requesting an impossibly large amount should fail
	err := CheckRootDiskFreeSpace(999999999999999)
	assert.Error(t, err)
}

func TestCheckDataDiskFreeSpaceSmall(t *testing.T) {
	// requesting 0 bytes should always succeed
	err := CheckDataDiskFreeSpace(0)
	assert.NoError(t, err)
}

func TestCheckDataDiskFreeSpaceHuge(t *testing.T) {
	// requesting an impossibly large amount should fail
	err := CheckDataDiskFreeSpace(999999999999999)
	assert.Error(t, err)
}
