// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// forceSystemBusFailure points the system bus at a non-existent socket so that
// dbus.SystemBus() fails. This exercises NewConfig's config-file → DSettings
// migration branch (useDSettings == false) without requiring a live dconfig.
func forceSystemBusFailure(t *testing.T) {
	t.Helper()
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-daemon-test-nonexistent-bus.sock")
}

func TestNewConfigMigrationSuccess(t *testing.T) {
	forceSystemBusFailure(t)

	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"Version":"0.1","AutoCheckUpdates":true,"CheckInterval":604800000000000,"CleanInterval":604800000000000,"UpdateMode":3,"Repository":"desktop"}`)
	require.NoError(t, os.WriteFile(configPath, data, 0644))

	cfg := NewConfig(configPath)
	require.NotNil(t, cfg)

	// useDSettings is toggled true after migration.
	assert.True(t, cfg.useDSettings)
	// Config version and update mode were migrated from the file.
	assert.Equal(t, ConfigVersion, cfg.Version)
	assert.Equal(t, system.UpdateType(3), cfg.UpdateMode)
	assert.Equal(t, "desktop", cfg.Repository)
	// Interval from file (604800000000000 ns = 7 days) is >= MinCheckInterval,
	// so the MinCheckInterval clamp is skipped and the value is preserved.
	assert.Equal(t, time.Duration(604800000000000), cfg.CheckInterval)
}

func TestNewConfigMigrationMissingFile(t *testing.T) {
	forceSystemBusFailure(t)

	configPath := filepath.Join(t.TempDir(), "does-not-exist.json")

	cfg := NewConfig(configPath)
	require.NotNil(t, cfg)

	// Migration still runs even though the file is missing: DecodeJson errors
	// out (logged) and useDSettings is still toggled true.
	assert.True(t, cfg.useDSettings)
	// With no migrated version, defaults are applied: Version set, then
	// CheckInterval and CleanInterval are both overwritten to 7 days.
	assert.Equal(t, ConfigVersion, cfg.Version)
	assert.Equal(t, time.Hour*24*7, cfg.CheckInterval)
	assert.Equal(t, time.Hour*24*7, cfg.CleanInterval)
}
