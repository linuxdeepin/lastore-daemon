/*
 * Copyright (C) 2020 ~ 2022 Deepin Technology Co., Ltd.
 *
 * Author:     lichangze <ut001335@uniontech.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package main

import (
	"os"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/stretchr/testify/assert"
)

func Test_handleAutoCheckEvent(t *testing.T) {
	m := &Manager{
		config: &Config{
			AutoCheckUpdates:      false,
			DisableUpdateMetadata: true,
			filePath:              "./tempCfgPath",
		},
	}
	err := m.handleAutoCheckEvent()
	assert.Nil(t, err)
	_ = os.RemoveAll("./tempCfgPath")
}

func Test_handleAutoCleanEvent(t *testing.T) {
	m := &Manager{
		config: &Config{
			AutoClean: false,
		},
	}
	err := m.handleAutoCleanEvent()
	assert.Nil(t, err)
}

func Test_getNextUpdateDelay(t *testing.T) {
	m := &Manager{
		config: &Config{
			LastCheckTime: time.Now(),
			CheckInterval: 0,
		},
	}
	assert.Equal(t, time.Duration(0), m.getNextUpdateDelay())
	m.config.CheckInterval = time.Hour * 1
	assert.True(t, m.getNextUpdateDelay() > time.Second*10)
}

func Test_canAutoQuit(t *testing.T) {
	m := &Manager{
		jobList:              nil,
		inhibitAutoQuitCount: 3,
	}
	assert.False(t, m.canAutoQuit())
	m.inhibitAutoQuitCount = 0
	assert.True(t, m.canAutoQuit())
}

func Test_saveUpdateSourceOnce(t *testing.T) {
	kf := keyfile.NewKeyFile()
	kf.SetBool("RecordData", "UpdateSourceOnce", false)
	err := kf.SaveToFile(lastoreUnitCache)
	if err != nil {
		logger.Warning(err)
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
