// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestGetLang(t *testing.T) {
	tests := []struct {
		name   string
		env    procfs.EnvVars
		expect string
	}{
		{"LC_ALL priority", procfs.EnvVars{"LC_ALL=fr_FR.UTF-8", "LANG=en_US.UTF-8"}, "fr_FR.UTF-8"},
		{"LANG fallback", procfs.EnvVars{"LANG=zh_CN.UTF-8"}, "zh_CN.UTF-8"},
		{"empty", procfs.EnvVars{}, ""},
		{"LC_MESSAGE", procfs.EnvVars{"LC_MESSAGE=de_DE", "LANG=en_US"}, "de_DE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, getLang(tt.env))
		})
	}
}

func TestGetUsedLang(t *testing.T) {
	env := map[string]string{"DEEPIN_LASTORE_LANG": "zh_CN.UTF-8"}
	assert.Equal(t, "zh_CN.UTF-8", getUsedLang(env))

	env2 := map[string]string{}
	assert.Equal(t, "", getUsedLang(env2))
}

func TestBuildDistUpgradePartlyCommand(t *testing.T) {
	cmd := buildDistUpgradePartlyCommand(system.SystemUpdate, true)
	assert.Contains(t, cmd, "dbus-send")
	assert.Contains(t, cmd, "DistUpgradePartly")
	assert.Contains(t, cmd, "uint64:1")
	assert.Contains(t, cmd, "boolean:true")

	cmd2 := buildDistUpgradePartlyCommand(system.SecurityUpdate, false)
	assert.Contains(t, cmd2, "uint64:4")
	assert.Contains(t, cmd2, "boolean:false")
}

func TestIsFirstBoot(t *testing.T) {
	// lastoreUnitCache path "/run/lastore/lastoreUnitCache" likely doesn't exist in test env
	result := isFirstBoot()
	// Just verify it doesn't panic and returns a bool
	_ = result
}

func TestIsTimerUnitFileExists(t *testing.T) {
	// Non-existent unit file
	assert.False(t, isTimerUnitFileExists(UnitName("nonexistent-test-unit")))
}

func TestIsAllowedToTriggerSystemEvent(t *testing.T) {
	// OsVersionChanged should always be allowed
	assert.True(t, isAllowedToTriggerSystemEvent(1000, OsVersionChanged))

	// Root should be allowed for all events
	assert.True(t, isAllowedToTriggerSystemEvent(0, AutoCheck))
	assert.True(t, isAllowedToTriggerSystemEvent(0, AutoClean))

	// Regular user should not be allowed for non-OsVersionChanged events
	// (unless they happen to be the deepin-daemon user)
	assert.False(t, isAllowedToTriggerSystemEvent(99999, AutoCheck))
	assert.False(t, isAllowedToTriggerSystemEvent(99999, AutoClean))
	assert.False(t, isAllowedToTriggerSystemEvent(99999, UpdateTimer))
}
