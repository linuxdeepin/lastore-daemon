// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
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
	sourceMap := GetCategorySourceMap()
	assert.Equal(t, SystemSourceFile, sourceMap[SystemUpdate])
	assert.Equal(t, SecuritySourceFile, sourceMap[OnlySecurityUpdate])
	assert.Equal(t, UnknownSourceDir, sourceMap[UnknownUpdate])
}
