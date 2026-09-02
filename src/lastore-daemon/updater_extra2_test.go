// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
