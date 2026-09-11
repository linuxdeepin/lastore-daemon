// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCfg 构造一个零值配置,避免触发任何 dsLastoreManager 分支。
func newCfg() *Cfg.Config {
	return &Cfg.Config{}
}

// redirectPostContentCacheDir 将 postContentCacheDir 指向临时目录并在测试后恢复。
func redirectPostContentCacheDir(t *testing.T) {
	t.Helper()
	old := postContentCacheDir
	postContentCacheDir = t.TempDir()
	t.Cleanup(func() { postContentCacheDir = old })
}

func TestNewUpdatePlatformManagerExtra(t *testing.T) {
	overridePathVars(t)
	redirectPostContentCacheDir(t)

	m := NewUpdatePlatformManager(newCfg(), false)
	require.NotNil(t, m)
	assert.NotNil(t, m.config)
	assert.Equal(t, UnknownUpdate, m.Tp)
}

func TestSyncRepoAndInReleaseExtra(t *testing.T) {
	overridePathVars(t)

	// 重定向 PlatFormSourceFile,避免写 /var/lib/lastore/platform.list。
	oldP := system.PlatFormSourceFile
	dir := t.TempDir()
	system.PlatFormSourceFile = filepath.Join(dir, "platform.list")
	t.Cleanup(func() { system.PlatFormSourceFile = oldP })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake InRelease"))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config: newCfg(),
		repoInfos: []repoInfo{
			{Uri: srv.URL, CodeName: "eagle"},
		},
	}
	m.SyncRepoAndInRelease(false)

	content, err := os.ReadFile(system.PlatFormSourceFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), srv.URL)
}

func TestGenUpdatePolicyByTokenDisabledExtra(t *testing.T) {
	overridePathVars(t)
	redirectPostContentCacheDir(t)

	m := &UpdatePlatformManager{
		config: &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion},
		Tp:     UnknownUpdate,
	}
	err := m.GenUpdatePolicyByToken()
	require.NoError(t, err)
	// targetBaseline 为空时 Tp 应回退为 NormalUpdate。
	assert.Equal(t, NormalUpdate, m.Tp)
	assert.False(t, m.UpdateNowForce)
}

// newPkgMetaSrv 返回一个对所有 /api/v1/* 请求返回 data:{} 的测试服务器,
// 覆盖各 *Sync 函数「空数据」执行路径而不需构造有效 JSON。
// 注意不能返回 data:null,否则 get*Data 会将指针置 nil 并返回错误。
func newPkgMetaSrv() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"result":true,"code":0,"data":{}}`)
	}))
}

func TestUpdateCurrentPreInstalledPkgMetaSyncExtra(t *testing.T) {
	overridePathVars(t)
	srv := newPkgMetaSrv()
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:       newCfg(),
		requestUrl:   srv.URL,
		preBaseline:  "base",
		arch:         "x86_64",
		BaselinePkgs: make(map[string]system.PackageInfo),
	}
	require.NoError(t, m.updateCurrentPreInstalledPkgMetaSync())
}

func TestUpdateCVEMetaDataSyncExtra(t *testing.T) {
	overridePathVars(t)
	srv := newPkgMetaSrv()
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:     newCfg(),
		requestUrl: srv.URL,
	}
	require.NoError(t, m.updateCVEMetaDataSync())
}

func TestUpdateLogMetaSyncExtra(t *testing.T) {
	overridePathVars(t)
	srv := newPkgMetaSrv()
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:     newCfg(),
		requestUrl: srv.URL,
	}
	require.NoError(t, m.updateLogMetaSync())
}

func TestUpdateAllPlatformDataSyncExtra(t *testing.T) {
	overridePathVars(t)
	srv := newPkgMetaSrv()
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:         newCfg(),
		requestUrl:     srv.URL,
		targetBaseline: "target",
		preBaseline:    "pre",
		arch:           "x86_64",
	}
	// 四个 sync 函数全部走 data:null 空数据路径,均返回 nil。
	require.NoError(t, m.UpdateAllPlatformDataSync())
}

func TestSaveCacheExtra(t *testing.T) {
	overridePathVars(t)

	m := &UpdatePlatformManager{
		TargetCorePkgs: make(map[string]system.PackageInfo),
		BaselinePkgs:   make(map[string]system.PackageInfo),
		SelectPkgs:     make(map[string]system.PackageInfo),
		FreezePkgs:     make(map[string]system.PackageInfo),
		PurgePkgs:      make(map[string]system.PackageInfo),
	}
	// 不 panic;SetOnlineCache 写 /var/lib/lastore 失败仅 logger.Warning。
	m.SaveCache(newCfg())
}
