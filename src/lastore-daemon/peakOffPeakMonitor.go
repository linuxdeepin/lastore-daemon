// SPDX-FileCopyrightText: 2024 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sync"
	"time"
)

type PeakOffPeakMonitor struct {
	manager       *Manager
	done          chan struct{}
	wg            sync.WaitGroup
	startTime     time.Time
	serverTime    string
	checkInterval time.Duration
	stopped       bool
	stopMu        sync.Mutex
	lastTimeState int
}

func NewPeakOffPeakMonitor(m *Manager, serverTime string) *PeakOffPeakMonitor {
	return &PeakOffPeakMonitor{
		manager:       m,
		done:          make(chan struct{}),
		startTime:     time.Now(),
		serverTime:    serverTime,
		checkInterval: 5 * time.Second,
	}
}

func (m *PeakOffPeakMonitor) Start() {
	if m.isAllDayRateLimit() {
		logger.Info("all-day rate limit enabled, apply and skip monitor")
		m.manager.setEffectiveOnlineRateLimit(m.serverTime)
		m.stopMu.Lock()
		m.stopped = true
		m.stopMu.Unlock()
		return
	}

	if !m.needMonitor() {
		logger.Info("peak/off-peak rate limit not enabled, skip starting monitor")
		m.stopMu.Lock()
		m.stopped = true
		m.stopMu.Unlock()
		return
	}

	m.lastTimeState = m.manager.getCurrentTimeState(m.getCurrentTime())
	logger.Infof("peak/off-peak monitor started, initial time state: %d", m.lastTimeState)
	m.manager.setEffectiveOnlineRateLimit(m.serverTime)

	m.wg.Add(1)
	go m.run()
}

func (m *PeakOffPeakMonitor) Stop() {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()
	if m.stopped {
		return
	}
	m.stopped = true
	close(m.done)
	m.wg.Wait()
	logger.Info("peak/off-peak monitor stopped")
}

func (m *PeakOffPeakMonitor) isAllDayRateLimit() bool {
	return m.manager.updatePlatform.OnlineRateLimit.AllDayRateLimit.Enable
}

func (m *PeakOffPeakMonitor) needMonitor() bool {
	onlineRateLimit := m.manager.updatePlatform.OnlineRateLimit
	return onlineRateLimit.PeakTimeRateLimit.Enable || onlineRateLimit.OffPeakTimeRateLimit.Enable
}

func (m *PeakOffPeakMonitor) run() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.refresh()
		}
	}
}

func (m *PeakOffPeakMonitor) refresh() {
	nowTime := m.getCurrentTime()
	currentState := m.manager.getCurrentTimeState(nowTime)

	if currentState != m.lastTimeState {
		logger.Infof("time state changed: %d -> %d, refreshing rate limit", m.lastTimeState, currentState)
		m.manager.setEffectiveOnlineRateLimit(nowTime)
		m.lastTimeState = currentState
	}
}

func (m *PeakOffPeakMonitor) getCurrentTime() string {
	layout := "15:04:05"
	downloadStartServiceTime, err := time.ParseInLocation(layout, m.serverTime, time.Local)
	if err != nil {
		logger.Warningf("format server time failed: %v", err)
		return time.Now().Format(layout)
	}
	now := time.Now()
	nowTime := downloadStartServiceTime.Add(now.Sub(m.startTime))
	return nowTime.Format(layout)
}
