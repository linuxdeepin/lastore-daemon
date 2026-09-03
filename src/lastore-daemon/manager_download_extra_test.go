// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	updateplatform "github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"normal HH:MM", "10:30", false},
		{"normal HH:MM:SS", "10:30:45", false},
		{"midnight", "00:00", false},
		{"end of day", "23:59", false},
		{"with seconds", "23:59:59", false},
		{"empty string", "", true},
		{"invalid format", "25:99", true},
		{"garbage", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

func TestIsTimeInRange(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		start    string
		end      string
		expected bool
	}{
		// Normal range (start < end)
		{"within range", "12:00", "10:00", "14:00", true},
		{"before range", "09:00", "10:00", "14:00", false},
		{"after range", "15:00", "10:00", "14:00", false},
		{"at start boundary", "10:00", "10:00", "14:00", false},
		{"at end boundary", "14:00", "10:00", "14:00", false},
		// Wraparound range (start > end, e.g. overnight)
		{"overnight within", "23:00", "22:00", "06:00", true},
		{"overnight early morning", "03:00", "22:00", "06:00", true},
		{"overnight outside", "10:00", "22:00", "06:00", false},
		// Invalid inputs
		{"invalid now", "abc", "10:00", "14:00", false},
		{"invalid start", "12:00", "abc", "14:00", false},
		{"invalid end", "12:00", "10:00", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeInRange(tt.now, tt.start, tt.end)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTimeFormatConsistency(t *testing.T) {
	t1, err1 := parseTime("10:30")
	assert.NoError(t, err1)
	assert.Equal(t, 10, t1.Hour())
	assert.Equal(t, 30, t1.Minute())

	t2, err2 := parseTime("10:30:45")
	assert.NoError(t, err2)
	assert.Equal(t, 10, t2.Hour())
	assert.Equal(t, 30, t2.Minute())
	assert.Equal(t, 45, t2.Second())
}

func TestIsTimeInRangeWithSeconds(t *testing.T) {
	result := isTimeInRange("12:00:30", "10:00:00", "14:00:00")
	assert.True(t, result)

	result = isTimeInRange("09:00:00", "10:00:00", "14:00:00")
	assert.False(t, result)
}

func TestIsTimeInRangeEqualStartEnd(t *testing.T) {
	// When start == end, start.Before(end) is false, so it uses the "or" branch
	// now.After(start) || now.Before(end)
	// With start == end, this means now != start (since if now == start, both conditions are false)
	result := isTimeInRange("12:00", "10:00", "10:00")
	// 12:00 is after 10:00, so true
	assert.True(t, result)

	result = isTimeInRange("10:00", "10:00", "10:00")
	// 10:00 is not after 10:00 and not before 10:00, so false
	assert.False(t, result)
}

func TestParseTimeWithTimeTime(t *testing.T) {
	// Ensure parseTime returns a time.Time that can be used with Before/After
	t1, err := parseTime("10:00")
	assert.NoError(t, err)
	t2, err := parseTime("14:00")
	assert.NoError(t, err)
	assert.True(t, t1.Before(t2))
	assert.True(t, t2.After(t1))

	_ = time.Time{}
}

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
	m := &Manager{config: &config.Config{}}
	cfg := getDownloadSpeedLimitConfigByTimeState(m, timeStateUnknown)
	assert.False(t, cfg.IsOnlineSpeedLimit)
	assert.True(t, cfg.DownloadSpeedLimitEnabled)
}
