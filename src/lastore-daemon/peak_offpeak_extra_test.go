// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

func TestPeakOffPeakMonitorStartNeedMonitorFalse(t *testing.T) {
	mgr := &Manager{updatePlatform: &updateplatform.UpdatePlatformManager{}}
	m := &PeakOffPeakMonitor{manager: mgr, done: make(chan struct{})}

	m.Start()
	assert.True(t, m.stopped)
}

func TestPeakOffPeakMonitorStartAllDayRateLimit(t *testing.T) {
	u := &Updater{service: newTestService(), config: newTestConfig(t)}
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
		updater:        u,
		jobManager:     &JobManager{queues: map[string]*JobQueue{}},
		config:         newTestConfig(t),
	}
	u.manager = mgr
	mgr.updatePlatform.OnlineRateLimit.AllDayRateLimit.Enable = true

	m := &PeakOffPeakMonitor{manager: mgr, done: make(chan struct{}), serverTime: "10:00:00"}
	m.Start()
	assert.True(t, m.stopped)
	if u.setDownloadSpeedLimitTimer != nil {
		u.setDownloadSpeedLimitTimer.Stop()
	}
}

func TestPeakOffPeakMonitorStopAlreadyStopped(t *testing.T) {
	m := &PeakOffPeakMonitor{stopped: true}
	m.Stop()
	assert.True(t, m.stopped)
}

func TestPeakOffPeakMonitorStop(t *testing.T) {
	m := &PeakOffPeakMonitor{done: make(chan struct{})}
	m.Stop()
	assert.True(t, m.stopped)
}

func TestPeakOffPeakMonitorRun(t *testing.T) {
	mgr := &Manager{updatePlatform: &updateplatform.UpdatePlatformManager{}}
	m := &PeakOffPeakMonitor{
		manager:       mgr,
		done:          make(chan struct{}),
		checkInterval: 5 * time.Millisecond,
		serverTime:    "10:00:00",
		startTime:     time.Now(),
	}
	m.wg.Add(1)
	done := make(chan struct{})
	go func() {
		m.run()
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let the ticker fire and exercise refresh
	close(m.done)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not terminate after done was closed")
	}
}

func TestPeakOffPeakMonitorRefreshNoChange(t *testing.T) {
	mgr := &Manager{updatePlatform: &updateplatform.UpdatePlatformManager{}}
	m := &PeakOffPeakMonitor{
		manager:       mgr,
		lastTimeState: 0,
		startTime:     time.Now(),
		serverTime:    "10:00:00",
	}
	m.refresh()
	assert.Equal(t, 0, m.lastTimeState)
}

func TestPeakOffPeakMonitorRefreshStateChanged(t *testing.T) {
	u := &Updater{service: newTestService(), config: newTestConfig(t)}
	mgr := &Manager{
		updatePlatform: &updateplatform.UpdatePlatformManager{},
		updater:        u,
		jobManager:     &JobManager{queues: map[string]*JobQueue{}},
		config:         newTestConfig(t),
	}
	u.manager = mgr

	m := &PeakOffPeakMonitor{
		manager:       mgr,
		lastTimeState: 1, // peak
		startTime:     time.Now(),
		serverTime:    "10:00:00",
	}
	m.refresh()
	assert.Equal(t, 0, m.lastTimeState)
	if u.setDownloadSpeedLimitTimer != nil {
		u.setDownloadSpeedLimitTimer.Stop()
	}
}
