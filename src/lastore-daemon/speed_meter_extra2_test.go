// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpeedMeterSetDownloadSizeAlreadySet(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 1000}
	s.SetDownloadSize(2000)
	assert.Equal(t, int64(1000), s.DownloadSize)
}

func TestSpeedMeterSpeedWithDelivery(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 1000000}
	// Set startTime in the past so elapsed > 1 second
	s.startTime = time.Now().Add(-10 * time.Second)
	s.updateTime = time.Now().Add(-10 * time.Second)
	s.progress = 0.0

	// Wait a bit so now.Sub(updateTime) > 5 seconds
	time.Sleep(6 * time.Second)

	speed := s.Speed(0.5, 5000)
	assert.Equal(t, int64(5000), speed)
}

func TestSpeedMeterSpeedCalculated(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 1000000}
	s.startTime = time.Now().Add(-10 * time.Second)
	s.updateTime = time.Now().Add(-10 * time.Second)
	s.progress = 0.0

	time.Sleep(6 * time.Second)

	speed := s.Speed(0.5, -1)
	assert.True(t, speed > 0)
}

func TestSpeedMeterSpeedReturnsZeroInitially(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 1000000}
	// First call sets startTime, elapsed < 1 second
	speed := s.Speed(0.1, -1)
	assert.Equal(t, int64(0), speed)
}
