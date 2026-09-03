// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJobQueue(t *testing.T) {
	q := NewJobQueue("test", 5)
	assert.Equal(t, "test", q.Name)
	assert.Equal(t, 5, q.Cap)
	assert.Empty(t, q.AllJobs())
}

func TestJobQueueAddAndFind(t *testing.T) {
	q := NewJobQueue("test", 5)
	j := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	err := q.Add(j)
	assert.NoError(t, err)
	assert.Len(t, q.AllJobs(), 1)

	found := q.Find("job1")
	assert.NotNil(t, found)
	assert.Equal(t, "job1", found.Id)

	assert.Nil(t, q.Find("nonexistent"))
}

func TestJobQueueAddNil(t *testing.T) {
	q := NewJobQueue("test", 5)
	err := q.Add(nil)
	assert.Error(t, err)
}

func TestJobQueueAddDuplicate(t *testing.T) {
	q := NewJobQueue("test", 5)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	_ = q.Add(j1)
	j2 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	err := q.Add(j2)
	assert.Error(t, err)
}

func TestJobQueueAddDuplicateSystemChange(t *testing.T) {
	q := NewJobQueue(SystemChangeQueue, 5)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	_ = q.Add(j1)
	j2 := &Job{Id: "job2", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	err := q.Add(j2)
	assert.Error(t, err)
}

func TestJobQueueAddSameTypeDifferentPackages(t *testing.T) {
	q := NewJobQueue("test", 5)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	j2 := &Job{Id: "job2", Type: system.DownloadJobType, Packages: []string{"pkg2"}}
	err := q.Add(j1)
	assert.NoError(t, err)
	err = q.Add(j2)
	assert.NoError(t, err)
	assert.Len(t, q.AllJobs(), 2)
}

func TestJobQueueRemove(t *testing.T) {
	q := NewJobQueue("test", 5)
	j := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}}
	_ = q.Add(j)

	removed, err := q.Remove("job1")
	assert.NoError(t, err)
	assert.Equal(t, "job1", removed.Id)
	assert.Empty(t, q.AllJobs())

	_, err = q.Remove("nonexistent")
	assert.Error(t, err)
}

func TestJobQueueRaise(t *testing.T) {
	q := NewJobQueue("test", 5)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}, CreateTime: 1}
	j2 := &Job{Id: "job2", Type: system.DownloadJobType, Packages: []string{"pkg2"}, CreateTime: 2}
	_ = q.Add(j1)
	_ = q.Add(j2)

	err := q.Raise("job2")
	assert.NoError(t, err)
	jobs := q.AllJobs()
	assert.Equal(t, "job2", jobs[0].Id)

	err = q.Raise("nonexistent")
	assert.Error(t, err)
}

func TestJobQueueDoneJobs(t *testing.T) {
	q := NewJobQueue("test", 5)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}, Status: system.EndStatus}
	j2 := &Job{Id: "job2", Type: system.DownloadJobType, Packages: []string{"pkg2"}, Status: system.RunningStatus}
	_ = q.Add(j1)
	_ = q.Add(j2)

	done := q.DoneJobs()
	assert.Len(t, done, 1)
	assert.Equal(t, "job1", done[0].Id)
}

func TestJobQueueRunningJobs(t *testing.T) {
	q := NewJobQueue("test", 5)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}, Status: system.RunningStatus}
	j2 := &Job{Id: "job2", Type: system.DownloadJobType, Packages: []string{"pkg2"}, Status: system.ReadyStatus}
	j3 := &Job{Id: "job3", Type: system.DownloadJobType, Packages: []string{"pkg3"}, Status: system.EndStatus}
	_ = q.Add(j1)
	_ = q.Add(j2)
	_ = q.Add(j3)

	running := q.RunningJobs()
	assert.Len(t, running, 2)
}

func TestJobQueuePendingJobs(t *testing.T) {
	q := NewJobQueue("test", 3)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}, Status: system.ReadyStatus}
	j2 := &Job{Id: "job2", Type: system.DownloadJobType, Packages: []string{"pkg2"}, Status: system.FailedStatus, retry: 1}
	j3 := &Job{Id: "job3", Type: system.DownloadJobType, Packages: []string{"pkg3"}, Status: system.RunningStatus}
	_ = q.Add(j1)
	_ = q.Add(j2)
	_ = q.Add(j3)

	pending := q.PendingJobs()
	require.NotEmpty(t, pending)
}

func TestJobQueuePendingJobsNoRetry(t *testing.T) {
	q := NewJobQueue("test", 3)
	j1 := &Job{Id: "job1", Type: system.DownloadJobType, Packages: []string{"pkg1"}, Status: system.FailedStatus, retry: 0}
	_ = q.Add(j1)

	pending := q.PendingJobs()
	assert.Empty(t, pending)
}

func TestJobListLen(t *testing.T) {
	l := JobList{&Job{Id: "1"}, &Job{Id: "2"}}
	assert.Equal(t, 2, l.Len())
	assert.Equal(t, 0, JobList{}.Len())
}

func TestJobListLess(t *testing.T) {
	l := JobList{
		{Id: "1", Type: system.UpdateSourceJobType, CreateTime: 10},
		{Id: "2", Type: system.DownloadJobType, CreateTime: 5},
	}
	assert.True(t, l.Less(0, 1))

	l2 := JobList{
		{Id: "1", Type: system.DownloadJobType, CreateTime: 1},
		{Id: "2", Type: system.DownloadJobType, CreateTime: 2},
	}
	assert.True(t, l2.Less(0, 1))
	assert.False(t, l2.Less(1, 0))
}

func TestJobListSwap(t *testing.T) {
	l := JobList{
		{Id: "1"},
		{Id: "2"},
	}
	l.Swap(0, 1)
	assert.Equal(t, "2", l[0].Id)
	assert.Equal(t, "1", l[1].Id)
}
