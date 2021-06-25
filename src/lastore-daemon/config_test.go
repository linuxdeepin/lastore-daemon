package main

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	testDataPath := "./TemporaryTestDataDirectoryNeedDelete"
	err := os.Mkdir(testDataPath, 0777)
	require.Nil(t, err)
	defer func() {
		err := os.RemoveAll(testDataPath)
		require.Nil(t, err)
	}()
	tmpfile, err := ioutil.TempFile(testDataPath, "config.json")
	require.Nil(t, err)
	defer tmpfile.Close()

	data := []byte(`{"Version":"0.1","AutoCheckUpdates":true,"DisableUpdateMetadata":false,"AutoDownloadUpdates":false,"AutoClean":true,"MirrorSource":"default","UpdateNotify":true,"CheckInterval":604800000000000,"CleanInterval":604800000000000,"UpdateMode":3,"CleanIntervalCacheOverLimit":86400000000000,"AppstoreRegion":"","LastCheckTime":"2021-06-17T14:10:21.896021304+08:00","LastCleanTime":"2021-06-17T09:18:31.515019638+08:00","LastCheckCacheSizeTime":"2021-06-17T09:18:31.5151104+08:00","Repository":"desktop","MirrorsUrl":"http://packages.deepin.com/mirrors/community.json","AllowInstallRemovePkgExecPaths":null}`)
	err = ioutil.WriteFile(tmpfile.Name(), data, 0777)
	require.Nil(t, err)

	configBefore := NewConfig(tmpfile.Name())
	require.NotNil(t, configBefore)
	config := NewConfig(tmpfile.Name())
	require.NotNil(t, config)

	time.Sleep(time.Millisecond * 10)
	err = config.UpdateLastCheckTime()
	require.Nil(t, err)
	err = config.UpdateLastCleanTime()
	require.Nil(t, err)
	err = config.UpdateLastCheckCacheSizeTime()
	require.Nil(t, err)
	err = config.SetAutoCheckUpdates(!config.AutoCheckUpdates)
	require.Nil(t, err)
	err = config.SetUpdateNotify(!config.UpdateNotify)
	require.Nil(t, err)
	err = config.SetAutoDownloadUpdates(!config.AutoDownloadUpdates)
	require.Nil(t, err)
	err = config.SetAutoClean(!config.AutoClean)
	require.Nil(t, err)
	err = config.SetMirrorSource(config.MirrorSource + "Test")
	require.Nil(t, err)
	err = config.SetAppstoreRegion(config.AppstoreRegion + "Test")
	require.Nil(t, err)
	err = config.SetUpdateMode(config.UpdateMode + 1)
	require.Nil(t, err)

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
