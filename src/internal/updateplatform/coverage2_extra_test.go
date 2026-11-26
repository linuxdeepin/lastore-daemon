// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
)

func TestIsMajorUpgradeWithCacheVersionFile(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(CacheVersion, []byte("SystemName=Deepin\nProductType=Professional\nEditionName=Professional\nMajorVersion=25\nMinorVersion=25.1\nOsBuild=1234\n"), 0644))

	m := &UpdatePlatformManager{targetVersion: "26.0"}
	assert.True(t, m.IsMajorUpgrade())

	m2 := &UpdatePlatformManager{targetVersion: "25.2"}
	assert.False(t, m2.IsMajorUpgrade())
}

func TestIsMajorUpgradeParseError(t *testing.T) {
	assert.False(t, isMajorUpgrade("abc", "25.1"))
	assert.False(t, isMajorUpgrade("25.1", "xyz"))
	assert.True(t, isMajorUpgrade("25", "26"))
	assert.False(t, isMajorUpgrade("26", "25"))
	assert.False(t, isMajorUpgrade("25", "25"))
}

func TestGenRepositoryFromPlatformWriteError(t *testing.T) {
	old := system.PlatFormSourceFile
	system.PlatFormSourceFile = filepath.Join(t.TempDir(), "nonexistent-dir", "platform.list")
	t.Cleanup(func() { system.PlatFormSourceFile = old })

	m := &UpdatePlatformManager{
		config:    &Cfg.Config{PlatformRepoComponents: "main"},
		repoInfos: []repoInfo{{Uri: "http://example.com/repo", CodeName: "deepin"}},
	}
	assert.NotPanics(t, func() { m.genRepositoryFromPlatform(false) })
}

func TestPrepareCheckScriptsDecodeError(t *testing.T) {
	oldBase := check.CheckBaseDir
	check.CheckBaseDir = filepath.Join(t.TempDir(), "check") + "/"
	t.Cleanup(func() { check.CheckBaseDir = oldBase })

	m := &UpdatePlatformManager{
		PreUpgradeCheck: []ShellCheck{
			{Name: "bad.sh", Shell: "!!!invalid-base64!!!"},
		},
	}
	m.PrepareCheckScripts()
	assert.NoFileExists(t, filepath.Join(check.CheckBaseDir, "pre_upgrade_check", "bad.sh"))
}

func TestPrepareCheckScriptsAllEmpty(t *testing.T) {
	oldBase := check.CheckBaseDir
	check.CheckBaseDir = filepath.Join(t.TempDir(), "check") + "/"
	t.Cleanup(func() { check.CheckBaseDir = oldBase })

	m := &UpdatePlatformManager{}
	m.PrepareCheckScripts()
}

func TestNeedPostSystemUpgradeMessageNoFile(t *testing.T) {
	overridePathVars(t)
	m := &UpdatePlatformManager{
		config:                            &Cfg.Config{AllowPostSystemUpgradeMessageVersion: []string{"Professional"}},
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
	}
	assert.False(t, m.needPostSystemUpgradeMessage(system.SystemUpdate))
}

func TestNeedPostSystemUpgradeMessageMatch(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(CacheVersion, []byte("SystemName=Deepin\nProductType=Professional\nEditionName=Professional\nMajorVersion=25\nMinorVersion=25.1\nOsBuild=1234\n"), 0644))

	m := &UpdatePlatformManager{
		config:                            &Cfg.Config{AllowPostSystemUpgradeMessageVersion: []string{"Professional"}},
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
	}
	assert.True(t, m.needPostSystemUpgradeMessage(system.SystemUpdate))
}

func TestNeedPostSystemUpgradeMessageNoEditionMatch(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(CacheVersion, []byte("SystemName=Deepin\nProductType=Community\nEditionName=Community\nMajorVersion=25\nMinorVersion=25.1\nOsBuild=1234\n"), 0644))

	m := &UpdatePlatformManager{
		config:                            &Cfg.Config{AllowPostSystemUpgradeMessageVersion: []string{"Professional"}},
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
	}
	assert.False(t, m.needPostSystemUpgradeMessage(system.SystemUpdate))
}

func TestNeedPostSystemUpgradeMessageModeMismatch(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(CacheVersion, []byte("SystemName=Deepin\nProductType=Professional\nEditionName=Professional\nMajorVersion=25\nMinorVersion=25.1\nOsBuild=1234\n"), 0644))

	m := &UpdatePlatformManager{
		config:                            &Cfg.Config{AllowPostSystemUpgradeMessageVersion: []string{"Professional"}},
		allowPostSystemUpgradeMessageType: system.SecurityUpdate,
	}
	assert.False(t, m.needPostSystemUpgradeMessage(system.SystemUpdate))
}

func TestEncryptMsgRandError(t *testing.T) {
	old := randRead
	randRead = func(b []byte) (int, error) {
		return 0, errors.New("random source unavailable")
	}
	t.Cleanup(func() { randRead = old })

	_, err := EncryptMsg([]byte("test data"))
	assert.Error(t, err)
}

func TestEncryptMsgSuccess(t *testing.T) {
	result, err := EncryptMsg([]byte("test data"))
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestPKCS7EncodeExactBlock(t *testing.T) {
	data := make([]byte, BlockSize)
	encoded := PKCS7Encode(data, BlockSize)
	assert.Equal(t, BlockSize*2, len(encoded))
}

func TestPKCS7EncodePartial(t *testing.T) {
	data := []byte("hello")
	encoded := PKCS7Encode(data, BlockSize)
	assert.Equal(t, BlockSize, len(encoded))
}

func TestCheckInReleaseFromPlatformOKResponse(t *testing.T) {
	overridePathVars(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("InRelease content"))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: srv.URL, CodeName: "deepin"}},
	}
	assert.NotPanics(t, func() { m.checkInReleaseFromPlatform() })
}

func TestPostSystemUpgradeMessageNotFoundInMap(t *testing.T) {
	m := &UpdatePlatformManager{config: &Cfg.Config{}}
	assert.NotPanics(t, func() { m.PostSystemUpgradeMessage("nonexistent") })
}

func TestPostSystemUpgradeMessageSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		config:     &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"uuid-1": {Uuid: "uuid-1", PostStatus: WaitPost},
		},
	}
	assert.NotPanics(t, func() { m.PostSystemUpgradeMessage("uuid-1") })
}

func TestPostSystemUpgradeMessageRequestError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		config:     &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"uuid-1": {Uuid: "uuid-1", PostStatus: WaitPost},
		},
	}
	assert.NotPanics(t, func() { m.PostSystemUpgradeMessage("uuid-1") })
}

func TestNewUpdatePlatformManagerBasic(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realBaseline, []byte("[General]\nBaseline=2500\nSystemType=Professional\n"), 0644))
	require.NoError(t, os.WriteFile(realVersion, []byte("[Version]\nMajorVersion=25\n"), 0644))

	c := &Cfg.Config{}
	m := NewUpdatePlatformManager(c, false)
	require.NotNil(t, m)
}

func TestNewUpdatePlatformManagerWithPlatformUrl(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realBaseline, []byte("[General]\nBaseline=2500\n"), 0644))
	require.NoError(t, os.WriteFile(realVersion, []byte("[Version]\n"), 0644))

	c := &Cfg.Config{PlatformUrl: "http://example.com"}
	m := NewUpdatePlatformManager(c, false)
	require.NotNil(t, m)
	assert.Equal(t, "http://example.com", m.requestUrl)
}

func TestNewUpdatePlatformManagerWithOnlineCache(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realBaseline, []byte("[General]\n"), 0644))
	require.NoError(t, os.WriteFile(realVersion, []byte("[Version]\n"), 0644))

	cacheContent := platformCacheContent{
		CoreListPkgs: map[string]system.PackageInfo{"pkg1": {Name: "pkg1"}},
	}
	cacheJSON, _ := json.Marshal(cacheContent)
	c := &Cfg.Config{OnlineCache: string(cacheJSON)}
	m := NewUpdatePlatformManager(c, false)
	require.NotNil(t, m)
	assert.NotNil(t, m.TargetCorePkgs)
}

func TestNewUpdatePlatformManagerBadOnlineCache(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realBaseline, []byte("[General]\n"), 0644))
	require.NoError(t, os.WriteFile(realVersion, []byte("[Version]\n"), 0644))

	c := &Cfg.Config{OnlineCache: "invalid json"}
	m := NewUpdatePlatformManager(c, false)
	require.NotNil(t, m)
}

func TestReplaceVersionCacheNotSymlink(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(CacheVersion, []byte("content"), 0644))
	require.NoError(t, os.WriteFile(realVersion, []byte("real"), 0644))

	m := &UpdatePlatformManager{}
	m.ReplaceVersionCache()
	data, _ := os.ReadFile(CacheVersion)
	assert.Equal(t, "content", string(data))
}

func TestGetHardwareIdByHelper(t *testing.T) {
	_ = GetHardwareId(false, true)
}

func TestGetCVEUpdateLogsBasic(t *testing.T) {
	oldCVEs := CVEs
	CVEs = map[string]CEVInfo{
		"CVE-2024-0001": {CveId: "CVE-2024-0001"},
		"CVE-2024-0002": {CveId: "CVE-2024-0002"},
	}
	t.Cleanup(func() { CVEs = oldCVEs })

	m := &UpdatePlatformManager{
		cvePkgs: map[string][]string{"pkg1": {"CVE-2024-0001", "CVE-2024-0002"}},
	}
	result := m.GetCVEUpdateLogs([]string{"pkg1"})
	assert.Len(t, result, 2)
	assert.Contains(t, result, "CVE-2024-0001")

	result2 := m.GetCVEUpdateLogs([]string{"unknown"})
	assert.Empty(t, result2)
}

func TestPrepareCheckScriptsWithWriteError(t *testing.T) {
	oldBase := check.CheckBaseDir
	parentFile := filepath.Join(t.TempDir(), "afile")
	require.NoError(t, os.WriteFile(parentFile, []byte("x"), 0644))
	check.CheckBaseDir = filepath.Join(parentFile, "subdir") + "/"
	t.Cleanup(func() { check.CheckBaseDir = oldBase })

	shell := base64.RawStdEncoding.EncodeToString([]byte("#!/bin/sh\necho hello\n"))
	m := &UpdatePlatformManager{
		PreUpgradeCheck: []ShellCheck{{Name: "test.sh", Shell: shell}},
	}
	m.PrepareCheckScripts()
}
