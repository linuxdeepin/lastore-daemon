// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteData(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "subdir", "data.json")
	data := map[string]string{"key": "value"}
	err := WriteData(fp, data)
	require.NoError(t, err)

	content, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"key":"value"`)
}

func TestValidURL(t *testing.T) {
	assert.True(t, ValidURL("http://example.com"))
	assert.True(t, ValidURL("https://example.com"))
	assert.False(t, ValidURL("ftp://example.com"))
	assert.False(t, ValidURL("example.com"))
	assert.False(t, ValidURL(""))
}

func TestWriteFileSecurely(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "secure.txt")
	content := []byte("secure content")

	err := WriteFileSecurely(fp, content, 0644)
	require.NoError(t, err)

	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	info, err := os.Stat(fp)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode())
}

func TestWriteFileSecurelyNestedDir(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "a", "b", "secure.txt")
	err := WriteFileSecurely(fp, []byte("nested"), 0600)
	require.NoError(t, err)
	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestEnsureBaseDir(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "subdir", "file.txt")
	err := EnsureBaseDir(fp)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "subdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Already exists
	err = EnsureBaseDir(fp)
	assert.NoError(t, err)
}

func TestRunCommand(t *testing.T) {
	out, err := RunCommand("echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)

	_, err = RunCommand("false")
	assert.Error(t, err)
}
