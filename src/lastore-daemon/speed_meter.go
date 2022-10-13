// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"time"
)

type SpeedMeter struct {
	DownloadSize int64

	speed      int64
	updateTime time.Time
	startTime  time.Time

	progress float64
}

func (s *SpeedMeter) SetDownloadSize(size int64) {
	if s.DownloadSize == 0 {
		s.DownloadSize = size
	}
}

func (s *SpeedMeter) Speed(newProgress float64) int64 {
	now := time.Now()

	if s.startTime.IsZero() {
		s.startTime = now
	}

	elapsed := now.Sub(s.startTime).Seconds()
	if elapsed < 1 {
		s.updateTime = now
		s.progress = newProgress
		return 0
	}

	if now.Sub(s.updateTime).Seconds() > 3 {
		s.speed = int64(1.024 * (newProgress - s.progress) * float64(s.DownloadSize) / now.Sub(s.updateTime).Seconds())
		s.updateTime = now
		s.progress = newProgress
	}
	return s.speed
}
