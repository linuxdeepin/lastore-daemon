/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"fixme/pkg_recommend"
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
