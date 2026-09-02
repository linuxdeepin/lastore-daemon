// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
