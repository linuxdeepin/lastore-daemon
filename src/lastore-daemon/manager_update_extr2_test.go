// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
)

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
