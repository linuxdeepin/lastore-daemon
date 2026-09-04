// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
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

func TestConfig(t *testing.T) {
	testDataPath := "./TemporaryTestDataDirectoryNeedDelete"
	err := os.Mkdir(testDataPath, 0777)
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testDataPath)
		require.NoError(t, err)
	}()
	tmpfile, err := os.CreateTemp(testDataPath, "config.json")
	require.NoError(t, err)
	defer tmpfile.Close()

	data := []byte(`{"filePath":"/","Enable":true}`)
	err = os.WriteFile(tmpfile.Name(), data, 0777)
	require.NoError(t, err)
	configBefore := newConfig(tmpfile.Name())
	require.NotNil(t, configBefore)
	config := newConfig(tmpfile.Name())
	require.NotNil(t, config)
	err = config.setEnable(!config.Enable)
	require.NoError(t, err)

	// 验证
	configAfter := newConfig(tmpfile.Name())
	require.NotNil(t, configAfter)
	assert.Equal(t, configAfter.Enable, !configBefore.Enable)
}

func TestConfigSave(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "config.json")

	c := newConfig(fpath)
	require.NotNil(t, c)
	require.True(t, c.Enable) // default

	err := c.setEnable(false)
	require.NoError(t, err)

	reloaded := newConfig(fpath)
	require.NotNil(t, reloaded)
	assert.False(t, reloaded.Enable)
}

func TestNewConfigNonexistent(t *testing.T) {
	dir := t.TempDir()
	c := newConfig(filepath.Join(dir, "does-not-exist.json"))
	require.NotNil(t, c)
	assert.True(t, c.Enable)
	assert.Equal(t, filepath.Join(dir, "does-not-exist.json"), c.filePath)
}

func TestConfigSaveRemoveError(t *testing.T) {
	// c.filePath is a non-empty directory, so os.Remove fails with an error
	// that is NOT os.ErrNotExist, exercising the logger.Warning branch.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0644))

	c := newConfig(dir)
	err := c.save()
	assert.Error(t, err)
}
