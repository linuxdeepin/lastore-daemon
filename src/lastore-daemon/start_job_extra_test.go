// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

// fakeSystem is a minimal, controllable system.System used to exercise
// StartSystemJob and startJobsInQueue without touching apt/systemd/dbus.
type fakeSystem struct {
	err            error
	abortErr       error
	abortFailedErr error
	calledMethod   string
}

func (f *fakeSystem) DownloadPackages(jobId string, packages []string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "DownloadPackages"
	return f.err
}

func (f *fakeSystem) DownloadSource(jobId string, packages []string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "DownloadSource"
	return f.err
}

func (f *fakeSystem) Install(jobId string, packages []string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "Install"
	return f.err
}

func (f *fakeSystem) Remove(jobId string, packages []string, environ map[string]string) error {
	f.calledMethod = "Remove"
	return f.err
}

func (f *fakeSystem) DistUpgrade(jobId string, packages []string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "DistUpgrade"
	return f.err
}

func (f *fakeSystem) UpdateSource(jobId string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "UpdateSource"
	return f.err
}

func (f *fakeSystem) Clean(jobId string) error {
	f.calledMethod = "Clean"
	return f.err
}

func (f *fakeSystem) Abort(jobId string) error {
	f.calledMethod = "Abort"
	return f.abortErr
}

func (f *fakeSystem) AbortWithFailed(jobId string) error {
	f.calledMethod = "AbortWithFailed"
	return f.abortFailedErr
}

func (f *fakeSystem) AttachIndicator(system.Indicator) {}

func (f *fakeSystem) AttachDeliveryIndicator(system.DeliveryIndicator) {}

func (f *fakeSystem) FixError(jobId string, errType string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "FixError"
	return f.err
}

func (f *fakeSystem) OsBackup(jobId string) error {
	f.calledMethod = "OsBackup"
	return f.err
}

func (f *fakeSystem) CheckSystem(jobId string, checkType string, environ map[string]string, cmdArgs map[string]string) error {
	f.calledMethod = "CheckSystem"
	return f.err
}

func TestStartSystemJobNilPanics(t *testing.T) {
	assert.Panics(t, func() { _ = StartSystemJob(&fakeSystem{}, nil) })
}

func TestStartSystemJobTransitionError(t *testing.T) {
	sys := &fakeSystem{}
	j := &Job{Id: "id", Type: system.DownloadJobType, Status: system.EndStatus}
	err := StartSystemJob(sys, j)
	assert.Error(t, err)
	assert.Empty(t, sys.calledMethod)
}

func TestStartSystemJobDispatch(t *testing.T) {
	cases := []struct {
		name       string
		jobType    string
		wantMethod string
	}{
		{"download", system.DownloadJobType, "DownloadPackages"},
		{"prepare_dist", system.PrepareDistUpgradeJobType, "DownloadSource"},
		{"install", system.InstallJobType, "Install"},
		{"dist_upgrade", system.DistUpgradeJobType, "DistUpgrade"},
		{"remove", system.RemoveJobType, "Remove"},
		{"update_source", system.UpdateSourceJobType, "UpdateSource"},
		{"update", system.UpdateJobType, "Install"},
		{"clean", system.CleanJobType, "Clean"},
		{"fix_error", system.FixErrorJobType, "FixError"},
		{"check_system", system.CheckSystemJobType, "CheckSystem"},
		{"backup", system.BackupJobType, "OsBackup"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sys := &fakeSystem{}
			j := &Job{Id: "id", Type: c.jobType, Status: system.ReadyStatus}
			err := StartSystemJob(sys, j)
			assert.NoError(t, err)
			assert.Equal(t, c.wantMethod, sys.calledMethod)
			assert.Equal(t, system.RunningStatus, j.Status)
		})
	}
}

func TestStartSystemJobUnknownTypeNoDispatch(t *testing.T) {
	sys := &fakeSystem{}
	j := &Job{Id: "id", Type: "bogus", Status: system.ReadyStatus}
	err := StartSystemJob(sys, j)
	assert.Error(t, err)
	assert.Empty(t, sys.calledMethod)
}

func TestTransitionJobStateInvalid(t *testing.T) {
	j := &Job{Id: "id", Status: system.EndStatus}
	err := TransitionJobState(j, system.RunningStatus)
	assert.Error(t, err)
	assert.Equal(t, system.EndStatus, j.Status)
}

func TestTransitionJobStateValid(t *testing.T) {
	j := &Job{Id: "id", Status: system.ReadyStatus}
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.RunningStatus)
	assert.NoError(t, err)
	assert.Equal(t, system.RunningStatus, j.Status)
}

func TestTransitionJobStateFailedWithRetry(t *testing.T) {
	j := &Job{Id: "id", Status: system.RunningStatus, retry: 1}
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.FailedStatus)
	assert.NoError(t, err)
	assert.Equal(t, system.FailedStatus, j.Status)
}

func TestTransitionJobStatePreHookError(t *testing.T) {
	j := &Job{Id: "id", Status: system.ReadyStatus}
	hookErr := errors.New("pre boom")
	j.setPreHooks(map[string]func() error{
		string(system.RunningStatus): func() error { return hookErr },
	})
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.RunningStatus)
	assert.Equal(t, hookErr, err)
	assert.Equal(t, system.ReadyStatus, j.Status)
}

func TestTransitionJobStatePreHookSuccess(t *testing.T) {
	j := &Job{Id: "id", Status: system.ReadyStatus}
	called := false
	j.setPreHooks(map[string]func() error{
		string(system.RunningStatus): func() error { called = true; return nil },
	})
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.RunningStatus)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, system.RunningStatus, j.Status)
}

func TestTransitionJobStateSucceedRecursesToEnd(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	j := &Job{Id: "id", service: newTestService(), Status: system.RunningStatus}
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.SucceedStatus)
	assert.NoError(t, err)
	assert.Equal(t, system.EndStatus, j.Status)
}

func TestTransitionJobStateAfterHookError(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	j := &Job{Id: "id", service: newTestService(), Status: system.ReadyStatus}
	hookErr := errors.New("after boom")
	j.setAfterHooks(map[string]func() error{
		string(system.RunningStatus): func() error { return hookErr },
	})
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.RunningStatus)
	assert.Equal(t, hookErr, err)
	assert.Equal(t, system.RunningStatus, j.Status)
}

func TestTransitionJobStateInhibitSignal(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	j := &Job{Id: "id", service: newTestService(), Status: system.RunningStatus}
	j.next = &Job{}
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	err := TransitionJobState(j, system.SucceedStatus)
	assert.NoError(t, err)
	assert.Equal(t, system.EndStatus, j.Status)
}
