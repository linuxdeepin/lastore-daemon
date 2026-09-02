// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	updateplatform "github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
)

func TestParseTimeNoSeconds(t *testing.T) {
	tm, err := parseTime("10:30")
	assert.NoError(t, err)
	assert.Equal(t, 10, tm.Hour())
	assert.Equal(t, 30, tm.Minute())
}

func TestParseTimeWithSeconds(t *testing.T) {
	tm, err := parseTime("10:30:45")
	assert.NoError(t, err)
	assert.Equal(t, 10, tm.Hour())
	assert.Equal(t, 30, tm.Minute())
	assert.Equal(t, 45, tm.Second())
}

func TestParseTimeInvalid(t *testing.T) {
	_, err := parseTime("invalid")
	assert.Error(t, err)
}

func TestIsTimeInRangeNormal(t *testing.T) {
	assert.True(t, isTimeInRange("10:00", "09:00", "12:00"))
}

func TestIsTimeInRangeOutside(t *testing.T) {
	assert.False(t, isTimeInRange("08:00", "09:00", "12:00"))
}

func TestIsTimeInRangeCrossMidnight(t *testing.T) {
	assert.True(t, isTimeInRange("23:00", "22:00", "06:00"))
	assert.True(t, isTimeInRange("02:00", "22:00", "06:00"))
	assert.False(t, isTimeInRange("10:00", "22:00", "06:00"))
}

func TestIsTimeInRangeBoundary(t *testing.T) {
	assert.False(t, isTimeInRange("09:00", "09:00", "12:00"))
	assert.False(t, isTimeInRange("12:00", "09:00", "12:00"))
}

func TestIsTimeInRangeInvalidInput(t *testing.T) {
	assert.False(t, isTimeInRange("bad", "09:00", "12:00"))
	assert.False(t, isTimeInRange("10:00", "bad", "12:00"))
	assert.False(t, isTimeInRange("10:00", "09:00", "bad"))
}

func newTestManagerWithUpdatePlatform() *Manager {
	return &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
}

func TestGetCurrentTimeStatePeak(t *testing.T) {
	m := newTestManagerWithUpdatePlatform()
	m.updatePlatform.OnlineRateLimit.PeakTimeRateLimit = updateplatform.PeakOrNotTimeRateLimit{
		Enable:    true,
		StartTime: "07:00",
		EndTime:   "22:00",
	}
	assert.Equal(t, timeStatePeak, m.getCurrentTimeState("10:00"))
}

func TestGetCurrentTimeStateOffPeak(t *testing.T) {
	m := newTestManagerWithUpdatePlatform()
	m.updatePlatform.OnlineRateLimit.OffPeakTimeRateLimit = updateplatform.PeakOrNotTimeRateLimit{
		Enable:    true,
		StartTime: "22:00",
		EndTime:   "07:00",
	}
	assert.Equal(t, timeStateOffPeak, m.getCurrentTimeState("23:00"))
}

func TestGetCurrentTimeStateUnknown(t *testing.T) {
	m := newTestManagerWithUpdatePlatform()
	assert.Equal(t, timeStateUnknown, m.getCurrentTimeState("10:00"))
}

func TestGetCurrentTimeStateDisabled(t *testing.T) {
	m := newTestManagerWithUpdatePlatform()
	m.updatePlatform.OnlineRateLimit.PeakTimeRateLimit = updateplatform.PeakOrNotTimeRateLimit{
		Enable:    false,
		StartTime: "07:00",
		EndTime:   "22:00",
	}
	assert.Equal(t, timeStateUnknown, m.getCurrentTimeState("10:00"))
}

func TestGetDownloadSpeedLimitConfigByTimeStatePeak(t *testing.T) {
	m := newTestManagerWithUpdatePlatform()
	m.updatePlatform.OnlineRateLimit.PeakTimeRateLimit = updateplatform.PeakOrNotTimeRateLimit{
		Bps: 2048,
	}
	cfg := getDownloadSpeedLimitConfigByTimeState(m, timeStatePeak)
	assert.Equal(t, "2048", cfg.LimitSpeed)
	assert.True(t, cfg.IsOnlineSpeedLimit)
	assert.True(t, cfg.DownloadSpeedLimitEnabled)
}

func TestGetDownloadSpeedLimitConfigByTimeStateOffPeak(t *testing.T) {
	m := newTestManagerWithUpdatePlatform()
	m.updatePlatform.OnlineRateLimit.OffPeakTimeRateLimit = updateplatform.PeakOrNotTimeRateLimit{
		Bps: 4096,
	}
	cfg := getDownloadSpeedLimitConfigByTimeState(m, timeStateOffPeak)
	assert.Equal(t, "4096", cfg.LimitSpeed)
	assert.True(t, cfg.IsOnlineSpeedLimit)
	assert.True(t, cfg.DownloadSpeedLimitEnabled)
}

func TestGetDownloadSpeedLimitConfigByTimeStateUnknown(t *testing.T) {
	m := &Manager{config: config.NewConfig("")}
	cfg := getDownloadSpeedLimitConfigByTimeState(m, timeStateUnknown)
	assert.False(t, cfg.IsOnlineSpeedLimit)
	assert.True(t, cfg.DownloadSpeedLimitEnabled)
}
