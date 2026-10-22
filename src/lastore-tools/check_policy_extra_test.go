// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenVersionResponse(t *testing.T) {
	// Force all dbus.SystemBus() calls inside getSystemInfo to fail fast,
	// avoiding a slow systemd service-activation timeout on the test host.
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/version", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Config{
		PlatformUrl:           server.URL,
		GetHardwareIdByHelper: true,
	}

	resp, err := genVersionResponse(c)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

// secureCacheDir 返回 0700 目录：生产环境缓存位于 root 独占的 /run/lastore。
func secureCacheDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lastore")
	require.NoError(t, os.Mkdir(dir, 0700))
	return dir
}

func TestCheckPolicyCacheRoundTrip(t *testing.T) {
	cacheFile := filepath.Join(secureCacheDir(t), "checkpolicy.cache")
	now := time.Now()

	require.NoError(t, writeCheckPolicyCache(cacheFile, "abc123", now))

	fi, err := os.Stat(cacheFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm())

	content, err := os.ReadFile(cacheFile)
	require.NoError(t, err)
	assert.Equal(t, "abc123\n"+now.Format(time.RFC3339)+"\n", string(content))

	sum, cacheTime := readCheckPolicyCache(cacheFile)
	assert.Equal(t, "abc123", sum)
	assert.True(t, cacheTime.Equal(now.Truncate(time.Second)))
}

// 缓存被预置为符号链接时，读取不得跟随链接，写入不得改写链接目标
func TestCheckPolicyCacheSymlinkNotFollowed(t *testing.T) {
	dir := secureCacheDir(t)
	victim := filepath.Join(dir, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("victim-data"), 0600))
	cacheFile := filepath.Join(dir, "checkpolicy.cache")
	require.NoError(t, os.Symlink(victim, cacheFile))

	sum, _ := readCheckPolicyCache(cacheFile)
	assert.Empty(t, sum)

	require.NoError(t, writeCheckPolicyCache(cacheFile, "abc123", time.Now()))
	content, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "victim-data", string(content))

	fi, err := os.Lstat(cacheFile)
	require.NoError(t, err)
	assert.True(t, fi.Mode().IsRegular())
}

// 缓存目录可被其它用户写入时拒绝落盘
func TestWriteCacheRejectsOpenDir(t *testing.T) {
	openDir := filepath.Join(t.TempDir(), "open")
	require.NoError(t, os.Mkdir(openDir, 0700))
	require.NoError(t, os.Chmod(openDir, 0777))

	openCache := filepath.Join(openDir, "checkpolicy.cache")
	assert.Error(t, writeCheckPolicyCache(openCache, "abc123", time.Now()))
	_, err := os.Stat(openCache)
	assert.True(t, os.IsNotExist(err))
}
