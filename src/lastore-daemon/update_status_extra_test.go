// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		old, new_ system.UpdateModeStatus
		want      bool
	}{
		{system.IsDownloading, system.DownloadPause, true},
		{system.NoUpdate, system.DownloadPause, false},
		{system.CanUpgrade, system.IsDownloading, false},
		{system.CanUpgrade, system.NotDownload, false},
		{system.DownloadErr, system.NotDownload, false},
		{system.Upgraded, system.NotDownload, false},
		{system.Upgraded, system.IsDownloading, false},
		{system.WaitRunUpgrade, system.NotDownload, false},
		{system.WaitRunUpgrade, system.IsDownloading, false},
		{system.UpgradeErr, system.IsDownloading, false},
		{system.UpgradeErr, system.NotDownload, false},
		{system.Upgrading, system.NotDownload, false},
		{system.Upgrading, system.IsDownloading, false},
		{system.NotDownload, system.IsDownloading, true},
		{system.NoUpdate, system.NotDownload, true},
	}
	for _, tt := range tests {
		got := canTransition(tt.old, tt.new_)
		assert.Equal(t, tt.want, got, "canTransition(%v, %v)", tt.old, tt.new_)
	}
}

func TestTransitionUpdateStatusValid(t *testing.T) {
	tests := []struct {
		old, new_ system.UpdateModeStatus
		want      bool
	}{
		// DownloadPause is only valid from IsDownloading
		{system.IsDownloading, system.DownloadPause, true},
		{system.NoUpdate, system.DownloadPause, false},
		{system.NotDownload, system.DownloadPause, false},
		{system.DownloadPause, system.DownloadPause, false},
		{system.DownloadErr, system.DownloadPause, false},
		{system.CanUpgrade, system.DownloadPause, true},
		{system.WaitRunUpgrade, system.DownloadPause, false},
		{system.Upgrading, system.DownloadPause, false},
		{system.UpgradeErr, system.DownloadPause, false},
		{system.Upgraded, system.DownloadPause, false},
		// CanUpgrade cannot transition to IsDownloading
		{system.CanUpgrade, system.IsDownloading, false},
		// Valid transitions
		{system.NoUpdate, system.NotDownload, true},
		{system.NotDownload, system.IsDownloading, true},
		{system.IsDownloading, system.CanUpgrade, true},
		{system.CanUpgrade, system.WaitRunUpgrade, true},
		{system.Upgraded, system.NoUpdate, true},
	}
	for _, tt := range tests {
		got := TransitionUpdateStatusValid(tt.old, tt.new_)
		assert.Equal(t, tt.want, got, "TransitionUpdateStatusValid(%v, %v)", tt.old, tt.new_)
	}
}

func TestFilterMode(t *testing.T) {
	updateMode := system.SystemUpdate | system.SecurityUpdate
	checkMode := system.SystemUpdate
	res0, res1 := filterMode(updateMode, checkMode)
	assert.True(t, res0&system.SystemUpdate != 0)
	assert.True(t, res1&system.SystemUpdate != 0)
}

func TestFilterModeWithOnlySecurity(t *testing.T) {
	updateMode := system.OnlySecurityUpdate
	checkMode := system.OnlySecurityUpdate
	res0, res1 := filterMode(updateMode, checkMode)
	assert.True(t, res0&system.SecurityUpdate != 0)
	assert.True(t, res1&system.SecurityUpdate != 0)
}

func TestFilterModeEmptyCheckMode(t *testing.T) {
	updateMode := system.SystemUpdate
	checkMode := system.UpdateType(0)
	res0, res1 := filterMode(updateMode, checkMode)
	assert.True(t, res0&system.SystemUpdate != 0)
	assert.Equal(t, system.UpdateType(0), res1)
}
