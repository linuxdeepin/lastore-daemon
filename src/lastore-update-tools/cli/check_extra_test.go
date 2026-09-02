// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
)

func TestExecuteCheckNil(t *testing.T) {
	err := executeCheck(nil)
	assert.Error(t, err)
}

func TestPreUpdateCheck(t *testing.T) {
	err := PreUpdateCheck()
	assert.NoError(t, err)
}

func TestPostUpdateCheck(t *testing.T) {
	err := PostUpdateCheck()
	assert.NoError(t, err)
}

func TestPreDownloadCheck(t *testing.T) {
	err := PreDownloadCheck()
	assert.NoError(t, err)
}

func TestPostDownloadCheck(t *testing.T) {
	err := PostDownloadCheck()
	assert.NoError(t, err)
}

func TestPreBackupCheck(t *testing.T) {
	err := PreBackupCheck()
	assert.NoError(t, err)
}

func TestPostBackupCheck(t *testing.T) {
	err := PostBackupCheck()
	assert.NoError(t, err)
}

func TestUpdatePostCheckStageStage1(t *testing.T) {
	PostCheckStage1 = true
	defer func() { PostCheckStage1 = false }()

	ThisCacheInfo = &cache.CacheInfo{}
	defer func() { ThisCacheInfo = nil }()

	updatePostCheckStage(cache.P_Run)
	assert.Equal(t, cache.P_Run, ThisCacheInfo.InternalState.IsPostCheckStage1)
}

func TestUpdatePostCheckStageStage2(t *testing.T) {
	PostCheckStage1 = false

	ThisCacheInfo = &cache.CacheInfo{}
	defer func() { ThisCacheInfo = nil }()

	updatePostCheckStage(cache.P_Run)
	assert.Equal(t, cache.P_Run, ThisCacheInfo.InternalState.IsPostCheckStage2)
}
