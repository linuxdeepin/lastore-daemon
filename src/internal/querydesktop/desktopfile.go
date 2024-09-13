// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package querydesktop

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/linuxdeepin/go-lib/keyfile"
)

// TODO: write tools to analyze the score of desktop in debs
// which has more then one of desktop files.
// So we can know whether it is a reliable way to detect right desktop file.

type DesktopFiles struct {
	PkgName string
	Files   []string
}

func (fs DesktopFiles) Len() int {
	return len(fs.Files)
}
func (fs DesktopFiles) Swap(i, j int) {
	fs.Files[i], fs.Files[j] = fs.Files[j], fs.Files[i]
}
func (fs DesktopFiles) Less(i, j int) bool {
	si, sj := fs.score(i), fs.score(j)
	if si == sj {
		return len(fs.Files[i]) > len(fs.Files[j])
	}
	return si < sj
}

func (fs DesktopFiles) BestOne() string {
	if len(fs.Files) == 0 {
		return ""
	}
	sort.Sort(fs)
	return fs.Files[len(fs.Files)-1]
}

func (fs DesktopFiles) score(i int) int {
	var score int
	bs, err := os.ReadFile(fs.Files[i])
	if err != nil {
		return -10
	}

	fpath := fs.Files[i]
	if strings.Contains(fpath, fs.PkgName) {
		score = score + 20
	}

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
	if strings.Contains(fpath, "qtcreator/templates") {
		score = score - 5
	}
	if strings.Contains(fpath, "autostart") {
		score = score - 1
	}
	if strings.Contains(fpath, "desktop-base") {
		score = score - 5
	}
	if strings.Contains(fpath, "xgreeters") {
		score = score - 5
	}
	// End black list

	return score
}

const (
	flatpakAppPkgPrefix = "deepin-fpapp-"
	flatpakAppsDir      = "/var/lib/flatpak/exports/share/applications"
	desktopExt          = ".desktop"
)

// QueryDesktopFile 根据软件包名 pkg 返回包内最好的一个 desktop 文件。
// 此功能可能随着 deepin-app-store 的升级而被废弃。
func QueryDesktopFile(pkg string) string {
	if strings.HasPrefix(pkg, flatpakAppPkgPrefix) {
		appId := pkg[len(flatpakAppPkgPrefix):]
		files, _ := os.ReadDir(flatpakAppsDir)
		for _, fi := range files {
			name := fi.Name()
			if strings.HasSuffix(name, desktopExt) && !fi.IsDir() {
				name0 := name[:len(name)-len(desktopExt)] // trim suffix
				if strings.ToLower(name0) == appId {
					return filepath.Join(flatpakAppsDir, fi.Name())
				}
			}
		}

		// search desktop file Exec field
		for _, fi := range files {
			if fi.IsDir() {
				continue
			}
			desktopFile := filepath.Join(flatpakAppsDir, fi.Name())
			kf := keyfile.NewKeyFile()
			err := kf.LoadFromFile(desktopFile)
			if err != nil {
				continue
			}
			cmdline, _ := kf.GetString("Desktop Entry", "Exec")
			if strings.Contains(cmdline, appId) ||
				strings.Contains(strings.ToLower(cmdline), appId) {
				return desktopFile
			}
		}
	}

	all := ListDesktopFiles(pkg)
	ret := DesktopFiles{pkg, all}.BestOne()
	if ret == "" {
		ret = QueryDesktopFilePathByDependencies(pkg)
	}
	return ret
}
