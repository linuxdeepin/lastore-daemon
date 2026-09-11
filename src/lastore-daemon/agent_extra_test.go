// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
	lastoreAgent "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.lastore1.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJsonExtra(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "data.json")
	data := map[string]string{"key": "value"}
	content, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fpath, content, 0644))

	var result map[string]string
	err = decodeJson(fpath, &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestDecodeJsonNonexistent(t *testing.T) {
	var result map[string]string
	err := decodeJson("/nonexistent/path/file.json", &result)
	assert.Error(t, err)
}

func TestDecodeJsonInvalid(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(fpath, []byte("not json"), 0644))

	var result map[string]string
	err := decodeJson(fpath, &result)
	assert.Error(t, err)
}

func TestUserAgentMapAddUserHasUser(t *testing.T) {
	m := newUserAgentMap()
	assert.False(t, m.hasUser("1000"))
	m.addUser("1000")
	assert.True(t, m.hasUser("1000"))
	m.addUser("1000")
	assert.True(t, m.hasUser("1000"))
}

func TestUserAgentMapAddLang(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	m.setActiveUID("1000")
	m.addLang("1000", "zh_CN.UTF-8")
	assert.Equal(t, "zh_CN.UTF-8", m.getActiveLastoreAgentLang())

	m.setActiveUID("2000")
	assert.Equal(t, "", m.getActiveLastoreAgentLang())
}

func TestUserAgentMapSetActiveUIDLang(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	m.addLang("1000", "zh_CN.UTF-8")
	m.setActiveUID("1000")
	assert.Equal(t, "zh_CN.UTF-8", m.getActiveLastoreAgentLang())

	m.setActiveUID("")
	assert.Equal(t, "", m.getActiveLastoreAgentLang())
}

func TestUserAgentMapGetActiveLastoreAgentLangNoActive(t *testing.T) {
	m := newUserAgentMap()
	assert.Equal(t, "", m.getActiveLastoreAgentLang())
}

func TestUserAgentMapGetActiveLastoreAgentLangNoItem(t *testing.T) {
	m := newUserAgentMap()
	m.setActiveUID("9999")
	assert.Equal(t, "", m.getActiveLastoreAgentLang())
}

func TestUserAgentMapGetActiveAgentNil(t *testing.T) {
	m := newUserAgentMap()
	assert.Nil(t, m.getActiveAgent("/some/path"))
}

func TestUserAgentMapGetActiveAgentNoActiveUid(t *testing.T) {
	m := newUserAgentMap()
	assert.Nil(t, m.getActiveLastoreAgent())
}

func TestUserAgentMapGetAgentsInfoEmpty(t *testing.T) {
	m := newUserAgentMap()
	info := m.getAgentsInfo()
	assert.NotNil(t, info)
	assert.Equal(t, "", info.ActiveUid)
	assert.Empty(t, info.UidInfoMap)
}

func TestUserAgentMapGetAgentsInfoWithUser(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	m.addLang("1000", "zh_CN.UTF-8")
	m.setActiveUID("1000")
	info := m.getAgentsInfo()
	assert.Equal(t, "1000", info.ActiveUid)
	assert.Contains(t, info.UidInfoMap, "1000")
	assert.Equal(t, "zh_CN.UTF-8", info.UidInfoMap["1000"].Lang)
}

func TestUserAgentMapSaveRecordContent(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	m.addLang("1000", "zh_CN.UTF-8")
	m.setActiveUID("1000")

	dir := t.TempDir()
	fpath := filepath.Join(dir, "record.json")
	m.saveRecordContent(fpath)

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var info userAgentInfoMap
	err = json.Unmarshal(data, &info)
	require.NoError(t, err)
	assert.Equal(t, "1000", info.ActiveUid)
}

func TestUserAgentMapRemoveAgentNotExist(t *testing.T) {
	m := newUserAgentMap()
	err := m.removeAgent("1000", "/some/path")
	assert.Error(t, err)
}

func TestUserAgentMapRemoveUserNotExist(t *testing.T) {
	m := newUserAgentMap()
	m.removeUser("9999")
	assert.False(t, m.hasUser("9999"))
}

func TestUserAgentMapRemoveUserExisting(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	assert.True(t, m.hasUser("1000"))
	m.removeUser("1000")
	assert.False(t, m.hasUser("1000"))
}

func TestUserAgentMapRemoveSessionNotExist(t *testing.T) {
	m := newUserAgentMap()
	m.removeSession("/some/path")
}

func TestUserAgentMapHandleNameLostEmpty(t *testing.T) {
	m := newUserAgentMap()
	m.handleNameLost("com.test.Service")
}

func newMockAgent(path dbus.ObjectPath, serviceName string) *lastoreAgent.MockAgent {
	a := &lastoreAgent.MockAgent{}
	a.MockObject.On("Path_").Return(path)
	a.MockObject.On("ServiceName_").Return(serviceName)
	return a
}

func TestUserAgentMapAddAgentNew(t *testing.T) {
	m := newUserAgentMap()
	a := newMockAgent("/agent/1", "com.test.Service")
	m.addAgent("1000", a)
	item := m.uidItemMap["1000"]
	require.NotNil(t, item)
	assert.Contains(t, item.agents, dbus.ObjectPath("/agent/1"))
}

func TestUserAgentMapAddAgentExisting(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	a := newMockAgent("/agent/1", "com.test.Service")
	m.addAgent("1000", a)
	assert.Contains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/1"))
}

func TestUserAgentMapAddAgentNilAgents(t *testing.T) {
	m := newUserAgentMap()
	m.uidItemMap["1000"] = &sessionAgentMapItem{agents: nil}
	a := newMockAgent("/agent/1", "com.test.Service")
	m.addAgent("1000", a)
	require.NotNil(t, m.uidItemMap["1000"].agents)
	assert.Contains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/1"))
}

func TestUserAgentMapAddAgentLimit(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	for i := 0; i < 11; i++ {
		m.addAgent("1000", newMockAgent(dbus.ObjectPath(fmt.Sprintf("/agent/%d", i)), "com.test.Service"))
	}
	m.addAgent("1000", newMockAgent("/agent/overflow", "com.test.Service"))
	assert.Len(t, m.uidItemMap["1000"].agents, 11)
	assert.NotContains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/overflow"))
}

func TestUserAgentMapRemoveAgentPathNotExist(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	err := m.removeAgent("1000", "/agent/nope")
	assert.Error(t, err)
}

func TestUserAgentMapRemoveAgentSuccess(t *testing.T) {
	m := newUserAgentMap()
	m.addAgent("1000", newMockAgent("/agent/1", "com.test.Service"))
	assert.Contains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/1"))
	err := m.removeAgent("1000", "/agent/1")
	assert.NoError(t, err)
	assert.NotContains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/1"))
}

func TestUserAgentMapHandleNameLost(t *testing.T) {
	m := newUserAgentMap()
	m.addAgent("1000", newMockAgent("/agent/1", "com.test.A"))
	m.addAgent("1000", newMockAgent("/agent/2", "com.test.B"))
	m.handleNameLost("com.test.A")
	assert.NotContains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/1"))
	assert.Contains(t, m.uidItemMap["1000"].agents, dbus.ObjectPath("/agent/2"))
}

func TestUserAgentMapGetActiveAgentNoItem(t *testing.T) {
	m := newUserAgentMap()
	m.setActiveUID("1000")
	assert.Nil(t, m.getActiveAgent("/agent/1"))
}

func TestUserAgentMapGetActiveAgentAbsent(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	m.setActiveUID("1000")
	assert.Nil(t, m.getActiveAgent("/agent/1"))
}

func TestUserAgentMapGetActiveAgentPresent(t *testing.T) {
	m := newUserAgentMap()
	a := newMockAgent("/agent/1", "com.test.Service")
	m.addAgent("1000", a)
	m.setActiveUID("1000")
	got := m.getActiveAgent("/agent/1")
	assert.Equal(t, lastoreAgent.Agent(a), got)
}

func TestUserAgentMapRemoveSession(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")

	s1 := &login1.MockSession{}
	s1.MockObject.On("RemoveAllHandlers").Return()
	m.uidItemMap["1000"].sessions[dbus.ObjectPath("/session/1")] = s1
	m.uidItemMap["1000"].sessions[dbus.ObjectPath("/session/2")] = nil

	m.removeSession("/session/1")
	m.removeSession("/session/2")
	assert.Empty(t, m.uidItemMap["1000"].sessions)
	s1.MockObject.AssertCalled(t, "RemoveAllHandlers")
}

func TestUserAgentMapRemoveSessionOther(t *testing.T) {
	m := newUserAgentMap()
	m.addUser("1000")
	m.uidItemMap["1000"].sessions[dbus.ObjectPath("/session/1")] = nil

	m.removeSession("/session/other")
	assert.Contains(t, m.uidItemMap["1000"].sessions, dbus.ObjectPath("/session/1"))
}

func TestUserAgentMapRecoverLastoreAgentsDecodeError(t *testing.T) {
	m := newUserAgentMap()
	old := userAgentRecordPath
	userAgentRecordPath = filepath.Join(t.TempDir(), "nonexistent.json")
	defer func() { userAgentRecordPath = old }()

	m.recoverLastoreAgents(newTestService(), func(sessionId string, sessionPath dbus.ObjectPath) {})
	assert.Nil(t, m.uidItemMap["1000"])
}
