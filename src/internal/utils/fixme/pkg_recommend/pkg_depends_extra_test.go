// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package pkg_recommend

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnhancedLocalePackages(t *testing.T) {
	pkgs := GetEnhancedLocalePackages("zh_CN.UTF-8", "gvfs")
	// On a system with i18n_dependent.json, this may return packages or nil
	// Just verify it doesn't panic
	_ = pkgs
}

func TestGetEnhancedLocalePackagesInvalidLocale(t *testing.T) {
	pkgs := GetEnhancedLocalePackages("xx_XX.INVALID", "gvfs")
	assert.Nil(t, pkgs)
}

func TestGetByPackage(t *testing.T) {
	pkgs, conflicts, err := GetByPackage("zh_CN.UTF-8", "gvfs")
	assert.NoError(t, err)
	_ = pkgs
	_ = conflicts
}

func TestGetByPackageInvalidLocale(t *testing.T) {
	_, _, err := GetByPackage("xx_XX.INVALID", "gvfs")
	assert.NoError(t, err)
}

func TestGetByLocale(t *testing.T) {
	infos, conflicts, err := GetByLocale("zh_CN.UTF-8")
	assert.NoError(t, err)
	_ = infos
	_ = conflicts
}

func TestGetByLocaleInvalidLocale(t *testing.T) {
	infos, conflicts, err := GetByLocale("xx_XX.INVALID")
	assert.NoError(t, err)
	// With invalid locale, infos should be empty (no matching lang code)
	_ = infos
	_ = conflicts
}
