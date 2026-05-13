// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"testing"
	"time"

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

	data := []byte(`{"Version":"0.1","AutoCheckUpdates":true,"DisableUpdateMetadata":false,"AutoDownloadUpdates":false,"AutoClean":true,"MirrorSource":"default","UpdateNotify":true,"CheckInterval":604800000000000,"CleanInterval":604800000000000,"UpdateMode":3,"CleanIntervalCacheOverLimit":86400000000000,"AppstoreRegion":"","LastCheckTime":"2021-06-17T14:10:21.896021304+08:00","LastCleanTime":"2021-06-17T09:18:31.515019638+08:00","LastCheckCacheSizeTime":"2021-06-17T09:18:31.5151104+08:00","Repository":"desktop","MirrorsUrl":"","AllowInstallRemovePkgExecPaths":null}`)
	err = os.WriteFile(tmpfile.Name(), data, 0777)
	require.NoError(t, err)

	config := NewConfig(tmpfile.Name())
	require.NotNil(t, config)

	bytes, err := json.Marshal(config)
	require.NoError(t, err)
	configBefore := new(Config)
	err = json.Unmarshal(bytes, configBefore)
	require.NoError(t, err)
	require.NotNil(t, configBefore)

	time.Sleep(time.Millisecond * 10)
	err = config.UpdateLastCheckTime()
	require.NoError(t, err)
	err = config.UpdateLastCleanTime()
	require.NoError(t, err)
	err = config.UpdateLastCheckCacheSizeTime()
	require.NoError(t, err)
	err = config.SetAutoCheckUpdates(!config.AutoCheckUpdates)
	require.NoError(t, err)
	err = config.SetUpdateNotify(!config.UpdateNotify)
	require.NoError(t, err)
	err = config.SetAutoDownloadUpdates(!config.AutoDownloadUpdates)
	require.NoError(t, err)
	err = config.SetAutoClean(!config.AutoClean)
	require.NoError(t, err)
	err = config.SetMirrorSource(config.MirrorSource + "Test")
	require.NoError(t, err)
	err = config.SetAppstoreRegion(config.AppstoreRegion + "Test")
	require.NoError(t, err)
	err = config.SetUpdateMode(config.UpdateMode + 1)
	require.NoError(t, err)

	// 验证
	configAfter := NewConfig(tmpfile.Name())
	require.NotNil(t, configAfter)

	assert.NotEqual(t, configAfter.LastCheckTime, configBefore.LastCheckTime)
	assert.NotEqual(t, configAfter.LastCleanTime, configBefore.LastCleanTime)
	assert.NotEqual(t, configAfter.LastCheckCacheSizeTime, configBefore.LastCheckCacheSizeTime)

	assert.Equal(t, configAfter.AutoCheckUpdates, !configBefore.AutoCheckUpdates)
	assert.Equal(t, configAfter.UpdateNotify, !configBefore.UpdateNotify)
	assert.Equal(t, configAfter.AutoDownloadUpdates, !configBefore.AutoDownloadUpdates)
	assert.Equal(t, configAfter.AutoClean, !configBefore.AutoClean)
	assert.Equal(t, configAfter.MirrorSource, configBefore.MirrorSource+"Test")
	assert.Equal(t, configAfter.AppstoreRegion, configBefore.AppstoreRegion+"Test")
	assert.Equal(t, configAfter.UpdateMode, configBefore.UpdateMode+1)
}

func withTestCachePaths(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	prevOnline := onlineCachePath
	prevClassified := classifiedCachePath
	prevLegacyOnline := legacyOnlineCachePath
	prevLegacyClassified := legacyClassifiedCachePath
	onlineCachePath = tempDir + "/platform_cache.json"
	classifiedCachePath = tempDir + "/classified_cache.json"
	legacyOnlineCachePath = tempDir + "/legacy_platform_cache.json"
	legacyClassifiedCachePath = tempDir + "/legacy_classified_cache.json"

	t.Cleanup(func() {
		onlineCachePath = prevOnline
		classifiedCachePath = prevClassified
		legacyOnlineCachePath = prevLegacyOnline
		legacyClassifiedCachePath = prevLegacyClassified
	})
}

func TestSetOnlineCacheRejectsSymlinkTarget(t *testing.T) {
	withTestCachePaths(t)

	targetFile, err := os.CreateTemp("", "lastore-online-cache-target-*")
	require.NoError(t, err)
	targetPath := targetFile.Name()
	require.NoError(t, targetFile.Close())
	t.Cleanup(func() {
		_ = os.Remove(targetPath)
	})

	originalContent := []byte("original")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0600))
	require.NoError(t, os.Symlink(targetPath, onlineCachePath))

	cfg := &Config{}
	err = cfg.SetOnlineCache(`{"safe":true}`)
	require.NoError(t, err)

	assert.NoFileExists(t, onlineCachePath+".tmp-*")
	content, readErr := os.ReadFile(onlineCachePath)
	require.NoError(t, readErr)
	assert.Equal(t, `{"safe":true}`, string(content))

	content, readErr = os.ReadFile(targetPath)
	require.NoError(t, readErr)
	assert.Equal(t, originalContent, content)
}

func TestSetClassifiedUpdatablePackagesRejectsSymlinkTarget(t *testing.T) {
	withTestCachePaths(t)

	targetFile, err := os.CreateTemp("", "lastore-classified-cache-target-*")
	require.NoError(t, err)
	targetPath := targetFile.Name()
	require.NoError(t, targetFile.Close())
	t.Cleanup(func() {
		_ = os.Remove(targetPath)
	})

	originalContent := []byte("original")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0600))
	require.NoError(t, os.Symlink(targetPath, classifiedCachePath))

	cfg := &Config{}
	err = cfg.SetClassifiedUpdatablePackages(map[string][]string{
		"system": {"pkg1"},
	})
	require.NoError(t, err)

	content, readErr := os.ReadFile(classifiedCachePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "system")

	content, readErr = os.ReadFile(targetPath)
	require.NoError(t, readErr)
	assert.Equal(t, originalContent, content)
}

func TestReadCacheWithLegacyFallback(t *testing.T) {
	withTestCachePaths(t)

	legacyContent := []byte(`{"cached":true}`)
	require.NoError(t, os.WriteFile(legacyOnlineCachePath, legacyContent, 0600))

	content, err := readCacheWithLegacyFallback(onlineCachePath, legacyOnlineCachePath)
	require.NoError(t, err)
	assert.Equal(t, legacyContent, content)
}
