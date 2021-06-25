package main

import (
	"internal/system"
	"internal/system/apt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobManager(t *testing.T) {
	jm := NewJobManager(nil, apt.New(), nil)

	// 空包只走流程
	_, err := jm.CreateJob(system.DistUpgradeJobType, system.InstallJobType, nil, nil, 0)
	assert.Nil(t, err)
	assert.Equal(t, jm.findJobByType(system.DistUpgradeJobType, nil), (*Job)(nil))

	jobDistUpgrade2, err := jm.CreateJob("", system.DistUpgradeJobType, nil, nil, 0)
	assert.Nil(t, err)
	assert.Equal(t, jm.findJobByType(system.DistUpgradeJobType, nil), jobDistUpgrade2)

	jobDownload, err := jm.CreateJob(system.DownloadJobType, system.DownloadJobType, nil, nil, 0)
	assert.Nil(t, err)
	assert.Equal(t, jm.findJobByType(system.DownloadJobType, nil), jobDownload)

	jm.MarkStart(jobDistUpgrade2.Id)
	assert.Equal(t, jm.List().Len(), 2)

	jobDistUpgrade2.Status = system.RunningStatus
	jm.CleanJob(jobDistUpgrade2.Id)
	assert.Equal(t, jobDistUpgrade2.Status, system.RunningStatus)
	jm.removeJob(jobDownload.Id, DownloadQueue)
	assert.Equal(t, jm.List().Len(), 1)
}
