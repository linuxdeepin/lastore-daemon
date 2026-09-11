// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJobManager() *JobManager {
	return NewJobManager(nil, apt.NewSystem(nil, nil, false), nil, nil)
}

func TestJobManagerPauseJobNotFound(t *testing.T) {
	jm := newTestJobManager()
	assert.Error(t, jm.PauseJob("nonexistent"))
}

func TestJobManagerForceAbortAndRetryIdleJob(t *testing.T) {
	jm := newTestJobManager()
	assert.NoError(t, jm.ForceAbortAndRetry(&Job{Status: system.ReadyStatus}))
}

func TestJobManagerDispatchEmptyQueues(t *testing.T) {
	jm := newTestJobManager()
	jm.dispatch()
}

func TestJobManagerSendNotifyNil(t *testing.T) {
	jm := newTestJobManager()
	jm.sendNotify()
}

func TestJobManagerStartJobsInQueueNotUseDBus(t *testing.T) {
	jm := newTestJobManager()
	jm.startJobsInQueue(nil)
}

func TestJobManagerDispatchLoop(t *testing.T) {
	jm := newTestJobManager()
	go jm.Dispatch()
	time.Sleep(10 * time.Millisecond)
}

func TestJobManagerHandleJobProgressInfoNotFound(t *testing.T) {
	jm := newTestJobManager()
	jm.handleJobProgressInfo(system.JobProgressInfo{})
}

func TestJobManagerHandleDeliveryDownloadInfoNotFound(t *testing.T) {
	jm := newTestJobManager()
	jm.handleDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{})
}

func TestJobManagerCreateJobTypes(t *testing.T) {
	jm := newTestJobManager()
	cases := []struct {
		name      string
		jobType   string
		wantType  string
		wantQueue string
	}{
		{"download", system.DownloadJobType, system.DownloadJobType, DownloadQueue},
		{"prepare_system", system.PrepareSystemUpgradeJobType, system.PrepareDistUpgradeJobType, DownloadQueue},
		{"prepare_appstore", system.PrepareAppStoreUpgradeJobType, system.PrepareDistUpgradeJobType, DownloadQueue},
		{"prepare_security", system.PrepareSecurityUpgradeJobType, system.PrepareDistUpgradeJobType, DownloadQueue},
		{"prepare_unknown", system.PrepareUnknownUpgradeJobType, system.PrepareDistUpgradeJobType, DownloadQueue},
		{"only_install", system.OnlyInstallJobType, system.InstallJobType, DelayLockQueue},
		{"remove", system.RemoveJobType, system.RemoveJobType, SystemChangeQueue},
		{"update_source", system.UpdateSourceJobType, system.UpdateSourceJobType, LockQueue},
		{"update", system.UpdateJobType, system.UpdateJobType, SystemChangeQueue},
		{"clean", system.CleanJobType, system.CleanJobType, LockQueue},
		{"fix_error", system.FixErrorJobType, system.FixErrorJobType, LockQueue},
		{"system_upgrade", system.SystemUpgradeJobType, system.InstallJobType, LockQueue},
		{"security_upgrade", system.SecurityUpgradeJobType, system.InstallJobType, LockQueue},
		{"unknown_upgrade", system.UnknownUpgradeJobType, system.InstallJobType, LockQueue},
		{"appstore_upgrade", system.AppStoreUpgradeJobType, system.InstallJobType, LockQueue},
		{"check_system", system.CheckSystemJobType, system.CheckSystemJobType, SystemChangeQueue},
		{"backup", system.BackupJobType, system.BackupJobType, LockQueue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, job, err := jm.CreateJob("", c.jobType, nil, nil, nil)
			assert.NoError(t, err)
			assert.False(t, ok)
			require.NotNil(t, job)
			assert.Equal(t, c.wantType, job.Type)
			assert.Equal(t, c.wantQueue, job.queueName)
		})
	}
}

func TestJobManagerCreateJobUnsupported(t *testing.T) {
	jm := newTestJobManager()
	ok, job, err := jm.CreateJob("", "bogus_type", nil, nil, nil)
	assert.False(t, ok)
	assert.Nil(t, job)
	assert.Equal(t, system.NotSupportError, err)
}

func TestJobManagerCreateJobDuplicate(t *testing.T) {
	jm := newTestJobManager()
	_, job, err := jm.CreateJob("", system.DownloadJobType, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, jm.addJob(job))

	ok, got, err := jm.CreateJob("", system.DownloadJobType, nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, job, got)
}

func TestJobManagerCreateJobDuplicateFailed(t *testing.T) {
	jm := newTestJobManager()
	_, job, err := jm.CreateJob("", system.DownloadJobType, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, jm.addJob(job))
	job.Status = system.FailedStatus

	ok, got, err := jm.CreateJob("", system.DownloadJobType, nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, job, got)
	assert.Equal(t, system.ReadyStatus, job.Status)
}

func TestJobManagerCreateJobPrepareDistUpgrade(t *testing.T) {
	jm := newTestJobManager()
	argc := map[string]interface{}{
		"UpdateMode":   system.SystemUpdate,
		"DownloadSize": map[string]float64{"system_upgrade": 100.0},
		"PackageMap":   map[string][]string{"system_upgrade": {"pkg1"}},
	}
	ok, job, err := jm.CreateJob("", system.PrepareDistUpgradeJobType, nil, nil, argc)
	assert.NoError(t, err)
	assert.False(t, ok)
	require.NotNil(t, job)
	assert.Equal(t, system.PrepareDistUpgradeJobType, job.Type)
}

func TestJobManagerCreateJobPrepareDistUpgradeNoPackages(t *testing.T) {
	jm := newTestJobManager()
	argc := map[string]interface{}{
		"UpdateMode":   system.SystemUpdate,
		"DownloadSize": map[string]float64{},
		"PackageMap":   map[string][]string{},
	}
	ok, job, err := jm.CreateJob("", system.PrepareDistUpgradeJobType, nil, nil, argc)
	assert.False(t, ok)
	assert.Nil(t, job)
	assert.Error(t, err)
}

func TestJobManagerCreateJobDistUpgradeWithUnknown(t *testing.T) {
	jm := newTestJobManager()
	argc := map[string]interface{}{
		"UpdateMode":              system.SystemUpdate | system.UnknownUpdate,
		"SupportDpkgScriptIgnore": true,
	}
	ok, job, err := jm.CreateJob("", system.DistUpgradeJobType, nil, nil, argc)
	assert.NoError(t, err)
	assert.False(t, ok)
	require.NotNil(t, job)
	assert.NotNil(t, job.next)
}

func TestJobManagerCreateJobDistUpgradeInvalidMode(t *testing.T) {
	jm := newTestJobManager()
	_, job, err := jm.CreateJob("", system.DistUpgradeJobType, nil, nil, map[string]interface{}{
		"UpdateMode": "not-a-mode",
	})
	assert.Error(t, err)
	assert.Nil(t, job)
}

func TestJobManagerPauseJobReady(t *testing.T) {
	jm := newTestJobManager()
	job := &Job{Id: "job-1", Type: system.InstallJobType, Status: system.ReadyStatus, queueName: DownloadQueue}
	require.NoError(t, jm.addJob(job))

	require.NoError(t, jm.PauseJob("job-1"))
	assert.Equal(t, system.PausedStatus, job.Status)
}

func TestJobManagerForceAbortAndRetryRunning(t *testing.T) {
	jm := newTestJobManager()
	job := &Job{Id: "job-1", Status: system.RunningStatus, retry: 1}
	err := jm.ForceAbortAndRetry(job)
	assert.Error(t, err)
}

func TestJobManagerForceAbortAndRetryRunningZeroRetry(t *testing.T) {
	jm := newTestJobManager()
	job := &Job{Id: "job-1", Status: system.RunningStatus, retry: 0}
	_ = jm.ForceAbortAndRetry(job)
	assert.Equal(t, 1, job.retry)
}

func TestJobManagerForceAbortAndRetryFailedZeroRetry(t *testing.T) {
	jm := newTestJobManager()
	job := &Job{Id: "job-1", Status: system.FailedStatus, retry: 0}
	require.NoError(t, jm.ForceAbortAndRetry(job))
	assert.Equal(t, 1, job.retry)
	assert.True(t, jm.DownloadLimitOnChanging)
}

func TestJobManagerForceAbortAndRetryFailed(t *testing.T) {
	jm := newTestJobManager()
	job := &Job{Id: "job-1", Status: system.FailedStatus, retry: 1}
	require.NoError(t, jm.ForceAbortAndRetry(job))
	assert.Equal(t, 1, job.retry)
	assert.True(t, jm.DownloadLimitOnChanging)
}

func TestJobManagerDispatchEndJob(t *testing.T) {
	jm := newTestJobManager()
	job := &Job{Id: "job-1", Type: system.DownloadJobType, Status: system.EndStatus, queueName: DownloadQueue}
	require.NoError(t, jm.addJob(job))

	jm.dispatch()
	assert.Nil(t, jm.findJobById("job-1"))
}

func TestJobManagerDispatchEndJobWithNext(t *testing.T) {
	jm := newTestJobManager()
	child := NewJob(newTestService(), "child", "", nil, system.InstallJobType, SystemChangeQueue, nil)
	parent := &Job{
		Id:        "parent",
		Type:      system.DownloadJobType,
		Status:    system.EndStatus,
		queueName: DownloadQueue,
		option:    map[string]string{aptHttpLimitKey: "100"},
		next:      child,
	}
	require.NoError(t, jm.addJob(parent))

	jm.dispatch()
	assert.Nil(t, jm.findJobById("parent"))
	// child should be added to its queue and inherit the limit option
	assert.Equal(t, "100", child.option[aptHttpLimitKey])
	assert.Equal(t, system.ReadyStatus, child.Status)
}

func TestJobManagerDispatchLockQueueBusy(t *testing.T) {
	jm := newTestJobManager()
	lockJob := &Job{Id: "lock-1", Type: system.CleanJobType, Status: system.RunningStatus, queueName: LockQueue}
	require.NoError(t, jm.addJob(lockJob))

	jm.dispatch()
}

func TestJobManagerSendNotifyCalled(t *testing.T) {
	jm := newTestJobManager()
	called := false
	jm.notify = func() { called = true }
	jm.changed = true
	jm.sendNotify()
	assert.True(t, called)
	assert.False(t, jm.changed)
}

func TestJobManagerSendNotifyNotChanged(t *testing.T) {
	jm := newTestJobManager()
	called := false
	jm.notify = func() { called = true }
	jm.changed = false
	jm.sendNotify()
	assert.False(t, called)
}

func TestJobManagerStartJobsInQueueSuccess(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	jm := newTestJobManager()
	jm.system = &fakeSystem{}
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.queues[DownloadQueue].Add(job))

	jm.startJobsInQueue(jm.queues[DownloadQueue])
	assert.Equal(t, system.RunningStatus, job.Status)
}

func TestJobManagerStartJobsInQueueFailedRetry(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	jm := newTestJobManager()
	jm.system = &fakeSystem{}
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	job.Status = system.FailedStatus
	job.retry = 1
	require.NoError(t, jm.queues[DownloadQueue].Add(job))

	jm.startJobsInQueue(jm.queues[DownloadQueue])
	assert.Equal(t, system.RunningStatus, job.Status)
	assert.Equal(t, 0, job.retry)
}

func TestJobManagerStartJobsInQueueJobError(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	je := &system.JobError{ErrType: system.ErrorUnknown, ErrDetail: "boom", ErrorLog: []string{"log"}}
	jm := newTestJobManager()
	jm.system = &fakeSystem{err: je}
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.queues[DownloadQueue].Add(job))

	jm.startJobsInQueue(jm.queues[DownloadQueue])
	assert.Equal(t, system.FailedStatus, job.Status)
	assert.Equal(t, 0, job.retry)
	assert.Equal(t, []string{"log"}, job.errLogPath)
}

func TestJobManagerStartJobsInQueueNonJobErrorZeroRetry(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	jm := newTestJobManager()
	jm.system = &fakeSystem{err: errors.New("plain error")}
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	job.retry = 0
	require.NoError(t, jm.queues[DownloadQueue].Add(job))

	jm.startJobsInQueue(jm.queues[DownloadQueue])
	assert.Equal(t, system.FailedStatus, job.Status)
	assert.NotEmpty(t, job.Description)
}

func TestJobManagerStartJobsInQueueNonJobErrorRetry(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	jm := newTestJobManager()
	jm.system = &fakeSystem{err: errors.New("plain error")}
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	job.retry = 1
	require.NoError(t, jm.queues[DownloadQueue].Add(job))

	jm.startJobsInQueue(jm.queues[DownloadQueue])
	assert.Equal(t, system.FailedStatus, job.Status)
}

func TestJobManagerHandleJobProgressInfoWithLog(t *testing.T) {
	jm := newTestJobManager()
	captured := ""
	jm.jobDetailFn = func(msg string) { captured = msg }
	jm.handleJobProgressInfo(system.JobProgressInfo{OriginalLog: "hello"})
	assert.Contains(t, captured, "hello")
}

func TestJobManagerHandleJobProgressInfoOnlyLog(t *testing.T) {
	jm := newTestJobManager()
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.addJob(job))

	jm.handleJobProgressInfo(system.JobProgressInfo{JobId: "job-1", OnlyLog: true})
	assert.Equal(t, system.ReadyStatus, job.Status)
}

func TestJobManagerHandleJobProgressInfoUpdates(t *testing.T) {
	jm := newTestJobManager()
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.addJob(job))

	jm.handleJobProgressInfo(system.JobProgressInfo{
		JobId:       "job-1",
		Status:      system.ReadyStatus,
		Cancelable:  true,
		Description: "desc",
	})
	assert.Equal(t, "desc", job.Description)
}

func TestJobManagerHandleDeliveryDownloadInfoUpdate(t *testing.T) {
	jm := newTestJobManager()
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.addJob(job))

	jm.handleDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{JobId: "job-1", Speed: 100, Proto: "http"})
	assert.Equal(t, int64(100), job.DeliverySpeed)
}

func TestJobManagerHandleDeliveryDownloadInfoResetLimit(t *testing.T) {
	jm := newTestJobManager()
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.addJob(job))
	jm.DownloadLimitOnChanging = true

	jm.handleDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{JobId: "job-1", Speed: 100, Proto: "http"})
	assert.False(t, jm.DownloadLimitOnChanging)
}

func TestJobManagerHandleDeliveryDownloadInfoEmpty(t *testing.T) {
	jm := newTestJobManager()
	job := NewJob(newTestService(), "job-1", "", nil, system.DownloadJobType, DownloadQueue, nil)
	require.NoError(t, jm.addJob(job))

	jm.handleDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{JobId: "job-1"})
	assert.Equal(t, int64(-1), job.DeliverySpeed)
}
