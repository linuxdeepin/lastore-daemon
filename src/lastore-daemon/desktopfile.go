/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"internal/system"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
)

// TODO: write tools to analyze the score of desktop in debs
// which has two or more desktop files.
// So we can know whether it is a reliable way to detect right desktop file.

type DesktopFiles []string

func (fs DesktopFiles) Len() int {
	return len(fs)
}
func (fs DesktopFiles) Swap(i, j int) {
	fs[i], fs[j] = fs[j], fs[i]
}
func (fs DesktopFiles) Less(i, j int) bool {
	return fs.score(i) < fs.score(j)
}

func (fs DesktopFiles) BestOne() string {
	if len(fs) == 0 {
		return ""
	}
	sort.Sort(fs)
	return fs[len(fs)-1]
}

func (fs DesktopFiles) score(i int) int {
	var score int
	bs, err := ioutil.ReadFile(fs[i])
	if err != nil {
		return -10
	}

	fpath := fs[i]
	content := string(bs)

	// Begin desktop content feature detect
	if !strings.Contains(content, "Exec=") {
		score = score - 10
	}
	if strings.Contains(content, "[Desktop Entry]") {
		score = score + 1
	} else {
		score = score - 10
	}

	if strings.Contains(content, "TryExec") {
		score = score + 5
	}
	if strings.Contains(content, "Type=Application") {
		score = score + 5
	}
	if strings.Contains(content, "StartupNotify") {
		score = score + 5
	}
	if strings.Contains(content, "Icon") {
		score = score + 3
	} else {
		score = score - 3
	}

	if strings.Contains(content, "NoDisplay=true") {
		score = score - 100
	}
	// End desktop content feature detect

	// Begin XDG Scan
	// Check wheter the desktop file in xdg directories.
	var dirs map[string]struct{} = map[string]struct{}{
		"/usr/share/applications":             struct{}{},
		"/usr/share/applications/kde4":        struct{}{},
		"/usr/local/share/applications":       struct{}{},
		"/usr/local/share/applications/kde4":  struct{}{},
		"/usr/share/deepin/applications":      struct{}{},
		"/usr/share/deepin/applications/kde4": struct{}{},
	}
	for _, dir := range strings.Split(os.Getenv("$XDG_DATA_DIR"), ":") {
		dirs[path.Join(dir, "applications")] = struct{}{}
	}
	for dir := range dirs {
		if strings.Contains(fpath, dir) {
			score = score + 10
		}
	}
	// End XDG Scan

	// Begin black list
	if strings.Contains(fpath, "/xsessions/") {
		score = score - 10
	}
	if strings.Contains(fs[i], "qtcreator/templates") {
		score = score - 5
	}
	if strings.Contains(fs[i], "autostart") {
		score = score - 1
	}
	if strings.Contains(fs[i], "desktop-base") {
		score = score - 5
	}
	if strings.Contains(fs[i], "xgreeters") {
		score = score - 5
	}
	// End black list

	return score
}

// QueryDesktopFilePath return the most possible right
// desktop file in the pkgId.
// It will parsing pkgId plus all dependencies of it.
func QueryDesktopFilePath(pkgId string) string {
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
	return DesktopFiles(r).BestOne()
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
