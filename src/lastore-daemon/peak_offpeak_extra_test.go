// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetCurrentTimeValidServerTime(t *testing.T) {
	m := &PeakOffPeakMonitor{
		startTime:  time.Now(),
		serverTime: "10:00:00",
	}

	result := m.getCurrentTime()
	assert.NotEmpty(t, result)

	// Should be in format "15:04:05"
	_, err := time.Parse("15:04:05", result)
	assert.NoError(t, err)
}

func TestGetCurrentTimeInvalidServerTime(t *testing.T) {
	m := &PeakOffPeakMonitor{
		startTime:  time.Now(),
		serverTime: "invalid",
	}

	result := m.getCurrentTime()
	assert.NotEmpty(t, result)

	// Should fall back to current time format
	_, err := time.Parse("15:04:05", result)
	assert.NoError(t, err)
}

func TestGetCurrentTimeEmptyServerTime(t *testing.T) {
	m := &PeakOffPeakMonitor{
		startTime:  time.Now(),
		serverTime: "",
	}

	result := m.getCurrentTime()
	assert.NotEmpty(t, result)

	_, err := time.Parse("15:04:05", result)
	assert.NoError(t, err)
}

func TestGetCurrentTimeProgression(t *testing.T) {
	// Set startTime in the past to verify time progression
	m := &PeakOffPeakMonitor{
		startTime:  time.Now().Add(-10 * time.Second),
		serverTime: "10:00:00",
	}

	result := m.getCurrentTime()
	assert.NotEmpty(t, result)

	parsed, err := time.Parse("15:04:05", result)
	assert.NoError(t, err)
	// The result should be around 10:00:10 (serverTime + 10 seconds elapsed)
	// Just verify it's after 10:00:00
	tenOClock, _ := time.Parse("15:04:05", "10:00:00")
	assert.True(t, parsed.After(tenOClock) || parsed.Equal(tenOClock))
}
