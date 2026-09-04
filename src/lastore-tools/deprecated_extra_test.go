// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestGetPackageName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pkg.list", "pkg"},
		{"pkg:amd64.list", "pkg"},
		{"short", "short"},
		{"a.list", "a"},
		{"abc.list", "abc"},
	}
	for _, tt := range tests {
		got := getPackageName(tt.input)
		assert.Equal(t, tt.want, got, "input=%s", tt.input)
	}
}

func TestMergeDesktopIndex(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "index.json")

	infos := map[string]string{
		"app1.desktop": "pkg1",
		"app2.desktop": "pkg2",
	}
	result := mergeDesktopIndex(infos, fpath)
	assert.Equal(t, "pkg1", result["app1.desktop"])
	assert.Equal(t, "pkg2", result["app2.desktop"])

	// Read back and merge again
	infos2 := map[string]string{
		"app3.desktop": "pkg3",
	}
	result2 := mergeDesktopIndex(infos2, fpath)
	assert.Equal(t, "pkg1", result2["app1.desktop"])
	assert.Equal(t, "pkg3", result2["app3.desktop"])
}

func TestMergeDesktopIndexNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "nonexistent.json")

	infos := map[string]string{
		"app1.desktop": "pkg1",
	}
	result := mergeDesktopIndex(infos, fpath)
	assert.Equal(t, "pkg1", result["app1.desktop"])

	_, err := os.Stat(fpath)
	assert.NoError(t, err)
}

func TestMergeDesktopIndexEmptyValues(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "index.json")

	infos := map[string]string{
		"":     "pkg1",
		"app1": "",
		"app2": "pkg2",
	}
	result := mergeDesktopIndex(infos, fpath)
	_, hasEmpty := result[""]
	assert.False(t, hasEmpty)
	_, hasEmptyVal := result["app1"]
	assert.False(t, hasEmptyVal)
	assert.Equal(t, "pkg2", result["app2"])
}

func TestBuildDesktopDirectories(t *testing.T) {
	dirs := BuildDesktopDirectories()
	assert.NotEmpty(t, dirs)
	// Should contain the standard application directory
	found := false
	for _, d := range dirs {
		if d == "/usr/share/applications" {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain /usr/share/applications")
}

func TestGetDesktopFiles(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .desktop files and a non-desktop file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app1.desktop"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app2.desktop"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("x"), 0644))

	result := GetDesktopFiles([]string{tmpDir})
	assert.Len(t, result, 2)
}

func TestGetDesktopFilesNonExistentDir(t *testing.T) {
	result := GetDesktopFiles([]string{"/nonexistent/path/12345"})
	assert.Nil(t, result)
}

func TestParseDesktopInfo(t *testing.T) {
	tmpDir := t.TempDir()
	desktopPath := filepath.Join(tmpDir, "testapp.desktop")
	content := `[Desktop Entry]
Name=TestApp
Icon=test-icon
Exec=/usr/bin/testapp %f
Type=Application
`
	require.NoError(t, os.WriteFile(desktopPath, []byte(content), 0644))

	pkgIndex := map[string]string{
		"testapp.desktop": "testpkg",
	}
	info := ParseDesktopInfo(pkgIndex, desktopPath)
	require.NotNil(t, info)
	assert.Equal(t, "testpkg", info.Package)
	assert.Equal(t, "test-icon", info.Icon)
	assert.Equal(t, "/usr/bin/testapp %f", info.Exec)
}

func TestParseDesktopInfoNonExistentFile(t *testing.T) {
	info := ParseDesktopInfo(map[string]string{}, "/nonexistent/file.desktop")
	assert.Nil(t, info)
}

func TestParseDesktopInfoFallbackPkgName(t *testing.T) {
	tmpDir := t.TempDir()
	desktopPath := filepath.Join(tmpDir, "unknown.desktop")
	content := "Icon=foo\nExec=bar\n"
	require.NoError(t, os.WriteFile(desktopPath, []byte(content), 0644))

	info := ParseDesktopInfo(map[string]string{}, desktopPath)
	require.NotNil(t, info)
	// Package should fall back to the base name
	assert.Equal(t, "unknown.desktop", info.Package)
}

func TestGetDesktopFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "list.txt")
	content := `/usr/share/applications/app1.desktop
/usr/share/applications/app2.desktop
/some/other/file.txt
/usr/share/applications/app3.desktop
`
	require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))

	result := getDesktopFilePaths(listFile)
	assert.Len(t, result, 3)
	for _, p := range result {
		assert.True(t, path.Ext(p) == ".desktop")
	}
}

func TestGetDesktopFilePathsNonExistent(t *testing.T) {
	result := getDesktopFilePaths("/nonexistent/list.txt")
	assert.Nil(t, result)
}

func TestGetPackageName2(t *testing.T) {
	assert.Equal(t, "foo", getPackageName("foo.list"))
	assert.Equal(t, "ab", getPackageName("ab.list"))
	assert.Equal(t, "pkg", getPackageName("pkg"))
}

func TestParsePackageInfos(t *testing.T) {
	r, tmap := ParsePackageInfos()
	// Both maps are non-nil even when /var/lib/dpkg/info is empty or unreadable.
	assert.NotNil(t, r)
	assert.NotNil(t, tmap)
}
