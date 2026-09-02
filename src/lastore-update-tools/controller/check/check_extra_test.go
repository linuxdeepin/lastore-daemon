// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetDynHookTimeoutPositive(t *testing.T) {
	orig := DynHookTimeout
	defer func() { DynHookTimeout = orig }()

	SetDynHookTimeout(300)
	assert.Equal(t, 300, DynHookTimeout)
}

func TestSetDynHookTimeoutZero(t *testing.T) {
	orig := DynHookTimeout
	defer func() { DynHookTimeout = orig }()

	DynHookTimeout = 100
	SetDynHookTimeout(0)
	assert.Equal(t, 100, DynHookTimeout)
}

func TestSetDynHookTimeoutNegative(t *testing.T) {
	orig := DynHookTimeout
	defer func() { DynHookTimeout = orig }()

	DynHookTimeout = 100
	SetDynHookTimeout(-5)
	assert.Equal(t, 100, DynHookTimeout)
}

func TestCheckDPKGVersionSupportDeepin(t *testing.T) {
	pkgs := map[string]*cache.AppTinyInfo{
		"dpkg": {Name: "dpkg", Version: "1.21.1deepin"},
	}
	err := CheckDPKGVersionSupport(pkgs)
	assert.NoError(t, err)
}

func TestCheckDPKGVersionSupportDDE(t *testing.T) {
	pkgs := map[string]*cache.AppTinyInfo{
		"dpkg": {Name: "dpkg", Version: "1.21.1dde"},
	}
	err := CheckDPKGVersionSupport(pkgs)
	assert.NoError(t, err)
}

func TestCheckDPKGVersionSupportUnsupported(t *testing.T) {
	pkgs := map[string]*cache.AppTinyInfo{
		"dpkg": {Name: "dpkg", Version: "1.21.1"},
	}
	err := CheckDPKGVersionSupport(pkgs)
	assert.Error(t, err)
}

func TestCheckDPKGVersionSupportNotFound(t *testing.T) {
	pkgs := map[string]*cache.AppTinyInfo{}
	err := CheckDPKGVersionSupport(pkgs)
	assert.Error(t, err)
}

func TestCheckVerifyCacheInfoValid(t *testing.T) {
	cfg := &cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgDebPath:  "/tmp/debs",
			UUID:        "test-uuid",
			RepoBackend: []cache.RepoInfo{{Name: "test", FilePath: "/nonexistent"}},
		},
	}
	err := CheckVerifyCacheInfo(cfg)
	assert.Error(t, err)
}

func TestCheckVerifyCacheInfoEmptyPkgDebPath(t *testing.T) {
	cfg := &cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			UUID: "test-uuid",
		},
	}
	err := CheckVerifyCacheInfo(cfg)
	assert.Error(t, err)
}

func TestCheckVerifyCacheInfoEmptyUUID(t *testing.T) {
	cfg := &cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			PkgDebPath: "/tmp/debs",
		},
	}
	err := CheckVerifyCacheInfo(cfg)
	assert.Error(t, err)
}

func TestCheckCoreFileExistValid(t *testing.T) {
	tmpDir := t.TempDir()
	coreFile := filepath.Join(tmpDir, "core.list")
	validPath := filepath.Join(tmpDir, "existing.txt")
	require.NoError(t, os.WriteFile(validPath, []byte("data"), 0644))
	require.NoError(t, os.WriteFile(coreFile, []byte(validPath), 0644))

	err := CheckCoreFileExist(coreFile)
	assert.NoError(t, err)
}

func TestCheckCoreFileExistWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	coreFile := filepath.Join(tmpDir, "core.list")
	validPath := filepath.Join(tmpDir, "existing.txt")
	require.NoError(t, os.WriteFile(validPath, []byte("data"), 0644))
	require.NoError(t, os.WriteFile(coreFile, []byte("# commented\n"+validPath), 0644))

	err := CheckCoreFileExist(coreFile)
	assert.NoError(t, err)
}

func TestCheckCoreFileExistNonExistentFile(t *testing.T) {
	err := CheckCoreFileExist("/nonexistent/path/core.list")
	assert.Error(t, err)
}

func TestCheckCoreFileExistNonExistentPathInside(t *testing.T) {
	tmpDir := t.TempDir()
	coreFile := filepath.Join(tmpDir, "core.list")
	require.NoError(t, os.WriteFile(coreFile, []byte("/nonexistent/path/inside\n"), 0644))

	err := CheckCoreFileExist(coreFile)
	assert.Error(t, err)
}

func TestCheckDebListInstallStateMissing(t *testing.T) {
	midpkgs := map[string]*cache.AppTinyInfo{}
	pkginfo := &cache.AppInfo{Name: "testpkg"}
	err := CheckDebListInstallState(midpkgs, pkginfo, "precheck", "test")
	assert.Error(t, err)
}

func TestCheckDebListInstallStateSkipState(t *testing.T) {
	midpkgs := map[string]*cache.AppTinyInfo{
		"testpkg": {Name: "testpkg", State: cache.PkgState("broken")},
	}
	pkginfo := &cache.AppInfo{Name: "testpkg", Need: "skipstate"}
	err := CheckDebListInstallState(midpkgs, pkginfo, "precheck", "test")
	assert.NoError(t, err)
}

func TestCheckDebListInstallStateExist(t *testing.T) {
	midpkgs := map[string]*cache.AppTinyInfo{
		"testpkg": {Name: "testpkg", State: cache.PkgState("broken")},
	}
	pkginfo := &cache.AppInfo{Name: "testpkg", Need: "exist"}
	err := CheckDebListInstallState(midpkgs, pkginfo, "precheck", "test")
	assert.NoError(t, err)
}

func TestCheckImportantProcessInvalidStage(t *testing.T) {
	err := CheckImportantProcess("invalid_stage")
	assert.Error(t, err)
}
