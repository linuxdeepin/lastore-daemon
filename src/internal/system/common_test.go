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
	SetSystemUpdate(true)
	sourceMap := GetCategorySourceMap()
	assert.Equal(t, PlatFormSourceFile, sourceMap[SystemUpdate])
	assert.Equal(t, SecuritySourceDir, sourceMap[SecurityUpdate])
	assert.Equal(t, UnknownSourceDir, sourceMap[UnknownUpdate])

	SetSystemUpdate(false)
	sourceMap = GetCategorySourceMap()
	assert.Equal(t, SoftLinkSystemSourceDir, sourceMap[SystemUpdate])
}

func Test_getGrubTitleByPrefix(t *testing.T) {
	title := getGrubTitleByPrefix("./testdata/grub.cfg", "BEGIN /etc/grub.d/10_linux", "END /etc/grub.d/10_linux")
	assert.Equal(t, "UnionTech OS Desktop 20 Pro GNU/Linux", title)
	title = getGrubTitleByPrefix("./testdata/grub.cfg", "BEGIN /etc/grub.d/11_deepin_ab_recovery", "END /etc/grub.d/11_deepin_ab_recovery")
	assert.Equal(t, "回退到 UOS Desktop 20 Professional（2023/5/19 10:33:44）", title)
}
