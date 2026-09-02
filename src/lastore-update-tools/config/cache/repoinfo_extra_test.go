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

func TestRepoInfoMerge(t *testing.T) {
	left := &RepoInfo{Name: "orig", Type: "deb"}
	right := RepoInfo{Name: "new", URL: "http://example.com"}

	err := left.Merge(right)
	assert.NoError(t, err)
	assert.Equal(t, "new", left.Name)
	assert.Equal(t, "http://example.com", left.URL)
	assert.Equal(t, "deb", left.Type)
}

func TestRepoInfoMergeNoOverwriteWithEmpty(t *testing.T) {
	left := &RepoInfo{Name: "orig", Type: "deb", URL: "http://keep.com"}
	right := RepoInfo{Name: "", Type: "", URL: ""}

	err := left.Merge(right)
	assert.NoError(t, err)
	assert.Equal(t, "orig", left.Name)
	assert.Equal(t, "deb", left.Type)
	assert.Equal(t, "http://keep.com", left.URL)
}

func TestRepoInfoMergeComponents(t *testing.T) {
	left := &RepoInfo{Name: "orig", Components: []string{"main"}}
	right := RepoInfo{Components: []string{"main", "contrib"}}

	err := left.Merge(right)
	assert.NoError(t, err)
	assert.Equal(t, []string{"main", "contrib"}, left.Components)
}

func TestRepoInfoCheckRepoIndexExist(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.repo")
	require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

	ri := &RepoInfo{FilePath: filePath}
	err := ri.CheckRepoIndexExist()
	assert.NoError(t, err)
}

func TestRepoInfoCheckRepoIndexExistNotFound(t *testing.T) {
	ri := &RepoInfo{FilePath: "/nonexistent/path/test.repo"}
	err := ri.CheckRepoIndexExist()
	assert.Error(t, err)
}

func TestRepoInfoCheckRepoFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.repo")
	require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

	ri := &RepoInfo{FilePath: filePath, HashSha256: "dummy"}
	err := ri.CheckRepoFile()
	assert.NoError(t, err)
}

func TestRepoInfoCheckRepoFileNotExist(t *testing.T) {
	ri := &RepoInfo{FilePath: "/nonexistent/path/test.repo"}
	err := ri.CheckRepoFile()
	assert.Error(t, err)
}

func TestRepoInfoLoaderPackageInfo(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.repo")
	require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

	ri := &RepoInfo{FilePath: filePath}
	err := ri.LoaderPackageInfo(&CacheInfo{})
	assert.NoError(t, err)
}

func TestRepoInfoLoaderPackageInfoNotExist(t *testing.T) {
	ri := &RepoInfo{FilePath: "/nonexistent/path/test.repo"}
	err := ri.LoaderPackageInfo(&CacheInfo{})
	assert.Error(t, err)
}
