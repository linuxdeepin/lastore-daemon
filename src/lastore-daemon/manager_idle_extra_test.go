// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCacheJobNoFile(t *testing.T) {
	// /tmp/lastoreJobCache.json is absent in a fresh test env; hits the
	// os.ReadFile error early-return.
	(&Manager{}).loadCacheJob()
}

func TestLoadCacheJobInvalidJSON(t *testing.T) {
	orig := lastoreJobCacheJson
	lastoreJobCacheJson = filepath.Join(t.TempDir(), "lastoreJobCache.json")
	t.Cleanup(func() { lastoreJobCacheJson = orig })
	require.NoError(t, os.WriteFile(lastoreJobCacheJson, []byte("{bad"), 0600))
	m := &Manager{service: newTestService(), jobManager: newTestJobManager()}
	m.loadCacheJob()
}

func TestLoadCacheJobFailedStatus(t *testing.T) {
	orig := lastoreJobCacheJson
	lastoreJobCacheJson = filepath.Join(t.TempDir(), "lastoreJobCache.json")
	t.Cleanup(func() { lastoreJobCacheJson = orig })

	jobs := []*JobContent{{
		Id:        "job-failed-1",
		Name:      "test",
		Type:      system.UpdateJobType,
		Status:    system.FailedStatus,
		QueueName: LockQueue,
		Environ:   map[string]string{},
	}}
	data, err := json.Marshal(jobs)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lastoreJobCacheJson, data, 0600))

	jm := newTestJobManager()
	m := &Manager{service: newTestService(), jobManager: jm}
	m.loadCacheJob()

	job := jm.findJobById("job-failed-1")
	require.NotNil(t, job)
	assert.Equal(t, system.FailedStatus, job.Status)
}

func TestLoadCacheJobDefaultStatus(t *testing.T) {
	orig := lastoreJobCacheJson
	lastoreJobCacheJson = filepath.Join(t.TempDir(), "lastoreJobCache.json")
	t.Cleanup(func() { lastoreJobCacheJson = orig })

	jobs := []*JobContent{{Id: "j", Name: "n", Status: system.SucceedStatus, QueueName: LockQueue}}
	data, _ := json.Marshal(jobs)
	require.NoError(t, os.WriteFile(lastoreJobCacheJson, data, 0600))

	m := &Manager{service: newTestService(), jobManager: newTestJobManager()}
	m.loadCacheJob() // SucceedStatus hits the default "continue" branch
}

func TestSaveCacheJob(t *testing.T) {
	(&Manager{}).saveCacheJob()
}

func TestLoadLastoreCache(t *testing.T) {
	// loadUpdateSourceOnce + loadAllowCaller both early-return when their
	// state files are absent.
	(&Manager{}).loadLastoreCache()
}

func TestSaveLastoreCache(t *testing.T) {
	m := &Manager{userAgents: newUserAgentMap()}
	m.saveLastoreCache()
}

func TestHandleOSSignal(t *testing.T) {
	m := &Manager{service: newTestService()}
	go m.handleOSSignal()
	// Give signal.Notify a chance to register before delivering SIGINT.
	time.Sleep(100 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	// Let the handler process the signal and call m.service.Quit().
	time.Sleep(50 * time.Millisecond)
}
