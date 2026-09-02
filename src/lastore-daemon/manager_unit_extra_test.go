// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestGetNextAutoCheckDelay_IntranetFirstRun(t *testing.T) {
	m := &Manager{
		config:                  config.NewConfig(""),
		isAutoCheckTimerFirstRun: true,
	}
	m.config.IntranetUpdate = true
	m.config.StartCheckRange = []int{100, 200}

	delay := m.getNextAutoCheckDelay()
	assert.GreaterOrEqual(t, delay, 100)
	assert.Less(t, delay, 200)
}

func TestGetNextAutoCheckDelay_IntranetNotFirstRun(t *testing.T) {
	m := &Manager{
		config:                  config.NewConfig(""),
		isAutoCheckTimerFirstRun: false,
	}
	m.config.IntranetUpdate = true
	m.config.CheckInterval = 5 * time.Minute

	delay := m.getNextAutoCheckDelay()
	assert.Equal(t, int(5*time.Minute/time.Second), delay)
}

func TestGetNextAutoCheckDelay_IntranetNotFirstRunNegative(t *testing.T) {
	m := &Manager{
		config:                  config.NewConfig(""),
		isAutoCheckTimerFirstRun: false,
	}
	m.config.IntranetUpdate = true
	m.config.CheckInterval = -1

	delay := m.getNextAutoCheckDelay()
	assert.Equal(t, 0, delay)
}

func TestGetNextAutoCheckDelay_NotIntranet(t *testing.T) {
	m := &Manager{
		config:                  config.NewConfig(""),
		isAutoCheckTimerFirstRun: true,
	}
	m.config.IntranetUpdate = false
	m.config.StartCheckRange = []int{0, 100}

	delay := m.getNextAutoCheckDelay()
	assert.GreaterOrEqual(t, delay, 0)
}
