// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestTypeString(t *testing.T) {
	tests := []struct {
		rt   requestType
		want string
	}{
		{GetVersion, "GET /api/v1/version"},
		{GetUpdateLog, "GET /api/v1/systemupdatelogs"},
		{GetPkgCVEs, "GET /api/v1/cve/sync"},
		{PostProcess, "POST /api/v1/process"},
		{PostResult, "POST /api/v1/update/status"},
	}
	for _, tt := range tests {
		got := tt.rt.string()
		assert.Equal(t, tt.want, got)
	}
}

func TestUpdateTpString(t *testing.T) {
	tests := []struct {
		tp   UpdateTp
		want string
	}{
		{UnknownUpdate, "UnknownUpdate"},
		{NormalUpdate, "NormalUpdate"},
		{UpdateNow, "UpdateNow"},
		{UpdateShutdown, "UpdateShutdown"},
		{UpdateRegularly, "UpdateRegularly"},
		{UpdateTp(99), "UpdateTp(99)"},
	}
	for _, tt := range tests {
		got := tt.tp.String()
		assert.Equal(t, tt.want, got)
	}
}

func TestProcessEventTypeString(t *testing.T) {
	tests := []struct {
		pe   ProcessEventType
		want string
	}{
		{CheckEnv, "CheckEnv"},
		{GetUpdateEvent, "GetUpdateEvent"},
		{StartDownload, "StartDownload"},
		{DownloadComplete, "DownloadComplete"},
		{StartBackUp, "StartBackUp"},
		{BackUpComplete, "BackUpComplete"},
		{StartInstall, "StartInstall"},
		{ProcessEventType(99), "ProcessEventType(99)"},
	}
	for _, tt := range tests {
		got := tt.pe.String()
		assert.Equal(t, tt.want, got)
	}
}

func TestProcessEventTypeIsValid(t *testing.T) {
	assert.True(t, CheckEnv.IsValid())
	assert.True(t, StartInstall.IsValid())
	assert.True(t, (MaxProcessEventType - 1).IsValid())
	assert.False(t, ProcessEventType(0).IsValid())
	assert.False(t, MaxProcessEventType.IsValid())
	assert.False(t, ProcessEventType(99).IsValid())
}

func TestUpgradeResultString(t *testing.T) {
	tests := []struct {
		r    UpgradeResult
		want string
	}{
		{UpgradeSucceed, "UpgradeSucceed"},
		{UpgradeFailed, "UpgradeFailed"},
		{CheckFailed, "CheckFailed"},
		{UpgradeResult(99), "Unknown"},
	}
	for _, tt := range tests {
		got := tt.r.String()
		assert.Equal(t, tt.want, got)
	}
}
