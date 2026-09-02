// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpeedMeterSetDownloadSize(t *testing.T) {
	s := &SpeedMeter{}
	s.SetDownloadSize(1024)
	assert.Equal(t, int64(1024), s.DownloadSize)

	s.SetDownloadSize(2048)
	assert.Equal(t, int64(1024), s.DownloadSize, "DownloadSize should not change once set")
}

func TestSpeedMeterSpeedInitial(t *testing.T) {
	s := &SpeedMeter{DownloadSize: 1000}
	speed := s.Speed(0.5, -1)
	assert.Equal(t, int64(0), speed, "first call within 1 second should return 0")
}
