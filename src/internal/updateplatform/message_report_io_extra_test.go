// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"os"
	"path/filepath"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")

	content := []byte("hello world")
	require.NoError(t, os.WriteFile(srcPath, content, 0644))

	copyFile(srcPath, dstPath)

	got, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestCopyFileSrcNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	dstPath := filepath.Join(tmpDir, "dst.txt")
	// Should not panic, just log warning
	copyFile(filepath.Join(tmpDir, "nonexistent"), dstPath)
	assert.NoFileExists(t, dstPath)
}

func TestUpdateKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baseline.conf")

	result := updateKeyFile(path, "Baseline", "25.1")
	assert.True(t, result)

	// Verify by reading back with getGeneralValueFromKeyFile
	val := getGeneralValueFromKeyFile(path, "Baseline")
	assert.Equal(t, "25.1", val)
}

func TestUpdateKeyFileExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baseline.conf")

	// First write
	assert.True(t, updateKeyFile(path, "Baseline", "25.0"))
	// Update with new value
	assert.True(t, updateKeyFile(path, "Baseline", "25.1"))
	// Add another key
	assert.True(t, updateKeyFile(path, "SystemType", "x86"))

	assert.Equal(t, "25.1", getGeneralValueFromKeyFile(path, "Baseline"))
	assert.Equal(t, "x86", getGeneralValueFromKeyFile(path, "SystemType"))
}

func TestUpdateBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baseline.conf")
	assert.True(t, updateBaseline(path, "1060"))
	assert.Equal(t, "1060", getGeneralValueFromKeyFile(path, "Baseline"))
}

func TestUpdateSystemType(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baseline.conf")
	assert.True(t, updateSystemType(path, "x86_64"))
	assert.Equal(t, "x86_64", getGeneralValueFromKeyFile(path, "SystemType"))
}

func TestUpdateVersion(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "baseline.conf")
	assert.True(t, updateVersion(path, "25.1.0"))
	assert.Equal(t, "25.1.0", getGeneralValueFromKeyFile(path, "Version"))
}

func TestGetTargetBaseline(t *testing.T) {
	// cacheBaseline = "/var/lib/lastore/os-baseline.b" - may not exist in test env
	// Just verify it doesn't panic and returns a string
	result := getTargetBaseline()
	_ = result
}

func TestGetTargetSystemType(t *testing.T) {
	result := getTargetSystemType()
	_ = result
}

func TestGetTargetVersion(t *testing.T) {
	result := getTargetVersion()
	_ = result
}

func TestGetUpdateTarget(t *testing.T) {
	m := &UpdatePlatformManager{
		targetVersion:  "25.1",
		targetBaseline: "1060",
		checkTime:      "2026-01-01",
	}
	result := m.GetUpdateTarget()
	assert.Contains(t, result, "25.1")
	assert.Contains(t, result, "1060")
	assert.Contains(t, result, "2026-01-01")
}

func TestGetUpdateTargetEmpty(t *testing.T) {
	m := &UpdatePlatformManager{}
	result := m.GetUpdateTarget()
	assert.Contains(t, result, "TargetVersion")
}

func TestIsUnstable(t *testing.T) {
	// No DBus in test env, should return ReleaseVersion (1)
	result := isUnstable()
	assert.Equal(t, ReleaseVersion, result)
}

func TestGetPlatformRepoSources(t *testing.T) {
	m := &UpdatePlatformManager{
		repoInfos: []repoInfo{
			{Source: "deb http://example.com stable main"},
			{Uri: "http://test.com", CodeName: "stable"},
		},
	}
	repos := m.GetPlatformRepoSources()
	assert.Len(t, repos, 2)
	assert.Contains(t, repos[0], "deb http://example.com stable main")
	assert.Contains(t, repos[1], "deb http://test.com stable")
}

func TestGetPlatformRepoSourcesWithConfig(t *testing.T) {
	m := &UpdatePlatformManager{
		repoInfos: []repoInfo{
			{Uri: "http://test.com", CodeName: "stable"},
		},
		config: &Cfg.Config{PlatformRepoComponents: "main contrib"},
	}
	repos := m.GetPlatformRepoSources()
	assert.Len(t, repos, 1)
	assert.Contains(t, repos[0], "main contrib")
}
