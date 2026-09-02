// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestRepoTypeString(t *testing.T) {
	tests := []struct {
		name string
		r    RepoType
		want string
	}{
		{"OSDefaultRepo", OSDefaultRepo, "OSDefaultRepo"},
		{"OemDefaultRepo", OemDefaultRepo, "OemDefaultRepo"},
		{"CustomRepo", CustomRepo, "CustomRepo"},
		{"Unknown", RepoType("UNKNOWN"), ""},
		{"Empty", RepoType(""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.r.String())
		})
	}
}

func TestRepoTypeIsValid(t *testing.T) {
	assert.True(t, OSDefaultRepo.IsValid())
	assert.True(t, OemDefaultRepo.IsValid())
	assert.True(t, CustomRepo.IsValid())
	assert.False(t, RepoType("UNKNOWN").IsValid())
	assert.False(t, RepoType("").IsValid())
}

func TestGetOemRepoInfoNonExistDir(t *testing.T) {
	sys, sec := GetOemRepoInfo("/nonexistent/path/for/test")
	assert.Nil(t, sys)
	assert.Nil(t, sec)
}

func TestGetOemRepoInfoEmptyDir(t *testing.T) {
	dir := t.TempDir()
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.Equal(t, system.SystemUpdate, sys.UpdateType)
	assert.Equal(t, system.SecurityUpdate, sec.UpdateType)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoWithFiles(t *testing.T) {
	dir := t.TempDir()

	systemRepo := OemRepoConfig{
		UpdateType:     system.SystemUpdate,
		RepoShowNameZh: "系统仓库",
		RepoShowNameEn: "System Repo",
		RepoUrl:        []string{"http://example.com/system"},
	}
	data, err := json.Marshal(&systemRepo)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "system.json"), data, 0644))

	securityRepo := OemRepoConfig{
		UpdateType:     system.SecurityUpdate,
		RepoShowNameZh: "安全仓库",
		RepoShowNameEn: "Security Repo",
		RepoUrl:        []string{"http://example.com/security"},
	}
	data, err = json.Marshal(&securityRepo)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "security.json"), data, 0644))

	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.Equal(t, "系统仓库", sys.RepoShowNameZh)
	assert.Equal(t, "System Repo", sys.RepoShowNameEn)
	assert.Equal(t, []string{"http://example.com/system"}, sys.RepoUrl)
	assert.True(t, sys.hasSet)

	assert.Equal(t, "安全仓库", sec.RepoShowNameZh)
	assert.Equal(t, "Security Repo", sec.RepoShowNameEn)
	assert.Equal(t, []string{"http://example.com/security"}, sec.RepoUrl)
	assert.True(t, sec.hasSet)
}

func TestGetOemRepoInfoInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid json}"), 0644))
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoInvalidUpdateType(t *testing.T) {
	dir := t.TempDir()
	badRepo := OemRepoConfig{
		UpdateType: system.UpdateType(999),
		RepoUrl:    []string{"http://example.com/bad"},
	}
	data, err := json.Marshal(&badRepo)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad_type.json"), data, 0644))
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoSkipNonJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoMultipleFilesSameType(t *testing.T) {
	dir := t.TempDir()

	repo1 := OemRepoConfig{
		UpdateType:     system.SystemUpdate,
		RepoShowNameZh: "第一个",
		RepoUrl:        []string{"http://first.com"},
	}
	data, _ := json.Marshal(&repo1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json"), data, 0644))

	repo2 := OemRepoConfig{
		UpdateType:     system.SystemUpdate,
		RepoShowNameZh: "第二个",
		RepoUrl:        []string{"http://second.com"},
	}
	data, _ = json.Marshal(&repo2)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.json"), data, 0644))

	sys, _ := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	assert.True(t, sys.hasSet)
	assert.Equal(t, "第二个", sys.RepoShowNameZh)
	assert.Equal(t, []string{"http://second.com"}, sys.RepoUrl)
}

func TestGetPlatformStatusDisable(t *testing.T) {
	c := &Config{PlatformDisabled: DisabledVersion | DisabledUpdateLog}
	assert.True(t, c.GetPlatformStatusDisable(DisabledVersion))
	assert.True(t, c.GetPlatformStatusDisable(DisabledUpdateLog))
	assert.False(t, c.GetPlatformStatusDisable(DisabledTargetPkgLists))

	c2 := &Config{PlatformDisabled: 0}
	assert.False(t, c2.GetPlatformStatusDisable(DisabledVersion))
}

func TestConfigUseIncrementalUpdate(t *testing.T) {
	c := &Config{IncrementalUpdate: true}
	assert.True(t, c.UseIncrementalUpdate())
	c2 := &Config{IncrementalUpdate: false}
	assert.False(t, c2.UseIncrementalUpdate())
}

func TestConfigResetDSettingsNoManager(t *testing.T) {
	c := &Config{}
	err := c.ResetDSettings("some-key")
	assert.NoError(t, err)
}
