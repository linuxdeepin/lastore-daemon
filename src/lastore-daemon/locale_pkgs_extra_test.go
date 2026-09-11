// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/utils/fixme/pkg_recommend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestQueryEnhancedLocalePackagesNonEmpty(t *testing.T) {
	oldDepends := pkg_recommend.PkgDependsFile
	oldLangInfo := pkg_recommend.LangInfoFile
	t.Cleanup(func() {
		pkg_recommend.PkgDependsFile = oldDepends
		pkg_recommend.LangInfoFile = oldLangInfo
	})

	// Point pkg_recommend at hermetic fixtures so the result does not depend on
	// the build host's /usr/share/i18n data.
	dir := t.TempDir()
	pkg_recommend.PkgDependsFile = filepath.Join(dir, "i18n_dependent.json")
	pkg_recommend.LangInfoFile = filepath.Join(dir, "language_info.json")

	require.NoError(t, os.WriteFile(pkg_recommend.PkgDependsFile, []byte(`{
  "PkgDepends": [
    {
      "Category": "tr",
      "PkgInfos": [
        {"FormatType": 1, "DependentPkg": "gvfs", "PkgPull": "language-pack-gnome-"}
      ]
    }
  ]
}`), 0644))
	require.NoError(t, os.WriteFile(pkg_recommend.LangInfoFile, []byte(`{
  "LanguageList": [
    {"Locale": "zh_CN.UTF-8", "Description": "简体中文", "LangCode": "zh-hans", "CountryCode": "CN"}
  ]
}`), 0644))

	checker := func(s string) bool { return true }

	result := QueryEnhancedLocalePackages(checker, "zh_CN.UTF-8", "gvfs")
	assert.NotEmpty(t, result)
}
