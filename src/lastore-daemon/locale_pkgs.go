// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils/fixme/pkg_recommend"
)

func QueryEnhancedLocalePackages(checker func(string) bool, lang string, pkgs ...string) []string {
	set := make(map[string]struct{})
	for _, pkg := range pkgs {
		for _, localePkg := range pkg_recommend.GetEnhancedLocalePackages(lang, pkg) {
			set[localePkg] = struct{}{}
		}
	}

	var r []string
	for pkg := range set {
		if checker(pkg) {
			r = append(r, pkg)
		}
	}
	return r
}
