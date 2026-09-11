// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"

	lastoreAgent "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.lastore1.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStatusManager(t *testing.T) *UpdateModeStatusManager {
	t.Helper()
	sm := NewStatusManager(newTestConfig(t), nil)
	sm.InitModifyData()
	return sm
}

func TestAgentRecoverLastoreAgentsNoRecord(t *testing.T) {
	// Redirect the record path to an empty temp dir so the "no record" branch
	// is deterministic regardless of whether a live daemon has written
	// /run/lastore/lastoreAgentCache on the host.
	prevRecordPath := userAgentRecordPath
	userAgentRecordPath = filepath.Join(t.TempDir(), "lastoreAgentCache")
	t.Cleanup(func() {
		userAgentRecordPath = prevRecordPath
	})

	m := newUserAgentMap()
	m.recoverLastoreAgents(newTestService(), nil)
}

func TestAgentAddAgent(t *testing.T) {
	agent, err := lastoreAgent.NewAgent(newFailingConnService(t).Conn(), "org.deepin.dde.Lastore1.Agent", lastoreAgentPath)
	require.NoError(t, err)
	m := newUserAgentMap()
	m.addAgent("1000", agent)
	assert.True(t, m.hasUser("1000"))
}

func TestAgentAddSession(t *testing.T) {
	sess, err := login1.NewSession(newFailingConnService(t).Conn(), "/org/freedesktop/login1/session/_31")
	require.NoError(t, err)
	m := newUserAgentMap()
	assert.True(t, m.addSession("1000", sess))
	assert.False(t, m.addSession("1000", sess))
}

func TestMakeEnvironWithSenderFailingConn(t *testing.T) {
	m := &Manager{userAgents: newUserAgentMap(), service: newFailingConnService(t)}
	_, err := makeEnvironWithSender(m, ifcTestSender)
	assert.Error(t, err)
}

func TestDownloadAndDecompressCoreList(t *testing.T) {
	_, _ = downloadAndDecompressCoreList()
}

func TestGetCoreListFromCacheMissing(t *testing.T) {
	assert.Nil(t, getCoreListFromCache())
}

func TestGetCoreListOnline(t *testing.T) {
	// May return a list if the core-list package is downloadable on the host;
	// only exercise the code path.
	_ = getCoreListOnline()
}

func TestInhibitor(t *testing.T) {
	fd, err := Inhibitor("shutdown:sleep", dbusServiceName, "unit test")
	if err == nil {
		require.NoError(t, closeInhibitFd(fd))
	}
}

func TestManagerGetExportedMethods(t *testing.T) {
	assert.NotEmpty(t, (&Manager{}).GetExportedMethods())
}

func TestUpdaterGetExportedMethods(t *testing.T) {
	assert.NotEmpty(t, (&Updater{}).GetExportedMethods())
}

func TestStartSystemJobUnknownType(t *testing.T) {
	j := NewJob(nil, "job-unknown", "unknown", nil, "unknown-type", DownloadQueue, nil)
	err := StartSystemJob(nil, j)
	assert.Error(t, err)
}

func TestUpdateUpdatableAppsEmpty(t *testing.T) {
	u := &Updater{}
	u.updateUpdatableApps()
}

func TestUpdateModeAllStatusBySize(t *testing.T) {
	sm := newTestStatusManager(t)
	sm.UpdateModeAllStatusBySize(nil)
}
