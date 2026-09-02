// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckFileExistStateExtra(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "exists.txt")
	require.NoError(t, os.WriteFile(fp, []byte("test"), 0644))

	assert.NoError(t, CheckFileExistState(fp))
	err := CheckFileExistState(filepath.Join(dir, "nonexistent"))
	assert.Error(t, err)
}

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "subdir", "testfile.txt")
	f, err := CreateFile(filePath)
	require.NoError(t, err)
	defer f.Close()
	assert.NotNil(t, f)

	_, err = os.Stat(filePath)
	assert.NoError(t, err)
}

func TestReadMode(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(fp, []byte("test"), 0755))

	mode, err := ReadMode(fp)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), mode)

	_, err = ReadMode(filepath.Join(dir, "nonexistent"))
	assert.Error(t, err)
}

func TestCreateDirMode(t *testing.T) {
	dir := t.TempDir()
	newDir := filepath.Join(dir, "newdir")
	err := CreateDirMode(newDir, 0755)
	require.NoError(t, err)

	info, err := os.Stat(newDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Already exists - should be no error
	err = CreateDirMode(newDir, 0755)
	assert.NoError(t, err)
}
