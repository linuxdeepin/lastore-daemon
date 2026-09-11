// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
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
	old := UpdateMetaConfigPath
	UpdateMetaConfigPath = ""
	defer func() { UpdateMetaConfigPath = old }()

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

	ConfigCfg = filepath.Join(t.TempDir(), "missing.yaml")
	UpdateMetaConfigPath = "/tmp/whatever.json"

	err := initCheckEnv()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
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
