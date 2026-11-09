// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMirrorsJSON = `[
	{"id": "default", "url": "https://default.example.com/deepin", "weight": 100},
	{"id": "Aliyun", "url": "https://aliyun.example.com/deepin/", "weight": 10}
]`

// setupMirrorTestFiles points the mirror helpers at temporary files and returns
// the fake apt.conf.d directory.
func setupMirrorTestFiles(t *testing.T, mirrorsJSON string) string {
	t.Helper()

	dir := t.TempDir()
	mirrorsPath := filepath.Join(dir, "mirrors.json")
	require.NoError(t, os.WriteFile(mirrorsPath, []byte(mirrorsJSON), 0644))

	aptDir := filepath.Join(dir, "apt.conf.d")
	require.NoError(t, os.MkdirAll(aptDir, 0755))

	oldMirrorsPath := mirrorsJSONPath
	oldAptConfDir := aptConfDir
	mirrorsJSONPath = mirrorsPath
	aptConfDir = aptDir
	t.Cleanup(func() {
		mirrorsJSONPath = oldMirrorsPath
		aptConfDir = oldAptConfDir
	})

	return aptDir
}

func readSmartMirrorConfig(t *testing.T, aptDir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(aptDir, smartMirrorConfigName))
	require.NoError(t, err)
	return string(data)
}

func TestResolveMirrorSourceKeepsExistingID(t *testing.T) {
	aptDir := setupMirrorTestFiles(t, testMirrorsJSON)

	cfg := &config.Config{MirrorSource: "Aliyun"}
	resolveMirrorSource(cfg)

	assert.Equal(t, "Aliyun", cfg.MirrorSource)
	_, err := os.Stat(filepath.Join(aptDir, smartMirrorConfigName))
	assert.True(t, os.IsNotExist(err))
}

func TestResolveMirrorSourceResetsRemovedID(t *testing.T) {
	aptDir := setupMirrorTestFiles(t, testMirrorsJSON)

	// cfg has no backing config file, so persisting the reset fails; the
	// in-memory value is still updated to the value main will use.
	cfg := &config.Config{MirrorSource: "Removed"}
	resolveMirrorSource(cfg)

	assert.Equal(t, config.OfficialCDNMirrorSourceID, cfg.MirrorSource)
	assert.Equal(t,
		aptSmartMirrorConfig("https://default.example.com/deepin"),
		readSmartMirrorConfig(t, aptDir))
}

func TestResolveMirrorSourcePersistsReset(t *testing.T) {
	setupMirrorTestFiles(t, testMirrorsJSON)

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.NewConfig(cfgPath)
	original := cfg.MirrorSource
	t.Cleanup(func() {
		if original != "" {
			_ = cfg.SetMirrorSource(original)
		}
	})
	cfg.MirrorSource = "Removed"

	resolveMirrorSource(cfg)

	require.Equal(t, config.OfficialCDNMirrorSourceID, cfg.MirrorSource)
	// Reload to prove the reset went through save(): when dsettings is active
	// the value is read back from there, otherwise from config.json.
	reloaded := config.NewConfig(cfgPath)
	assert.Equal(t, config.OfficialCDNMirrorSourceID, reloaded.MirrorSource)
}

func TestResolveMirrorSourceKeepsSourceOnInvalidDefaultURL(t *testing.T) {
	aptDir := setupMirrorTestFiles(t, `[
		{"id": "default", "url": "ftp://default.example.com/deepin", "weight": 100},
		{"id": "Aliyun", "url": "https://aliyun.example.com/deepin/", "weight": 10}
	]`)

	const oldConfig = `Acquire::SmartMirrors::MirrorSource "https://old.example.com/deepin/";`
	require.NoError(t, os.WriteFile(
		filepath.Join(aptDir, smartMirrorConfigName), []byte(oldConfig), 0644))

	cfg := &config.Config{MirrorSource: "Removed"}
	resolveMirrorSource(cfg)

	assert.Equal(t, "Removed", cfg.MirrorSource)
	assert.Equal(t, oldConfig, readSmartMirrorConfig(t, aptDir))
}

func TestResolveMirrorSourceSkipsEmptySource(t *testing.T) {
	aptDir := setupMirrorTestFiles(t, testMirrorsJSON)

	cfg := &config.Config{}
	resolveMirrorSource(cfg)

	_, err := os.Stat(filepath.Join(aptDir, smartMirrorConfigName))
	assert.True(t, os.IsNotExist(err))
}

func TestSetAPTSmartMirrorKeepsFileMode(t *testing.T) {
	dir := t.TempDir()
	oldAptConfDir := aptConfDir
	aptConfDir = dir
	t.Cleanup(func() { aptConfDir = oldAptConfDir })

	path := filepath.Join(dir, smartMirrorConfigName)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0600))

	require.NoError(t, SetAPTSmartMirror("https://new.example.com/deepin/"))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	assert.Equal(t,
		aptSmartMirrorConfig("https://new.example.com/deepin/"),
		readSmartMirrorConfig(t, dir))
}

func TestValidateMirrorURLRequiresHost(t *testing.T) {
	assert.Error(t, validateMirrorURL("https://"))
	assert.Error(t, validateMirrorURL("http:///path"))
	assert.NoError(t, validateMirrorURL("https://mirror.example.com/deepin"))
}
