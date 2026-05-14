// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateType_JobType(t *testing.T) {
	updateType := SystemUpdate
	assert.Equal(t, SystemUpgradeJobType, updateType.JobType())
	updateType = AppStoreUpdate
	assert.Equal(t, AppStoreUpgradeJobType, updateType.JobType())
	updateType = SecurityUpdate
	assert.Equal(t, SecurityUpgradeJobType, updateType.JobType())
	updateType = UnknownUpdate
	assert.Equal(t, UnknownUpgradeJobType, updateType.JobType())
}

func Test_GetCategorySourceMap(t *testing.T) {
	SetSystemUpdate(true)
	sourceMap := GetCategorySourceMap()
	assert.Equal(t, PlatFormSourceFile, sourceMap[SystemUpdate])
	assert.Equal(t, SecuritySourceDir, sourceMap[SecurityUpdate])
	assert.Equal(t, UnknownSourceDir, sourceMap[UnknownUpdate])

	SetSystemUpdate(false)
	sourceMap = GetCategorySourceMap()
	assert.Equal(t, SoftLinkSystemSourceDir, sourceMap[SystemUpdate])
}

func TestCollectAndClearLocaleEnvs(t *testing.T) {
	oldOriginalLocaleEnvs := OriginalLocaleEnvs
	originalValues := make(map[string]*string)
	localeEnvNames := []string{"LC_ALL", "LANGUAGE", "LC_MESSAGES", "LANG"}

	for _, name := range localeEnvNames {
		value, exists := os.LookupEnv(name)
		if exists {
			valueCopy := value
			originalValues[name] = &valueCopy
		} else {
			originalValues[name] = nil
		}
	}

	t.Cleanup(func() {
		OriginalLocaleEnvs = oldOriginalLocaleEnvs
		for _, name := range localeEnvNames {
			value := originalValues[name]
			if value == nil {
				_ = os.Unsetenv(name)
				continue
			}
			_ = os.Setenv(name, *value)
		}
	})

	OriginalLocaleEnvs = nil
	assert.NoError(t, os.Setenv("LC_ALL", "zh_CN.UTF-8"))
	assert.NoError(t, os.Setenv("LANGUAGE", "zh_CN:en_US"))
	assert.NoError(t, os.Setenv("LC_MESSAGES", "zh_CN.UTF-8"))
	assert.NoError(t, os.Setenv("LANG", "zh_CN.UTF-8"))

	CollectAndClearLocaleEnvs()

	assert.Equal(t, []string{
		"LC_ALL=zh_CN.UTF-8",
		"LANGUAGE=zh_CN:en_US",
		"LC_MESSAGES=zh_CN.UTF-8",
		"LANG=zh_CN.UTF-8",
	}, OriginalLocaleEnvs)

	for _, name := range localeEnvNames {
		_, exists := os.LookupEnv(name)
		assert.False(t, exists, name)
	}
}
