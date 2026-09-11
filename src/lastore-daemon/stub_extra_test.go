// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestManagerGetInterfaceName(t *testing.T) {
	m := &Manager{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Manager", m.GetInterfaceName())
}

func TestJobGetInterfaceName(t *testing.T) {
	j := &Job{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Job", j.GetInterfaceName())
}

func TestUpdaterGetInterfaceName(t *testing.T) {
	u := &Updater{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Updater", u.GetInterfaceName())
}

func TestJobGetPath(t *testing.T) {
	tests := []struct {
		id   string
		want dbus.ObjectPath
	}{
		{"/1", "/org/deepin/dde/Lastore1/Job/1"},
		{"/abc", "/org/deepin/dde/Lastore1/Job/abc"},
		{"", "/org/deepin/dde/Lastore1/Job"},
	}
	for _, tt := range tests {
		j := &Job{Id: tt.id}
		got := j.getPath()
		assert.Equal(t, tt.want, got)
	}
}

func TestUpdaterSetUpdatableApps(t *testing.T) {
	u := &Updater{service: newTestService()}

	u.setUpdatableApps([]string{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, u.UpdatableApps)

	// unchanged input must not disturb the stored slice
	u.setUpdatableApps([]string{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, u.UpdatableApps)

	u.setUpdatableApps([]string{"a"})
	assert.Equal(t, []string{"a"}, u.UpdatableApps)
}

func TestUpdaterSetUpdatablePackages(t *testing.T) {
	u := &Updater{service: newTestService()}

	u.setUpdatablePackages([]string{"p1", "p2"})
	assert.Equal(t, []string{"p1", "p2"}, u.UpdatablePackages)

	u.setUpdatablePackages([]string{"p1", "p2"})
	assert.Equal(t, []string{"p1", "p2"}, u.UpdatablePackages)

	u.setUpdatablePackages([]string{"p1"})
	assert.Equal(t, []string{"p1"}, u.UpdatablePackages)
}

func TestManagerUpdatableApps(t *testing.T) {
	m := &Manager{service: newTestService()}

	m.updatableApps([]string{"x"})
	assert.Equal(t, []string{"x"}, m.UpgradableApps)

	m.updatableApps([]string{"x"})
	assert.Equal(t, []string{"x"}, m.UpgradableApps)

	m.updatableApps([]string{"y", "z"})
	assert.Equal(t, []string{"y", "z"}, m.UpgradableApps)
}

func TestJobNotifyAll(t *testing.T) {
	j := &Job{
		service: newTestService(),
		Type:    system.DownloadJobType,
	}
	// The service has no exported object, so every emit returns an error that
	// is intentionally ignored; the call must not panic.
	assert.NotPanics(t, func() { j.notifyAll() })
}

func TestManagerUpdateJobListEmpty(t *testing.T) {
	m := &Manager{
		service:    newTestService(),
		jobManager: &JobManager{queues: map[string]*JobQueue{}},
	}
	m.updateJobList()
	assert.Empty(t, m.jobList)
	assert.False(t, m.SystemOnChanging)
}

func TestManagerUpdateJobListChanged(t *testing.T) {
	m := &Manager{
		service:    newTestService(),
		jobManager: &JobManager{queues: map[string]*JobQueue{}},
		jobList:    []*Job{{Id: "/1"}},
	}
	m.updateJobList()
	assert.Empty(t, m.jobList)
	assert.Empty(t, m.JobList)
}

func TestManagerUpdateJobListSystemOnChanging(t *testing.T) {
	oldInhibitor := inhibitorFn
	inhibitorFn = func(what, who, why string) (dbus.UnixFD, error) {
		return dbus.UnixFD(42), nil
	}
	defer func() {
		inhibitorFn = oldInhibitor
		sharedInhibitMu.Lock()
		sharedInhibitRef = 0
		sharedInhibitFd = -1
		sharedInhibitMu.Unlock()
	}()

	sharedInhibitMu.Lock()
	sharedInhibitRef = 0
	sharedInhibitFd = -1
	sharedInhibitMu.Unlock()

	q := NewJobQueue("download", 4)
	q.jobs = JobList{{Id: "/1", Type: system.DownloadJobType, Cancelable: false}}
	m := &Manager{
		service:    newTestService(),
		jobManager: &JobManager{queues: map[string]*JobQueue{"download": q}},
		inhibitFd:  -1,
	}

	m.updateJobList()
	assert.True(t, m.SystemOnChanging)
	assert.Len(t, m.JobList, 1)
}

func TestManagerUpdatableAppsSameLenDiff(t *testing.T) {
	m := &Manager{service: newTestService()}
	m.UpgradableApps = []string{"a", "b"}
	m.updatableApps([]string{"a", "c"})
	assert.Equal(t, []string{"a", "c"}, m.UpgradableApps)
}

func TestDestroyJobDBusNotUseDBus(t *testing.T) {
	j := &Job{service: newTestService()}
	DestroyJobDBus(j)
}

func TestDestroyJobDBusWithNext(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	j := &Job{service: newTestService(), next: &Job{}}
	DestroyJobDBus(j)
}

func TestDestroyJobDBusNoNext(t *testing.T) {
	old := NotUseDBus
	NotUseDBus = false
	defer func() { NotUseDBus = old }()

	j := &Job{service: newTestService()}
	assert.NotPanics(t, func() { DestroyJobDBus(j) })
}
