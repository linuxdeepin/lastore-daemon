// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDesktop2uaid(t *testing.T) {
	dir := t.TempDir()
	origBaseDir := BaseDir
	BaseDir = dir
	defer func() { BaseDir = origBaseDir }()

	// non-existent dir should return error and empty map
	result, err := BuildDesktop2uaid()
	assert.Error(t, err)
	assert.Empty(t, result)

	// create dir with a JSON file
	subDir := filepath.Join(dir, "override", "desktop2uaid")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	jsonContent := `{"app1.desktop": "uaid1"}`
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "mapping.json"), []byte(jsonContent), 0644))

	result, err = BuildDesktop2uaid()
	require.NoError(t, err)
	assert.Equal(t, "uaid1", result["app1.desktop"])
}

func TestBuildCategories(t *testing.T) {
	dir := t.TempDir()
	origBaseDir := BaseDir
	BaseDir = dir
	defer func() { BaseDir = origBaseDir }()

	// non-existent dir should return error and empty map
	result, err := BuildCategories()
	assert.Error(t, err)
	assert.Empty(t, result)

	// create dir with a JSON file
	subDir := filepath.Join(dir, "override", "xcategories")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	jsonContent := `{"Game": "games"}`
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "cats.json"), []byte(jsonContent), 0644))

	result, err = BuildCategories()
	require.NoError(t, err)
	assert.Equal(t, "games", result["Game"])
}
