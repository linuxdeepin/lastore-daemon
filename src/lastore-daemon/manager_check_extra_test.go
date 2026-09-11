// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redirectRebootCheckOptionPaths points optionFilePath/optionFilePathTemp at a
// temp dir so tests never touch the real /etc or /tmp files.
func redirectRebootCheckOptionPaths(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldPath := optionFilePath
	oldTemp := optionFilePathTemp
	optionFilePath = filepath.Join(dir, "deepin_update_option.json")
	optionFilePathTemp = filepath.Join(dir, "deepin_update_option.json.tmp")
	t.Cleanup(func() {
		optionFilePath = oldPath
		optionFilePathTemp = oldTemp
	})
}

func TestCheckUpgrade(t *testing.T) {
	m := &Manager{service: newTestService(), jobManager: newTestJobManager()}
	path, err := m.checkUpgrade(ifcTestSender, system.SystemUpdate, firstCheck)
	assert.NotEmpty(t, path)
	assert.NoError(t, err)
}

func TestSetRebootCheckOption(t *testing.T) {
	// SecurityUpdate has no SystemUpdate bit, so isMajorUpgradeForMode returns
	// false without dereferencing updatePlatform.
	m := &Manager{systemd: newFailingSystemdManager(t)}
	_ = m.setRebootCheckOption(system.SecurityUpdate, "test-uuid")
}

func TestGetRebootCheckJobUUID(t *testing.T) {
	_ = getRebootCheckJobUUID()
}

func TestDelRebootCheckOptionInvalidType(t *testing.T) {
	assert.Error(t, (&Manager{}).delRebootCheckOption(checkType(99)))
}

func TestJobType(t *testing.T) {
	assert.Equal(t, "first check", checkType(firstCheck).JobType())
	assert.Equal(t, "second check", checkType(secondCheck).JobType())
	assert.Equal(t, "invalid type", checkType(all).JobType())
	assert.Equal(t, "invalid type", checkType(0).JobType())
}

func TestIsMajorUpgradeForMode(t *testing.T) {
	m := &Manager{updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false)}

	// SecurityUpdate has no SystemUpdate bit: returns false without consulting
	// updatePlatform.
	assert.False(t, m.isMajorUpgradeForMode(system.SecurityUpdate))
	// SystemUpdate bit present: consults updatePlatform.IsMajorUpgrade(). With
	// no targetVersion configured it reports false, but the call path is hit.
	assert.False(t, m.isMajorUpgradeForMode(system.SystemUpdate))
	assert.False(t, m.isMajorUpgradeForMode(system.SystemUpdate|system.SecurityUpdate))
}

func TestGetRebootCheckJobUUIDPaths(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		redirectRebootCheckOptionPaths(t)
		assert.Empty(t, getRebootCheckJobUUID())
	})

	t.Run("valid json", func(t *testing.T) {
		redirectRebootCheckOptionPaths(t)
		content, err := json.Marshal(fullUpgradeOption{UUID: "test-uuid-42"})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(optionFilePath, content, 0644))
		assert.Equal(t, "test-uuid-42", getRebootCheckJobUUID())
	})

	t.Run("malformed json", func(t *testing.T) {
		redirectRebootCheckOptionPaths(t)
		require.NoError(t, os.WriteFile(optionFilePath, []byte("{not-json"), 0644))
		assert.Empty(t, getRebootCheckJobUUID())
	})
}

func TestDelRebootCheckOptionFirstCheck(t *testing.T) {
	redirectRebootCheckOptionPaths(t)
	content, err := json.Marshal(fullUpgradeOption{PreGreeterCheck: true, UUID: "u-1"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(optionFilePath, content, 0644))

	require.NoError(t, (&Manager{}).delRebootCheckOption(firstCheck))

	data, err := os.ReadFile(optionFilePath)
	require.NoError(t, err)
	var opt fullUpgradeOption
	require.NoError(t, json.Unmarshal(data, &opt))
	assert.False(t, opt.PreGreeterCheck)
	assert.Equal(t, "u-1", opt.UUID)
}

func TestDelRebootCheckOptionFirstCheckMissingFile(t *testing.T) {
	redirectRebootCheckOptionPaths(t)
	assert.Error(t, (&Manager{}).delRebootCheckOption(firstCheck))
}

func TestDelRebootCheckOptionRemove(t *testing.T) {
	for _, order := range []checkType{secondCheck, all} {
		t.Run(order.JobType(), func(t *testing.T) {
			redirectRebootCheckOptionPaths(t)
			require.NoError(t, os.WriteFile(optionFilePath, []byte("opt"), 0644))
			require.NoError(t, os.WriteFile(optionFilePathTemp, []byte("tmp"), 0644))

			m := &Manager{systemd: newFailingSystemdManager(t)}
			require.NoError(t, m.delRebootCheckOption(order))

			assert.NoFileExists(t, optionFilePath)
			assert.NoFileExists(t, optionFilePathTemp)
		})
	}
}
