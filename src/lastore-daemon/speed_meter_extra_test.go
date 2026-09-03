// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSpeedMeterSetDownloadSize(t *testing.T) {
	s := &SpeedMeter{}
	s.SetDownloadSize(1000)
	assert.Equal(t, int64(1000), s.DownloadSize)

	// Second call should not overwrite
	s.SetDownloadSize(2000)
	assert.Equal(t, int64(1000), s.DownloadSize, "SetDownloadSize should not overwrite existing value")
}

func TestSpeedMeterInitialSpeed(t *testing.T) {
	s := &SpeedMeter{}
	speed := s.Speed(0.5, -1)
	// On first call with elapsed < 1 second, should return 0
	assert.Equal(t, int64(0), speed)
}

func TestSpeedMeterSpeedZeroDownloadSize(t *testing.T) {
	s := &SpeedMeter{}
	speed := s.Speed(0.0, -1)
	assert.Equal(t, int64(0), speed)
}

func TestSpeedMeterSpeedWithDeliverySpeed(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 10000}
	// Force startTime to be in the past
	s.startTime = time.Now().Add(-10 * time.Second)
	s.updateTime = time.Now().Add(-10 * time.Second)
	s.progress = 0.0

	// deliverySpeed >= 0 should use it
	speed := s.Speed(0.5, 5000)
	assert.Equal(t, int64(5000), speed)
}

func TestSpeedMeterSpeedCalculated2(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 100000}
	// Force startTime to be in the past
	s.startTime = time.Now().Add(-10 * time.Second)
	s.updateTime = time.Now().Add(-10 * time.Second)
	s.progress = 0.0

	// deliverySpeed < 0 should calculate from progress
	speed := s.Speed(0.5, -1)
	// speed = int64(1.024 * (0.5 - 0.0) * 100000 / elapsed_seconds)
	// elapsed is roughly 10 seconds, so speed should be around 5120
	assert.Greater(t, speed, int64(0))
}

func TestSpeedMeterSpeedNoUpdateWithin5Seconds(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 10000}
	s.startTime = time.Now().Add(-10 * time.Second)
	// updateTime is recent (within 5 seconds)
	s.updateTime = time.Now().Add(-2 * time.Second)
	s.progress = 0.3

	speed := s.Speed(0.5, -1)
	// Since now - updateTime < 5 seconds, speed should not be updated
	// s.speed is still 0 (zero value)
	assert.Equal(t, int64(0), speed)
}

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
