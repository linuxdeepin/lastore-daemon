// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryEnhancedLocalePackages(t *testing.T) {
	checker := func(s string) bool { return true }

	result := QueryEnhancedLocalePackages(checker, "zh_CN", "deepin-software-center")
	_ = result
}

func TestQueryEnhancedLocalePackagesFilterFalse(t *testing.T) {
	checker := func(s string) bool { return false }

	result := QueryEnhancedLocalePackages(checker, "zh_CN", "deepin-software-center")
	assert.Empty(t, result)
}

func TestQueryEnhancedLocalePackagesNoPkgs(t *testing.T) {
	checker := func(s string) bool { return true }

	result := QueryEnhancedLocalePackages(checker, "zh_CN")
	assert.Nil(t, result)
}
