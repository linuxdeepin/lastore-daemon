// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"strconv"
	"testing"
)

func TestCompareVersionsGeFast(t *testing.T) {
	tests := []struct {
		name    string
		ver1    string
		ver2    string
		want    bool
		wantErr bool
	}{
		{"equal versions", "1.0.0", "1.0.0", true, false},
		{"greater than", "2.0.0", "1.0.0", true, false},
		{"less than", "1.0.0", "2.0.0", false, false},
		{"with revision", "1.0.0-1", "1.0.0-1", true, false},
		{"greater with revision", "1.0.0-2", "1.0.0-1", true, false},
		{"complex versions", "1:2.3.4-5", "1:2.3.4-5", true, false},
		{"invalid version", "abc", "1.0.0", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := compareVersionsGeFast(tt.ver1, tt.ver2)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestCompareVersionsGeDpkg(t *testing.T) {
	if _, err := exec.LookPath("dpkg"); err != nil {
		t.Skip("dpkg not available")
	}

	tests := []struct {
		name string
		ver1 string
		ver2 string
		want bool
	}{
		{"equal versions", "1.0.0", "1.0.0", true},
		{"greater than", "2.0.0", "1.0.0", true},
		{"less than", "1.0.0", "2.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersionsGeDpkg(tt.ver1, tt.ver2)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCompareVersionsLtDpkg(t *testing.T) {
	if _, err := exec.LookPath("dpkg"); err != nil {
		t.Skip("dpkg not available")
	}

	tests := []struct {
		name string
		ver1 string
		ver2 string
		want bool
	}{
		{"less than", "1.0.0", "2.0.0", true},
		{"equal versions", "1.0.0", "1.0.0", false},
		{"greater than", "2.0.0", "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersionsLtDpkg(tt.ver1, tt.ver2)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCompareVersionsGe(t *testing.T) {
	tests := []struct {
		name string
		ver1 string
		ver2 string
		want bool
	}{
		{"equal versions", "1.0.0", "1.0.0", true},
		{"greater than", "2.0.0", "1.0.0", true},
		{"less than", "1.0.0", "2.0.0", false},
		{"with epoch", "1:0.0.1", "0.9.9", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersionsGe(tt.ver1, tt.ver2)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCompareVersionLt(t *testing.T) {
	if _, err := exec.LookPath("dpkg"); err != nil {
		t.Skip("dpkg not available")
	}

	tests := []struct {
		name string
		ver1 string
		ver2 string
		want bool
	}{
		{"less than", "1.0.0", "2.0.0", true},
		{"equal versions", "1.0.0", "1.0.0", false},
		{"greater than", "2.0.0", "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersionLt(tt.ver1, tt.ver2)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDisableOnlineSpeedLimitConfig_AlreadyDisabled(t *testing.T) {
	m := &Manager{config: config.NewConfig("")}
	cfg := downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: false,
		LimitSpeed:                "1024",
		IsOnlineSpeedLimit:        true,
	}
	data, _ := json.Marshal(cfg)
	m.config.SetDownloadSpeedLimitConfig(string(data))

	err := m.disableOnlineSpeedLimitConfig()
	assert.NoError(t, err)
	var result downloadSpeedLimitConfig
	json.Unmarshal([]byte(m.config.DownloadSpeedLimitConfig), &result)
	assert.False(t, result.DownloadSpeedLimitEnabled)
}

func TestDisableOnlineSpeedLimitConfig_Enabled(t *testing.T) {
	m := &Manager{config: config.NewConfig("")}
	cfg := downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: true,
		LimitSpeed:                "2048",
		IsOnlineSpeedLimit:        true,
	}
	data, _ := json.Marshal(cfg)
	m.config.SetDownloadSpeedLimitConfig(string(data))

	err := m.disableOnlineSpeedLimitConfig()
	assert.NoError(t, err)

	var result downloadSpeedLimitConfig
	json.Unmarshal([]byte(m.config.DownloadSpeedLimitConfig), &result)
	assert.False(t, result.DownloadSpeedLimitEnabled)
	assert.Equal(t, "2048", result.LimitSpeed)
}

func TestDisableOnlineSpeedLimitConfig_InvalidJSON(t *testing.T) {
	m := &Manager{config: config.NewConfig("")}
	m.config.SetDownloadSpeedLimitConfig("invalid-json")

	err := m.disableOnlineSpeedLimitConfig()
	assert.NoError(t, err)

	var result downloadSpeedLimitConfig
	json.Unmarshal([]byte(m.config.DownloadSpeedLimitConfig), &result)
	assert.False(t, result.DownloadSpeedLimitEnabled)
	assert.Equal(t, strconv.FormatInt(defaultSpeedLimit, 10), result.LimitSpeed)
	assert.True(t, result.IsOnlineSpeedLimit)
}
