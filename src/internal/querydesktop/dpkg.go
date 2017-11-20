/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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

package querydesktop

import (
	"internal/system"
	"internal/utils"
	"path"
	"strings"
)

// QueryDesktopFilePath return the most possible right
// desktop file in the pkgId.
// It will parsing pkgId plus all dependencies of it.
func QueryDesktopFilePathByDependencies(pkgId string) string {
	var r []string
	found := make(chan bool, 1)
	ch := queryRelateDependencies(found, pkgId, nil)
	for pkgname := range ch {
		for _, f := range system.ListPackageFile(pkgname) {
			if path.Base(f) == pkgId+".desktop" {
				found <- true
				return f
			}
			if strings.HasSuffix(f, ".desktop") {
				r = append(r, f)
			}
		}
	}
	return DesktopFiles{pkgId, r}.BestOne()
}

// QueryPackageSameNameDepends try find the packages which possible
// contain the right desktop file.
// e.g.
//    stardict-gtk --> stardict-common
//    stardict-gnome --> stardict-common
//    evince --> evince-common
//    evince-gtk --> evince, evince-common  Note: (recursion guest)
func queryRelateDependencies(stopCh chan bool, pkgId string, set map[string]struct{}) chan string {
	ch := make(chan string, 1)
	if set == nil {
		set = map[string]struct{}{pkgId: struct{}{}}
		ch <- pkgId
	}

	go func() {
		defer close(ch)
		for _, p := range system.QueryPackageDependencies(pkgId) {
			if _, ok := set[p]; ok {
				continue
			}

			if !system.QueryPackageInstalled(p) {
				continue
			}

			set[p] = struct{}{}
			select {
			case <-stopCh:
				return
			case ch <- p:
			}

			for x := range queryRelateDependencies(stopCh, p, set) {
				set[x] = struct{}{}
				select {
				case <-stopCh:
					return
				case ch <- p:
				}
			}
		}
	}()

	return ch
}

func ListDesktopFiles(pkg string) []string {
	var ret []string
	for _, p := range ListPkgsFiles(QuerySameSourcePkgs(pkg)) {
		if strings.HasSuffix(p, ".desktop") {
			ret = append(ret, p)
		}
	}
	return ret
}

func ListPkgsFiles(pkgs []string) []string {
	if len(pkgs) == 0 {
		return nil
	}
	s, err := utils.RunCommand("dpkg", append([]string{"-L"}, pkgs...)...)
	if err != nil {
		return nil
	}
	return strings.Split(s, "\n")
}

func QuerySameSourcePkgs(pkg string) []string {
	src, ok := __B2S__[pkg]
	if !ok {
		return nil
	}
	return __S2B__[src]
}

var __S2B__ map[string][]string
var __B2S__ map[string]string

func InitDB() {
	__S2B__, __B2S__ = groupBySource()
}

func groupBySource() (map[string][]string, map[string]string) {
	p2s := make(map[string]string)
	ret := make(map[string][]string)
	s, err := utils.RunCommand("dpkg-query", "-W", "-f", `${source} ${package}\n`)
	if err != nil {
		return ret, p2s
	}

	for _, line := range strings.Split(s, "\n") {
		var src, bin string
		fields := strings.Split(strings.TrimLeft(line, " "), " ")
		switch len(fields) {
		case 1:
			src = fields[0]
			bin = src
		case 2:
			src = fields[0]
			bin = fields[1]
		case 3:
			src = fields[0]
			bin = fields[2]
		default:
			continue
		}
		p2s[bin] = src
		ret[src] = append(ret[src], bin)
	}
	return ret, p2s
}
