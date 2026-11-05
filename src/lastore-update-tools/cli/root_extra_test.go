// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorError(t *testing.T) {
	tests := []struct {
		err  Error
		want string
	}{
		{Error{Code: 1, Ext: 0, Msg: "not found"}, "Code: 1, Ext: 0, Msg: not found"},
		{Error{Code: 500, Ext: 3, Msg: "internal error"}, "Code: 500, Ext: 3, Msg: internal error"},
		{Error{Code: 0, Ext: 0, Msg: ""}, "Code: 0, Ext: 0, Msg: "},
	}
	for _, tt := range tests {
		got := tt.err.Error()
		assert.Equal(t, tt.want, got)
	}
}

func TestInitCheckEnvError(t *testing.T) {
	old, oldCfg, oldRoot := UpdateMetaConfigPath, ConfigCfg, RootCoreConfig
	ConfigCfg = filepath.Join(t.TempDir(), "config.yaml")
	UpdateMetaConfigPath = ""
	defer func() { UpdateMetaConfigPath, ConfigCfg, RootCoreConfig = old, oldCfg, oldRoot }()

	err := initCheckEnv()
	require.Error(t, err)

	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
}

func TestInitCheckEnvLoadConfigError(t *testing.T) {
	oldCfg, oldPath, oldRoot := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig
	t.Cleanup(func() {
		ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
	})

	ConfigCfg = filepath.Join(t.TempDir(), "invalid.yaml")
	require.NoError(t, os.WriteFile(ConfigCfg, []byte("Base: [invalid"), 0644))
	UpdateMetaConfigPath = "/tmp/whatever.json"

	err := initCheckEnv()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
	assert.Contains(t, jobErr.ErrDetail, "load config failed")
}

func TestInitCheckEnvRestoresMissingConfig(t *testing.T) {
	for _, missingDir := range []bool{false, true} {
		t.Run(fmt.Sprintf("missingDir=%t", missingDir), func(t *testing.T) {
			oldCfg, oldPath, oldRoot, oldCache, oldSysPkg := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig, ThisCacheInfo, SysPkgInfo
			oldTimeout := check.DynHookTimeout
			t.Cleanup(func() {
				ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
				ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
				check.DynHookTimeout = oldTimeout
			})
			dir := t.TempDir()
			ConfigCfg = filepath.Join(dir, "config.yaml")
			if missingDir {
				ConfigCfg = filepath.Join(dir, "check", "config.yaml")
			}
			UpdateMetaConfigPath = filepath.Join(dir, "meta.json")
			require.NoError(t, os.WriteFile(UpdateMetaConfigPath, []byte(`{"UUID":"test-uuid"}`), 0644))
			RootCoreConfig = config.CoreConfig{Base: "/stale", DynHookTimeout: 1}
			require.NoError(t, initCheckEnv())
			assert.Equal(t, "/var/lib/lastore/check//test-uuid", ThisCacheInfo.WorkStation)
			assert.Equal(t, 1800, check.DynHookTimeout)
			var loaded config.CoreConfig
			require.NoError(t, loaded.LoaderCfg(ConfigCfg))
			assert.Equal(t, RootCoreConfig, loaded)
		})
	}
}

func TestInitCheckEnvEmptyMetaPath(t *testing.T) {
	oldCfg, oldPath, oldRoot := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig
	t.Cleanup(func() {
		ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
	})

	ConfigCfg = filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(ConfigCfg, []byte("Base: /tmp/test\n"), 0644))
	UpdateMetaConfigPath = ""

	err := initCheckEnv()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
	assert.Contains(t, jobErr.ErrDetail, "empty")
}

func TestInitCheckEnvMetaFileNotExist(t *testing.T) {
	oldCfg, oldPath, oldRoot := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig
	t.Cleanup(func() {
		ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
	})

	ConfigCfg = filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(ConfigCfg, []byte("Base: /tmp/test\n"), 0644))
	UpdateMetaConfigPath = filepath.Join(t.TempDir(), "missing-meta.json")

	err := initCheckEnv()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
	assert.Contains(t, jobErr.ErrDetail, "update meta config path")
}

func TestInitCheckEnvMetaJsonInvalid(t *testing.T) {
	oldCfg, oldPath, oldRoot := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig
	t.Cleanup(func() {
		ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
	})

	ConfigCfg = filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(ConfigCfg, []byte("Base: /tmp/test\n"), 0644))
	UpdateMetaConfigPath = filepath.Join(t.TempDir(), "meta.json")
	require.NoError(t, os.WriteFile(UpdateMetaConfigPath, []byte("not-valid-json"), 0644))

	err := initCheckEnv()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
	assert.Contains(t, jobErr.ErrDetail, "load meta config failed")
}

func TestInitCheckEnvSuccess(t *testing.T) {
	oldCfg, oldPath, oldRoot, oldCache, oldSysPkg := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig, ThisCacheInfo, SysPkgInfo
	t.Cleanup(func() {
		ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
		ThisCacheInfo = oldCache
		SysPkgInfo = oldSysPkg
	})

	ConfigCfg = filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(ConfigCfg, []byte("Base: /tmp/test\nDynHookTimeout: 60\n"), 0644))
	UpdateMetaConfigPath = filepath.Join(t.TempDir(), "meta.json")
	require.NoError(t, os.WriteFile(UpdateMetaConfigPath, []byte(`{"UUID":"test-uuid","PkgDebPath":"/tmp/debs"}`), 0644))

	SysPkgInfo = nil

	err := initCheckEnv()
	require.NoError(t, err)
	require.NotNil(t, ThisCacheInfo)
	assert.Equal(t, "test-uuid", ThisCacheInfo.UUID)
	assert.NotNil(t, SysPkgInfo)
}

func TestInitCheckEnvFillsImportantFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want int
	}{
		{"missing", "{}", config.DefaultDynHookTimeout},
		{"zero", "DynHookTimeout: 0\n", config.DefaultDynHookTimeout},
		{"negative", "DynHookTimeout: -1\n", config.DefaultDynHookTimeout},
		{"positive", "DynHookTimeout: 42\n", 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldCfg, oldPath, oldRoot, oldCache, oldSysPkg := ConfigCfg, UpdateMetaConfigPath, RootCoreConfig, ThisCacheInfo, SysPkgInfo
			oldTimeout := check.DynHookTimeout
			t.Cleanup(func() {
				ConfigCfg, UpdateMetaConfigPath, RootCoreConfig = oldCfg, oldPath, oldRoot
				ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
				check.DynHookTimeout = oldTimeout
			})
			dir := t.TempDir()
			ConfigCfg = filepath.Join(dir, "config.yaml")
			UpdateMetaConfigPath = filepath.Join(dir, "meta.json")
			require.NoError(t, os.WriteFile(ConfigCfg, []byte(tc.yaml), 0644))
			require.NoError(t, os.WriteFile(UpdateMetaConfigPath, []byte(`{"UUID":"test-uuid"}`), 0644))
			RootCoreConfig = config.CoreConfig{Base: "/stale", DynHookTimeout: 60}
			check.SetDynHookTimeout(60)
			require.NoError(t, initCheckEnv())
			assert.Equal(t, tc.want, RootCoreConfig.DynHookTimeout)
			assert.Equal(t, tc.want, check.DynHookTimeout)
			assert.Equal(t, "/var/lib/lastore/check/", RootCoreConfig.Base)
			assert.Equal(t, "caches.yaml", RootCoreConfig.CacheList)
			assert.Equal(t, "/var/lib/lastore/check//test-uuid", ThisCacheInfo.WorkStation)
			got, err := os.ReadFile(ConfigCfg)
			require.NoError(t, err)
			assert.Equal(t, tc.yaml, string(got))
		})
	}
}
