// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
)

func TestNewPeakOffPeakMonitor(t *testing.T) {
	mgr := &Manager{}
	mon := NewPeakOffPeakMonitor(mgr, "12:00:00")
	assert.NotNil(t, mon)
	assert.Equal(t, mgr, mon.manager)
	assert.Equal(t, "12:00:00", mon.serverTime)
	assert.Equal(t, 5*time.Second, mon.checkInterval)
	assert.NotNil(t, mon.done)
}

func TestIsAllDayRateLimit_True(t *testing.T) {
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	mgr.updatePlatform.OnlineRateLimit.AllDayRateLimit.Enable = true
	mon := &PeakOffPeakMonitor{manager: mgr}
	assert.True(t, mon.isAllDayRateLimit())
}

func TestIsAllDayRateLimit_False(t *testing.T) {
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	mon := &PeakOffPeakMonitor{manager: mgr}
	assert.False(t, mon.isAllDayRateLimit())
}

func TestNeedMonitor_PeakEnabled(t *testing.T) {
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	mgr.updatePlatform.OnlineRateLimit.PeakTimeRateLimit.Enable = true
	mon := &PeakOffPeakMonitor{manager: mgr}
	assert.True(t, mon.needMonitor())
}

func TestNeedMonitor_OffPeakEnabled(t *testing.T) {
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	mgr.updatePlatform.OnlineRateLimit.OffPeakTimeRateLimit.Enable = true
	mon := &PeakOffPeakMonitor{manager: mgr}
	assert.True(t, mon.needMonitor())
}

func TestNeedMonitor_NoneEnabled(t *testing.T) {
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
	}
	mon := &PeakOffPeakMonitor{manager: mgr}
	assert.False(t, mon.needMonitor())
}
