// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package pkg_recommend

import (
	"path/filepath"
	"testing"
)

// useTestDataPaths 把 i18n 数据文件路径切换到仓库内置的测试数据，
// 使测试不依赖构建环境中的 /usr/share/i18n 数据文件。
func useTestDataPaths(t *testing.T) {
	t.Helper()

	origLangInfo := LangInfoFile
	origSupported := langSupportedFile
	origDepends := pkgDependsFile

	LangInfoFile = filepath.Join("testdata", "support_languages.json")
	langSupportedFile = filepath.Join("testdata", "SUPPORTED")
	pkgDependsFile = filepath.Join("pkg_depends.json")

	t.Cleanup(func() {
		LangInfoFile = origLangInfo
		langSupportedFile = origSupported
		pkgDependsFile = origDepends
	})
}
