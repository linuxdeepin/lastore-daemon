// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPackagesPathList_NonexistentDir(t *testing.T) {
	result := getPackagesPathList(system.SystemUpdate, "/nonexistent/path/12345")
	assert.Nil(t, result)
}

func TestGenRepoInfo_NonexistentDir(t *testing.T) {
	result := genRepoInfo(system.SystemUpdate, "/nonexistent/path/12345")
	assert.Nil(t, result)
}

func TestGetPackagesPathList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result := getPackagesPathList(system.SystemUpdate, dir)
	assert.Nil(t, result)
}

func TestGenRepoInfo_WithFiles(t *testing.T) {
	// Set up source directory with a repo URL
	srcDir := t.TempDir()
	listFile := filepath.Join(srcDir, "test.list")
	require.NoError(t, os.WriteFile(listFile, []byte("deb http://test.example.com/debian stable main\n"), 0644))

	// Save and restore SystemUpdateSource
	origSource := system.SystemUpdateSource
	system.SystemUpdateSource = srcDir
	defer func() { system.SystemUpdateSource = origSource }()

	// Create Packages dir with matching file
	// URIToPath("http://test.example.com/debian") = "test.example.com/debian"
	// ReplaceAll("/", "_") = "test.example.com_debian"
	listDir := t.TempDir()
	pkgFile := filepath.Join(listDir, "test.example.com_debian_dists_stable_main_binary-amd64_Packages")
	require.NoError(t, os.WriteFile(pkgFile, []byte("Package: testpkg\nVersion: 1.0\n"), 0644))

	result := getPackagesPathList(system.SystemUpdate, listDir)
	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "Packages")

	// Test genRepoInfo with same setup
	repoInfos := genRepoInfo(system.SystemUpdate, listDir)
	assert.Len(t, repoInfos, 1)
	assert.NotEmpty(t, repoInfos[0].HashSha256)
	assert.Equal(t, "test.example.com_debian_dists_stable_main_binary-amd64_Packages", repoInfos[0].Name)
}
