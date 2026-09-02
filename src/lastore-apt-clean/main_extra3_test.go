// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMustGetBin(t *testing.T) {
	// dpkg should exist in the test environment
	path := mustGetBin("dpkg")
	assert.NotEmpty(t, path)
}

func TestFindBins(t *testing.T) {
	findBins()
	assert.NotEmpty(t, binDpkg)
	assert.NotEmpty(t, binDpkgQuery)
	assert.NotEmpty(t, binDpkgDeb)
	assert.NotEmpty(t, binAptCache)
}

func TestCompareVersionsGtDpkg(t *testing.T) {
	findBins()
	// 2.0 > 1.0
	assert.True(t, compareVersionsGtDpkg("2.0", "1.0"))
	// 1.0 is not > 2.0
	assert.False(t, compareVersionsGtDpkg("1.0", "2.0"))
	// 1.0 is not > 1.0
	assert.False(t, compareVersionsGtDpkg("1.0", "1.0"))
}
