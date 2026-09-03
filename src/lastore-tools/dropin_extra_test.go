// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleDropInDir(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.json")
	file2 := filepath.Join(tmpDir, "f2.json")
	os.WriteFile(file1, []byte(`{"key1":"val1"}`), 0644)
	os.WriteFile(file2, []byte(`{"key2":"val2"}`), 0644)

	var count int
	err := handleDropInDir(tmpDir, func(f io.Reader) error {
		count++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestHandleDropInDirNonexistent(t *testing.T) {
	err := handleDropInDir("/nonexistent/path/xyz", func(f io.Reader) error {
		return nil
	})
	assert.Error(t, err)
}

func TestBuildMapStringStringInfo(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.json")
	file2 := filepath.Join(tmpDir, "f2.json")
	os.WriteFile(file1, []byte(`{"key1":"val1"}`), 0644)
	os.WriteFile(file2, []byte(`{"key2":"val2","key3":""}`), 0644)

	result, err := buildMapStringStringInfo(tmpDir)
	assert.NoError(t, err)
	assert.Equal(t, "val1", result["key1"])
	assert.Equal(t, "val2", result["key2"])
	_, hasKey3 := result["key3"]
	assert.False(t, hasKey3)
}

func TestBuildMapStringStringInfoNonexistent(t *testing.T) {
	result, err := buildMapStringStringInfo("/nonexistent/path/xyz")
	assert.Error(t, err)
	assert.NotNil(t, result)
}

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
