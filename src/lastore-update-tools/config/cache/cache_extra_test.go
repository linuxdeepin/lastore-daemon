// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestPStateIsOk(t *testing.T) {
	assert.True(t, P_OK.IsOk())
	assert.False(t, P_Init.IsOk())
	assert.False(t, P_Run.IsOk())
	assert.False(t, P_Error.IsOk())
}

func TestPStateIsFault(t *testing.T) {
	assert.False(t, P_Init.IsFault())
	assert.False(t, P_OK.IsFault())
	assert.True(t, P_Run.IsFault())
	assert.True(t, P_Error.IsFault())
	assert.True(t, P_Stage0_Failed.IsFault())
}

func TestPStateIsRunning(t *testing.T) {
	assert.True(t, P_Run.IsRunning())
	assert.False(t, P_OK.IsRunning())
	assert.False(t, P_Init.IsRunning())
}

func TestPStateIsFirstRun(t *testing.T) {
	assert.True(t, P_Init.IsFirstRun())
	assert.True(t, PState("").IsFirstRun())
	assert.False(t, P_OK.IsFirstRun())
	assert.False(t, P_Run.IsFirstRun())
}

func TestPkgStateCheckOK(t *testing.T) {
	tests := []struct {
		name  string
		state PkgState
		want  bool
	}{
		{"installed ok", InstalledOK, true},
		{"hold installed", HoldInstalled, true},
		{"hold purged", HoldPurged, true},
		{"hold trigger pending", HoldTrigerPending, true},
		{"installed trigger pending", InstalledTriggerPending, true},
		{"only config files", OnlyConfigFiles, true},
		{"removed", Removed, true},
		{"purged", Purged, true},
		{"empty", AppStateDefault, false},
		{"install half", InstallHalf, false},
		{"purged half", PurgedHalf, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.CheckOK())
		})
	}
}

func TestPkgStateCheckFailed(t *testing.T) {
	tests := []struct {
		name    string
		state   PkgState
		want    bool
		wantErr bool
	}{
		{"install half", InstallHalf, true, false},
		{"install unpacked", InstallUnpacked, true, false},
		{"purged half", PurgedHalf, true, false},
		{"remove half", RemoveHalf, true, false},
		{"hold half", HoldHalf, true, false},
		{"hold unpacked", HoldUnpacked, true, false},
		{"installed ok", InstalledOK, false, false},
		{"empty", AppStateDefault, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.state.CheckFailed()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPkgStateCheckConfigure(t *testing.T) {
	tests := []struct {
		name    string
		state   PkgState
		want    bool
		wantErr bool
	}{
		{"install config pending", InstallConfigPending, true, false},
		{"hold config pending", HoldConfigPending, true, false},
		{"hold trigger await", HoldTriggerAwait, true, false},
		{"install trigger await", InstallTriggerAwait, true, false},
		{"installed ok", InstalledOK, false, false},
		{"empty", AppStateDefault, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.state.CheckConfigure()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCacheConfigLoader(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cache.yaml")
	cc := &CacheConfig{
		Cache: map[string]CacheInfo{
			"uuid-1": {UUID: "uuid-1", Status: "ok"},
		},
	}
	data, err := yaml.Marshal(cc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0644))

	var loaded CacheConfig
	err = loaded.Loader(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "uuid-1", loaded.Cache["uuid-1"].UUID)
	assert.Equal(t, "ok", loaded.Cache["uuid-1"].Status)
}

func TestCacheConfigLoaderNonExist(t *testing.T) {
	var cc CacheConfig
	err := cc.Loader("/nonexistent/path/file.yaml")
	assert.Error(t, err)
}

func TestCacheConfigUpdateUUID(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cache.yaml")
	cc := &CacheConfig{Cache: make(map[string]CacheInfo)}
	err := cc.UpdateUUID(cfgPath, "uuid-new", CacheInfo{UUID: "uuid-new", Status: "run"})
	require.NoError(t, err)

	var loaded CacheConfig
	require.NoError(t, loaded.Loader(cfgPath))
	assert.Equal(t, "uuid-new", loaded.Cache["uuid-new"].UUID)

	err = cc.UpdateUUID(cfgPath, "uuid-new", CacheInfo{UUID: "uuid-new"})
	assert.Error(t, err)
}

func TestCacheInfoClearUUID(t *testing.T) {
	dir := t.TempDir()
	uuid := "test-uuid"
	workDir := filepath.Join(dir, uuid)
	require.NoError(t, os.MkdirAll(workDir, 0755))
	archiveFile := filepath.Join(dir, uuid+"-archive.tar.gz")
	require.NoError(t, os.WriteFile(archiveFile, []byte("archive"), 0644))

	var ci CacheInfo
	err := ci.ClearUUID(dir, uuid)
	assert.NoError(t, err)
	_, statErr := os.Stat(workDir)
	assert.True(t, os.IsNotExist(statErr))
	_, statErr = os.Stat(archiveFile)
	assert.True(t, os.IsNotExist(statErr))
}

func TestCacheInfoClearUUIDNonExist(t *testing.T) {
	dir := t.TempDir()
	var ci CacheInfo
	err := ci.ClearUUID(dir, "nonexistent-uuid")
	assert.NoError(t, err)
}
