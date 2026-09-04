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

func TestMarkStart_UnknownQueue(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.ReadyStatus, queueName: "nonexistent"}
	err := jm.markStart(job)
	assert.Error(t, err)
}

func TestMarkStart_TransitionError(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.EndStatus, queueName: DownloadQueue}
	err := jm.markStart(job)
	assert.Error(t, err)
}

func TestMarkStart_UnknownJobId(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	err := jm.MarkStart("missing-id")
	assert.Error(t, err)
}

func TestCleanJob_UnknownJobId(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	err := jm.CleanJob("missing-id")
	assert.Error(t, err)
}

func TestPauseJob_PausedNoop(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.PausedStatus, queueName: DownloadQueue}
	err := jm.pauseJob(job)
	assert.NoError(t, err)
	assert.Equal(t, system.PausedStatus, job.Status)
}

func TestPauseJob_RunningAbortError(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.RunningStatus, queueName: DownloadQueue}
	err := jm.pauseJob(job)
	assert.Error(t, err)
}

func TestPauseJob_ReadyTransitions(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.ReadyStatus, queueName: DownloadQueue}
	err := jm.pauseJob(job)
	assert.NoError(t, err)
	assert.Equal(t, system.PausedStatus, job.Status)
}

func TestAddJob_Nil(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	err := jm.addJob(nil)
	assert.Error(t, err)
}

func TestAddJob_UnknownQueue(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.ReadyStatus, queueName: "nonexistent"}
	err := jm.addJob(job)
	assert.Error(t, err)
}

func TestRemoveJob_UnknownQueue(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	err := jm.removeJob("job-1", "nonexistent")
	assert.Error(t, err)
}

func TestRemoveJob_NotFound(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	err := jm.removeJob("missing-id", DownloadQueue)
	assert.Error(t, err)
}

func TestFindJobByType_Next(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	parent := &Job{Id: "parent", Type: system.DownloadJobType, Status: system.ReadyStatus, queueName: DownloadQueue}
	child := &Job{Id: "child", Type: system.InstallJobType, Packages: []string{"pkg1"}, Status: system.ReadyStatus}
	parent.next = child
	assert.NoError(t, jm.addJob(parent))

	got := jm.findJobByType(system.InstallJobType, []string{"pkg1"})
	assert.Equal(t, parent, got)
}

func TestMarkReady_TransitionError(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.EndStatus, queueName: DownloadQueue}
	err := jm.markReady(job)
	assert.Error(t, err)
}

func TestCleanJob_EndTransition(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.ReadyStatus, Cancelable: false, queueName: DownloadQueue}
	assert.NoError(t, jm.addJob(job))
	err := jm.CleanJob("job-1")
	assert.NoError(t, err)
	assert.Equal(t, system.EndStatus, job.Status)
}

func TestAddJob_UpdateSourceBusy(t *testing.T) {
	jm := NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
	runJob := &Job{Id: "run-1", Type: system.DownloadJobType, Status: system.RunningStatus, queueName: DownloadQueue}
	assert.NoError(t, jm.addJob(runJob))

	usJob := &Job{Id: genJobId(system.UpdateSourceJobType), Type: system.UpdateSourceJobType, Status: system.ReadyStatus, queueName: DownloadQueue}
	err := jm.addJob(usJob)
	assert.Error(t, err)
}

func TestAddJob_ExportError(t *testing.T) {
	oldNotUseDBus := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = oldNotUseDBus }()

	jm := NewJobManager(newTestService(), apt.NewSystem(nil, nil, false), nil, nil)
	job := &Job{Id: "bad id", Type: system.InstallJobType, Status: system.ReadyStatus, queueName: DownloadQueue}
	err := jm.addJob(job)
	assert.Error(t, err)
}
