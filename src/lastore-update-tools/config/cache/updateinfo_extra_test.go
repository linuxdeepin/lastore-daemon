// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyUpdateInfoEmptyPkgDebPath(t *testing.T) {
	ui := &UpdateInfo{UUID: "test-uuid", RepoBackend: []RepoInfo{{Name: "r1"}}}
	err := ui.VerifyUpdateInfo()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pkgDebPath")
}

func TestVerifyUpdateInfoEmptyUUID(t *testing.T) {
	ui := &UpdateInfo{PkgDebPath: "/tmp/", RepoBackend: []RepoInfo{{Name: "r1"}}}
	err := ui.VerifyUpdateInfo()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uuid")
}

func TestVerifyUpdateInfoEmptyRepoBackend(t *testing.T) {
	ui := &UpdateInfo{PkgDebPath: "/tmp/", UUID: "test-uuid"}
	err := ui.VerifyUpdateInfo()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repoBackend")
}

func TestVerifyUpdateInfoValid(t *testing.T) {
	dir := t.TempDir()
	repoFile := filepath.Join(dir, "Packages.test")
	require.NoError(t, os.WriteFile(repoFile, []byte("test"), 0644))

	ui := &UpdateInfo{
		PkgDebPath:  "/tmp/",
		UUID:        "test-uuid",
		RepoBackend: []RepoInfo{{Name: "r1", FilePath: repoFile, HashSha256: "dummy"}},
	}
	err := ui.VerifyUpdateInfo()
	assert.NoError(t, err)
}

func TestUpdateInfoLoaderJson(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "config.json")
	jsonData := `{"PkgDebPath":"/tmp/","UUID":"abc-123","Time":"2024-01-01"}`
	require.NoError(t, os.WriteFile(jsonFile, []byte(jsonData), 0644))

	ui := &UpdateInfo{}
	err := ui.LoaderJson(jsonFile)
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/", ui.PkgDebPath)
	assert.Equal(t, "abc-123", ui.UUID)
	assert.Equal(t, "2024-01-01", ui.Time)
}

func TestUpdateInfoLoaderJsonFileNotExist(t *testing.T) {
	ui := &UpdateInfo{}
	err := ui.LoaderJson("/nonexistent/path/config.json")
	assert.Error(t, err)
}

func TestUpdateInfoLoaderJsonInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte("{invalid"), 0644))

	ui := &UpdateInfo{}
	err := ui.LoaderJson(jsonFile)
	assert.Error(t, err)
}

func TestUpdateInfoRemovedRepoInfo(t *testing.T) {
	ui := &UpdateInfo{
		RepoBackend: []RepoInfo{
			{Name: "r1"},
			{Name: "r2"},
			{Name: "r3"},
		},
	}
	err := ui.RemovedRepoInfo(1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ui.RepoBackend))
	assert.Equal(t, "r1", ui.RepoBackend[0].Name)
	assert.Equal(t, "r3", ui.RepoBackend[1].Name)
}

func TestUpdateInfoRemovedRepoInfoSingleElement(t *testing.T) {
	ui := &UpdateInfo{
		RepoBackend: []RepoInfo{{Name: "r1"}},
	}
	err := ui.RemovedRepoInfo(0)
	assert.NoError(t, err)
	assert.Nil(t, ui.RepoBackend)
}

func TestUpdateInfoRemovedRepoInfoOutOfRange(t *testing.T) {
	ui := &UpdateInfo{
		RepoBackend: []RepoInfo{{Name: "r1"}, {Name: "r2"}},
	}
	err := ui.RemovedRepoInfo(10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestUpdateInfoRemovedRepoInfoInvalidIndex(t *testing.T) {
	ui := &UpdateInfo{
		RepoBackend: []RepoInfo{{Name: "r1"}, {Name: "r2"}},
	}
	err := ui.RemovedRepoInfo(-1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ui.RepoBackend))
}
