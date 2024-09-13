// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/stretchr/testify/assert"
)

func Test_handleAutoCheckEvent(t *testing.T) {
	m := &Manager{
		config: &config.Config{
			AutoCheckUpdates:      false,
			DisableUpdateMetadata: true,
		},
	}
	err := m.handleAutoCheckEvent()
	assert.Nil(t, err)
}

func Test_handleAutoCleanEvent(t *testing.T) {
	m := &Manager{
		config: &config.Config{
			AutoClean: false,
		},
	}
	err := m.handleAutoCleanEvent()
	assert.Nil(t, err)
}

func Test_getNextUpdateDelay(t *testing.T) {
	m := &Manager{
		config: &config.Config{
			LastCheckTime: time.Now(),
			CheckInterval: 0,
		},
	}
	assert.Equal(t, _minDelayTime, m.getNextUpdateDelay())
	m.config.CheckInterval = time.Hour * 1
	assert.True(t, m.getNextUpdateDelay() > time.Second*10)
}

func Test_canAutoQuit(t *testing.T) {
	m := &Manager{
		jobList:              nil,
		inhibitAutoQuitCount: 3,
		config:               config.NewConfig(""),
	}
	assert.False(t, m.canAutoQuit())
	m.inhibitAutoQuitCount = 0
	m.config.UpgradeStatus.Status = system.UpgradeReady
	assert.True(t, m.canAutoQuit())
}

func Test_saveUpdateSourceOnce(t *testing.T) {
	kf := keyfile.NewKeyFile()
	kf.SetBool("RecordData", "UpdateSourceOnce", false)
	err := kf.SaveToFile(lastoreUnitCache)
	if err != nil {
		logger.Warning(err)
		return
	}
	defer func() {
		_ = os.RemoveAll(lastoreUnitCache) // lastore有生成时，有对应文件（0644），无权限删除；无生成时，单元测试生成，需要移除
	}()
	m := &Manager{}
	m.saveUpdateSourceOnce()
	assert.FileExists(t, lastoreUnitCache)
	m.loadUpdateSourceOnce()
	assert.Equal(t, true, m.updateSourceOnce)
}
