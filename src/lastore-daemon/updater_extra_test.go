// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetStartupDownloadSpeedLimitConfigEmpty(t *testing.T) {
	cfg := &config.Config{}
	result := getStartupDownloadSpeedLimitConfig(cfg)
	assert.Equal(t, "", result)
}

func TestGetStartupDownloadSpeedLimitConfigLocal(t *testing.T) {
	cfg := &config.Config{
		LocalDownloadSpeedLimitConfig: `{"enabled":true,"speed":1000}`,
	}
	result := getStartupDownloadSpeedLimitConfig(cfg)
	assert.Equal(t, `{"enabled":true,"speed":1000}`, result)
}

func TestGetStartupDownloadSpeedLimitConfigStartupEnabled(t *testing.T) {
	speedConfig, _ := json.Marshal(downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: true,
	})
	cfg := &config.Config{
		DownloadSpeedLimitConfig: string(speedConfig),
	}
	result := getStartupDownloadSpeedLimitConfig(cfg)
	assert.Equal(t, string(speedConfig), result)
}

func TestGetStartupDownloadSpeedLimitConfigStartupDisabledFallsToLocal(t *testing.T) {
	speedConfig, _ := json.Marshal(downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: false,
	})
	cfg := &config.Config{
		DownloadSpeedLimitConfig:      string(speedConfig),
		LocalDownloadSpeedLimitConfig: `{"enabled":true,"speed":2000}`,
	}
	result := getStartupDownloadSpeedLimitConfig(cfg)
	assert.Equal(t, `{"enabled":true,"speed":2000}`, result)
}

func TestValidateMirrorURL(t *testing.T) {
	tests := []struct {
		url    string
		hasErr bool
	}{
		{"http://example.com", false},
		{"https://example.com", false},
		{"ftp://example.com", true},
		{"", true},
		{"not-a-url", true},
		{"file:///etc/passwd", true},
	}
	for _, tt := range tests {
		err := validateMirrorURL(tt.url)
		if tt.hasErr {
			assert.Error(t, err, "validateMirrorURL(%q)", tt.url)
		} else {
			assert.NoError(t, err, "validateMirrorURL(%q)", tt.url)
		}
	}
}

func TestShouldEnableUpgradeDeliveryServiceNil(t *testing.T) {
	assert.False(t, shouldEnableUpgradeDeliveryService(nil, true))
}

func TestShouldEnableUpgradeDeliveryServiceEnabled(t *testing.T) {
	cfg := &config.Config{UpgradeDeliveryEnabled: true}
	assert.True(t, shouldEnableUpgradeDeliveryService(cfg, false))
}

func TestGetLimitConfig(t *testing.T) {
	u := &Updater{}
	u.downloadSpeedLimitConfigObj = downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: true,
		LimitSpeed:                "1024",
		IsOnlineSpeedLimit:        true,
	}

	enabled, limitSpeed, isOnline := u.GetLimitConfig()
	assert.True(t, enabled)
	assert.Equal(t, "1024", limitSpeed)
	assert.True(t, isOnline)
}

func TestGetLimitConfigDisabled(t *testing.T) {
	u := &Updater{}
	u.downloadSpeedLimitConfigObj = downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: false,
		LimitSpeed:                "",
		IsOnlineSpeedLimit:        false,
	}

	enabled, limitSpeed, isOnline := u.GetLimitConfig()
	assert.False(t, enabled)
	assert.Equal(t, "", limitSpeed)
	assert.False(t, isOnline)
}

func TestGetLimitConfigDefaults(t *testing.T) {
	u := &Updater{}

	enabled, limitSpeed, isOnline := u.GetLimitConfig()
	assert.False(t, enabled)
	assert.Equal(t, "", limitSpeed)
	assert.False(t, isOnline)
}

func TestGetIdleDownloadEnabled(t *testing.T) {
	u := &Updater{}
	u.idleDownloadConfigObj = idleDownloadConfig{
		IdleDownloadEnabled: true,
	}
	assert.True(t, u.getIdleDownloadEnabled())

	u.idleDownloadConfigObj.IdleDownloadEnabled = false
	assert.False(t, u.getIdleDownloadEnabled())
}

func TestGetIdleDownloadEnabledDefault(t *testing.T) {
	u := &Updater{}
	assert.False(t, u.getIdleDownloadEnabled())
}
