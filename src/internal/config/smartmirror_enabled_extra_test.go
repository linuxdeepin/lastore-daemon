// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSmartMirrorEnabledFileNotExist(t *testing.T) {
	oldPath := smartMirrorConfigPath
	smartMirrorConfigPath = filepath.Join(t.TempDir(), "smartmirror_config.json")
	t.Cleanup(func() { smartMirrorConfigPath = oldPath })

	// File does not exist → DecodeJson returns error → default to true
	assert.True(t, isSmartMirrorEnabled())
}

func TestIsSmartMirrorEnabledDisabled(t *testing.T) {
	oldPath := smartMirrorConfigPath
	dir := t.TempDir()
	smartMirrorConfigPath = filepath.Join(dir, "smartmirror_config.json")
	t.Cleanup(func() { smartMirrorConfigPath = oldPath })

	require.NoError(t, os.WriteFile(smartMirrorConfigPath, []byte(`{"Enable": false}`), 0644))
	assert.False(t, isSmartMirrorEnabled())
}

func TestIsSmartMirrorEnabledEnabled(t *testing.T) {
	oldPath := smartMirrorConfigPath
	dir := t.TempDir()
	smartMirrorConfigPath = filepath.Join(dir, "smartmirror_config.json")
	t.Cleanup(func() { smartMirrorConfigPath = oldPath })

	require.NoError(t, os.WriteFile(smartMirrorConfigPath, []byte(`{"Enable": true}`), 0644))
	assert.True(t, isSmartMirrorEnabled())
}

func TestIsSmartMirrorEnabledInvalidJSON(t *testing.T) {
	oldPath := smartMirrorConfigPath
	dir := t.TempDir()
	smartMirrorConfigPath = filepath.Join(dir, "smartmirror_config.json")
	t.Cleanup(func() { smartMirrorConfigPath = oldPath })

	require.NoError(t, os.WriteFile(smartMirrorConfigPath, []byte(`{invalid json`), 0644))
	// Invalid JSON → DecodeJson returns error → default to true
	assert.True(t, isSmartMirrorEnabled())
}

func TestUseIncrementalUpdateEditionError(t *testing.T) {
	oldEditionFn := getEditionNameFn
	oldSmartMirrorFn := isSmartMirrorEnabledFn
	t.Cleanup(func() {
		getEditionNameFn = oldEditionFn
		isSmartMirrorEnabledFn = oldSmartMirrorFn
	})
	getEditionNameFn = func() (string, error) {
		return "", assert.AnError
	}
	isSmartMirrorEnabledFn = func() bool { return false }

	c := &Config{IncrementalUpdate: true}
	// When edition lookup fails, UseIncrementalUpdate returns true.
	assert.True(t, c.UseIncrementalUpdate())
}
