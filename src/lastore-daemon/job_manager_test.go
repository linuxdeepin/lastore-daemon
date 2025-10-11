// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"

	"github.com/stretchr/testify/assert"
)

func TestJobManager(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	option := map[string]interface{}{
		"UpdateMode":              system.SystemUpdate, // 原始mode
		"WrapperModePath":         "",
		"SupportDpkgScriptIgnore": true,
	}
	// 空包只走流程
	_, _, err := jm.CreateJob(system.DistUpgradeJobType, system.InstallJobType, nil, nil, option)
	assert.NoError(t, err)
	assert.Equal(t, jm.findJobByType(system.DistUpgradeJobType, nil), (*Job)(nil))

	_, jobDistUpgrade2, err := jm.CreateJob("", system.DistUpgradeJobType, nil, nil, option)
	assert.NoError(t, err)
	err = jm.addJob(jobDistUpgrade2)
	assert.NoError(t, err)
	err = jm.addJob(jobDistUpgrade2)
	assert.Equal(t, jm.findJobByType(system.DistUpgradeJobType, nil), jobDistUpgrade2)

	_, jobDownload, err := jm.CreateJob(system.DownloadJobType, system.DownloadJobType, nil, nil, nil)
	assert.NoError(t, err)
	err = jm.addJob(jobDownload)
	assert.NoError(t, err)
	err = jm.addJob(jobDownload)
	assert.Equal(t, jm.findJobByType(system.DownloadJobType, nil), jobDownload)

	jm.MarkStart(jobDistUpgrade2.Id)
	assert.Equal(t, jm.List().Len(), 2)

	jobDistUpgrade2.Status = system.RunningStatus
	jm.CleanJob(jobDistUpgrade2.Id)
	assert.Equal(t, jobDistUpgrade2.Status, system.RunningStatus)
	jm.removeJob(jobDownload.Id, DownloadQueue)
	assert.Equal(t, jm.List().Len(), 1)

	_, jobDownload2, err := jm.CreateJob(system.DownloadJobType, system.DownloadJobType, nil, nil, nil)
	assert.NoError(t, err)
	err = jm.addJob(jobDownload2)
	assert.NoError(t, err)
	err = jm.addJob(jobDownload2)
	assert.NoError(t, err)
	assert.Equal(t, jm.findJobByType(system.DownloadJobType, nil), jobDownload2)
	jobDownload2.Status = system.FailedStatus
	NotUseDBus = true
	err = jm.markReady(jobDownload2)
	assert.NoError(t, err)
}
