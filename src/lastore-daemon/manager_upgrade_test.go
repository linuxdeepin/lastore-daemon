// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInstallLikeJobType(t *testing.T) {
	assert.True(t, isInstallLikeJobType("install"))
	assert.True(t, isInstallLikeJobType("only_install"))
	assert.True(t, isInstallLikeJobType("system_upgrade"))
	assert.True(t, isInstallLikeJobType("security_upgrade"))
	assert.True(t, isInstallLikeJobType("appstore_upgrade"))
	assert.False(t, isInstallLikeJobType("update_source"))
	assert.False(t, isInstallLikeJobType(""))
}
