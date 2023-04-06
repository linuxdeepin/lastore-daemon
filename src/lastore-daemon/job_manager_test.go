// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobManager(t *testing.T) {
	jm := NewJobManager(nil, apt.New(), nil)

	// 空包只走流程
	_, err := jm.CreateJob(system.DistUpgradeJobType, system.InstallJobType, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, jm.findJobByType(system.DistUpgradeJobType, nil), (*Job)(nil))

	jobDistUpgrade2, err := jm.CreateJob("", system.DistUpgradeJobType, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, jm.findJobByType(system.DistUpgradeJobType, nil), jobDistUpgrade2)

	jobDownload, err := jm.CreateJob(system.DownloadJobType, system.DownloadJobType, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, jm.findJobByType(system.DownloadJobType, nil), jobDownload)

	jm.MarkStart(jobDistUpgrade2.Id)
	assert.Equal(t, jm.List().Len(), 2)

	jobDistUpgrade2.Status = system.RunningStatus
	jm.CleanJob(jobDistUpgrade2.Id)
	assert.Equal(t, jobDistUpgrade2.Status, system.RunningStatus)
	jm.removeJob(jobDownload.Id, DownloadQueue)
	assert.Equal(t, jm.List().Len(), 1)

	jobDownload2, err := jm.CreateJob(system.DownloadJobType, system.DownloadJobType, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, jm.findJobByType(system.DownloadJobType, nil), jobDownload2)
	jobDownload2.Status = system.FailedStatus
	NotUseDBus = true
	err = jm.markReady(jobDownload2)
	assert.NoError(t, err)
}

func Test_GetUpgradeInfoMap(t *testing.T) {
	upgradeInfoMap := GetUpgradeInfoMap()
	_, ok := upgradeInfoMap[system.SystemUpgradeJobType]
	assert.Equal(t, true, ok)
	_, ok = upgradeInfoMap[system.AppStoreUpgradeJobType]
	assert.Equal(t, true, ok)
	_, ok = upgradeInfoMap[system.UnknownUpgradeJobType]
	assert.Equal(t, true, ok)
	_, ok = upgradeInfoMap[system.SecurityUpgradeJobType]
	assert.Equal(t, true, ok)
}
