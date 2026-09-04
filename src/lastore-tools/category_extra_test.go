// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/dstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteDataCategory(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "sub", "data.json")
	err := writeData(fp, map[string]string{"k": "v"})
	require.NoError(t, err)

	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"k":"v"`)
}

func TestGenerateCategoryDeprecated(t *testing.T) {
	err := GenerateCategory("repo", "/some/path")
	assert.NoError(t, err)
}

func TestGenApplications(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "apps.json")

	pkgs := []*dstore.PackageInfo{
		{
			Name:        "Test App",
			Category:    "graphics",
			PackageName: "test-app",
			Locale: map[string]struct {
				Description struct {
					Name string `json:"name"`
				} `json:"description"`
			}{
				"zh_CN": {Description: struct {
					Name string `json:"name"`
				}{Name: "测试应用"}},
			},
		},
		{
			Name:        "Simple",
			Category:    "utils",
			PackageName: "simple",
			Locale:      nil,
		},
	}

	err := genApplications(pkgs, fp)
	require.NoError(t, err)

	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-app")
	assert.Contains(t, string(data), "测试应用")
	assert.Contains(t, string(data), "simple")
}

func TestGenerateApplications(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "apps.json")

	// Write a fresh cache so dstore.cacheFetchJSON decodes locally (no network).
	cacheContent := `{"dpk://deb/testapp":{"name":"Test","category":"graphics","package_name":"testapp","locale":{"zh_CN":{"description":{"name":"测试应用"}}}}}`
	require.NoError(t, os.WriteFile(fpath+".cache.json", []byte(cacheContent), 0644))

	err := GenerateApplications("repo", fpath)
	require.NoError(t, err)

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "testapp")
	assert.Contains(t, string(data), "graphics")
	assert.Contains(t, string(data), "测试应用")
}

func TestGenerateApplicationsWriteError(t *testing.T) {
	dir := t.TempDir()
	// fpath is a non-empty directory: the cache decode succeeds, then the
	// final os.Rename in WriteFileSecurely fails, exercising the error branch.
	fpath := filepath.Join(dir, "apps.json")
	require.NoError(t, os.MkdirAll(filepath.Join(fpath, "child"), 0755))

	cacheContent := `{"dpk://deb/testapp":{"name":"Test","category":"graphics","package_name":"testapp","locale":{}}}`
	require.NoError(t, os.WriteFile(fpath+".cache.json", []byte(cacheContent), 0644))

	err := GenerateApplications("repo", fpath)
	assert.Error(t, err)
}
