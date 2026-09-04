// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
)

func TestBuildProgress(t *testing.T) {
	tests := []struct {
		p, begin, end, want float64
	}{
		{0, 0.1, 0.5, 0.1},
		{1, 0.1, 0.5, 0.5},
		{0.5, 0.0, 1.0, 0.5},
		{0.25, 0.2, 0.6, 0.3},
	}
	for _, tt := range tests {
		got := buildProgress(tt.p, tt.begin, tt.end)
		assert.InDelta(t, tt.want, got, 1e-9)
	}
}

func TestNormalizeDownloadProto(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"http", "http"},
		{"HTTP", "http"},
		{"https", "http"},
		{"HTTPS", "http"},
		{"p2p", "delivery"},
		{"P2P", "delivery"},
		{"delivery", "delivery"},
		{"Delivery", "delivery"},
		{"ftp", "ftp"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, normalizeDownloadProto(tt.input))
	}
}

func TestIsDownloadProtocolJob(t *testing.T) {
	assert.True(t, isDownloadProtocolJob(system.DownloadJobType))
	assert.True(t, isDownloadProtocolJob(system.PrepareDistUpgradeJobType))
	assert.True(t, isDownloadProtocolJob(system.IncrementalDownloadJobType))
	assert.False(t, isDownloadProtocolJob(system.UpdateSourceJobType))
	assert.False(t, isDownloadProtocolJob("install"))
	assert.False(t, isDownloadProtocolJob(""))
}

func TestJobHasStatus(t *testing.T) {
	j := &Job{Status: system.ReadyStatus}
	assert.True(t, j.HasStatus(system.ReadyStatus))
	assert.False(t, j.HasStatus(system.RunningStatus))
}

func TestJobSubRetryCount(t *testing.T) {
	j := &Job{retry: 5}
	j.subRetryCount(true)
	assert.Equal(t, 0, j.retry)

	j.retry = 3
	j.subRetryCount(false)
	assert.Equal(t, 2, j.retry)
}

func TestJobSubRetryCountWithHook(t *testing.T) {
	called := false
	j := &Job{retry: 3}
	j.subRetryHookFn = func(j *Job) { called = true }
	j.subRetryCount(false)
	assert.True(t, called)
	assert.Equal(t, 2, j.retry)
}

func TestJobPreHooks(t *testing.T) {
	j := &Job{}
	assert.Nil(t, j.getPreHook("run"))

	called := false
	j.setPreHooks(map[string]func() error{
		"run": func() error { called = true; return nil },
	})
	fn := j.getPreHook("run")
	assert.NotNil(t, fn)
	err := fn()
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestJobPreHooksWrap(t *testing.T) {
	j := &Job{}
	order := []string{}
	j.setPreHooks(map[string]func() error{
		"run": func() error { order = append(order, "first"); return nil },
	})
	j.wrapPreHooks(map[string]func() error{
		"run": func() error { order = append(order, "second"); return nil },
	})
	fn := j.getPreHook("run")
	err := fn()
	assert.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestJobPreHooksWrapError(t *testing.T) {
	j := &Job{}
	errFirst := errors.New("first error")
	j.setPreHooks(map[string]func() error{
		"run": func() error { return errFirst },
	})
	j.wrapPreHooks(map[string]func() error{
		"run": func() error { return nil },
	})
	fn := j.getPreHook("run")
	err := fn()
	assert.Equal(t, errFirst, err)
}

func TestJobPreHooksWrapOnEmpty(t *testing.T) {
	j := &Job{}
	called := false
	j.wrapPreHooks(map[string]func() error{
		"run": func() error { called = true; return nil },
	})
	fn := j.getPreHook("run")
	assert.NotNil(t, fn)
	_ = fn()
	assert.True(t, called)
}

func TestJobAfterHooks(t *testing.T) {
	j := &Job{}
	assert.Nil(t, j.getAfterHook("success"))

	called := false
	j.setAfterHooks(map[string]func() error{
		"success": func() error { called = true; return nil },
	})
	fn := j.getAfterHook("success")
	assert.NotNil(t, fn)
	_ = fn()
	assert.True(t, called)
}

func TestJobAfterHooksWrap(t *testing.T) {
	j := &Job{}
	order := []string{}
	j.setAfterHooks(map[string]func() error{
		"success": func() error { order = append(order, "first"); return nil },
	})
	j.wrapAfterHooks(map[string]func() error{
		"success": func() error { order = append(order, "second"); return nil },
	})
	fn := j.getAfterHook("success")
	_ = fn()
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestJobAfterHooksWrapOnEmpty(t *testing.T) {
	j := &Job{}
	called := false
	j.wrapAfterHooks(map[string]func() error{
		"success": func() error { called = true; return nil },
	})
	fn := j.getAfterHook("success")
	assert.NotNil(t, fn)
	_ = fn()
	assert.True(t, called)
}

func TestJobAfterHooksWrapError(t *testing.T) {
	j := &Job{}
	errFirst := errors.New("after first error")
	j.setAfterHooks(map[string]func() error{
		"success": func() error { return errFirst },
	})
	j.wrapAfterHooks(map[string]func() error{
		"success": func() error { return nil },
	})
	fn := j.getAfterHook("success")
	err := fn()
	assert.Equal(t, errFirst, err)
}

func TestJobInitDownloadSize(t *testing.T) {
	j := &Job{service: newTestService()}
	j.initDownloadSize(1024.0)
	assert.Equal(t, int64(1024), j.DownloadSize)

	// second call should not overwrite existing non-zero size
	j.initDownloadSize(2048.0)
	assert.Equal(t, int64(1024), j.DownloadSize)
}

func TestJobInitDownloadSize_ZeroInitial(t *testing.T) {
	j := &Job{service: newTestService()}
	j.initDownloadSize(0)
	assert.Equal(t, int64(0), j.DownloadSize)
}

func TestJobSetUpdatePolicy(t *testing.T) {
	j := &Job{service: newTestService()}
	j.setUpdatePolicy(updateplatform.UpdateTp(2))
	assert.Equal(t, 2, j.PolicyTyp)

	// set same value — no change
	j.setUpdatePolicy(updateplatform.UpdateTp(2))
	assert.Equal(t, 2, j.PolicyTyp)

	j.setUpdatePolicy(updateplatform.UpdateTp(5))
	assert.Equal(t, 5, j.PolicyTyp)
}

func TestJobSetError(t *testing.T) {
	j := &Job{service: newTestService(), Description: "old"}
	je := &system.JobError{}
	j.setError(je)
	assert.NotEqual(t, "old", j.Description)
}

func TestJobUpdateInfo_SetError(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	je := &system.JobError{
		ErrType:      system.ErrorUnknown,
		ErrDetail:    "some detail",
		IsCheckError: true,
		ErrorLog:     []string{"log line 1", "log line 2"},
	}
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     j.Status,
		Cancelable: j.Cancelable,
		Error:      je,
	})
	assert.True(t, changed)
	assert.Equal(t, []string{"log line 1", "log line 2"}, j.errLogPath)

	var got system.JobError
	assert.NoError(t, json.Unmarshal([]byte(j.Description), &got))
	assert.Equal(t, je.ErrType, got.ErrType)
	assert.Equal(t, je.ErrDetail, got.ErrDetail)
	assert.True(t, got.IsCheckError)
	assert.Equal(t, []string{"log line 1", "log line 2"}, got.ErrorLog)
}

func TestJobUpdateInfo_DescriptionChange(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:       "id",
		Status:      j.Status,
		Cancelable:  j.Cancelable,
		Description: "new description",
	})
	assert.True(t, changed)
	assert.Equal(t, "new description", j.Description)
}

func TestJobUpdateInfo_CancelableChange(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     j.Status,
		Cancelable: false,
	})
	assert.True(t, changed)
	assert.False(t, j.Cancelable)
}

func TestJobUpdateInfo_ResetProgress(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	j.Progress = 0.5
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:         "id",
		Status:        j.Status,
		Cancelable:    j.Cancelable,
		ResetProgress: true,
	})
	assert.True(t, changed)
	assert.Equal(t, 0.0, j.Progress)
}

func TestJobUpdateInfo_RunningHook(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	called := false
	j.realRunningHookFn = func() { called = true }
	j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     system.RunningStatus,
		Cancelable: j.Cancelable,
	})
	assert.True(t, called)
}

func TestJobUpdateInfo_FatalError(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	assert.Equal(t, 1, j.retry)
	j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     j.Status,
		Cancelable: j.Cancelable,
		FatalError: true,
	})
	assert.Equal(t, 0, j.retry)
}

func TestJobUpdateInfo_ProtoFlip(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.DownloadJobType, "", nil)
	j.Proto = "p2p"
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     j.Status,
		Cancelable: j.Cancelable,
	})
	assert.True(t, changed)
	assert.Equal(t, "http", j.Proto)
}

func TestJobUpdateInfo_SucceedHookError(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	j.Status = system.RunningStatus
	j.Cancelable = false
	je := &system.JobError{ErrType: system.ErrorUnknown, ErrDetail: "hook err", ErrorLog: []string{"hook log"}}
	j.setPreHooks(map[string]func() error{
		string(system.SucceedStatus): func() error { return je },
	})
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     system.SucceedStatus,
		Cancelable: false,
	})
	assert.True(t, changed)
	assert.Equal(t, system.FailedStatus, j.Status)
	assert.True(t, j.Cancelable)
	assert.Equal(t, []string{"hook log"}, j.errLogPath)
}

func TestJobUpdateInfo_StatusTransition(t *testing.T) {
	j := NewJob(newTestService(), "id", "name", nil, system.InstallJobType, "", nil)
	changed := j.updateInfo(system.JobProgressInfo{
		JobId:      "id",
		Status:     system.RunningStatus,
		Cancelable: j.Cancelable,
	})
	assert.True(t, changed)
	assert.Equal(t, system.RunningStatus, j.Status)
}

func TestJobInitProgressRange(t *testing.T) {
	j := &Job{service: newTestService()}
	j._InitProgressRange(0.1, 0.5)
	assert.Equal(t, 0.1, j.progressRangeBegin)
	assert.Equal(t, 0.5, j.progressRangeEnd)
	assert.Equal(t, 0.1, j.Progress)
}

func TestJobInitProgressRange_Panics(t *testing.T) {
	cases := []struct {
		name       string
		begin, end float64
	}{
		{"end equals begin", 0.5, 0.5},
		{"end less than begin", 0.5, 0.4},
		{"range greater than one", 0.0, 1.5},
		{"begin greater than one", 1.5, 1.6},
		{"end greater than one", 0.5, 1.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			j := &Job{service: newTestService()}
			assert.Panics(t, func() { j._InitProgressRange(c.begin, c.end) })
		})
	}

	t.Run("progress nonzero", func(t *testing.T) {
		j := &Job{service: newTestService(), Progress: 0.5}
		assert.Panics(t, func() { j._InitProgressRange(0, 1) })
	})
}

func TestJobSetErrorRoundTrip(t *testing.T) {
	j := &Job{service: newTestService(), Description: "old"}
	je := &system.JobError{
		ErrType:      system.ErrorUnknown,
		ErrDetail:    "detail",
		IsCheckError: true,
		ErrorLog:     []string{"a", "b"},
	}
	j.setError(je)
	var got system.JobError
	assert.NoError(t, json.Unmarshal([]byte(j.Description), &got))
	assert.Equal(t, je.ErrType, got.ErrType)
	assert.Equal(t, je.ErrDetail, got.ErrDetail)
	assert.True(t, got.IsCheckError)
	assert.Equal(t, []string{"a", "b"}, got.ErrorLog)
}
