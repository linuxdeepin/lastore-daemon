// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
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
