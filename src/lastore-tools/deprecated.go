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
	"bufio"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
)

type DesktopInfo struct {
	FilePath string
	Package  string
	Icon     string
	Exec     string
}

func BuildDesktopDirectories() []string {
	var scanDirectories map[string]struct{} = map[string]struct{}{
		"/usr/share/applications":             struct{}{},
		"/usr/share/applications/kde4":        struct{}{},
		"/usr/local/share/applications":       struct{}{},
		"/usr/local/share/applications/kde4":  struct{}{},
		"/usr/share/deepin/applications":      struct{}{},
		"/usr/share/deepin/applications/kde4": struct{}{},
	}
	xdg_data_home := os.Getenv("$XDG_DATA_HOME")
	if xdg_data_home == "" {
		xdg_data_home = os.ExpandEnv("$HOME/.local/share")
	}
	scanDirectories[path.Join(xdg_data_home, "applications")] = struct{}{}
	for _, dir := range strings.Split(os.Getenv("$XDG_DATA_DIR"), ":") {
		scanDirectories[path.Join(dir, "applications")] = struct{}{}
	}
	var r []string
	for dir := range scanDirectories {
		r = append(r, dir)
	}
	return r
}

func GetDesktopFiles(dirs []string) []string {
	var r []string
	for _, dir := range dirs {
		fs, err := ioutil.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, finfo := range fs {
			name := finfo.Name()
			if strings.HasSuffix(name, ".desktop") {
				r = append(r, path.Join(dir, finfo.Name()))
			}
		}
	}
	return r
}

// GenerateDesktopIndexes 生成 desktop 相关的查询表
// 1. desktop --> icon
// 2. desktop --> exec
// 3. desktop --> package
func GenerateDesktopIndexes(baseDir string) error {
	os.MkdirAll(baseDir, 0755)

	packageIndex, installTimeIndex := ParsePackageInfos()
	if err := writeData(path.Join(baseDir, "pacakge_installedTime.json"), installTimeIndex); err != nil {
		return err
	}

	if d, err := BuildDesktop2uaid(); err == nil {
		for k, v := range d {
			packageIndex[k] = v
		}
	} else {
		return err
	}
	packageIndex = mergeDesktopIndex(packageIndex, path.Join(baseDir, "desktop_package.json"))

	var execInfo, iconInfo = make(map[string]string), make(map[string]string)
	for _, dPath := range GetDesktopFiles(BuildDesktopDirectories()) {
		info := ParseDesktopInfo(packageIndex, dPath)
		if info == nil {
			continue
		}
		execInfo[info.Package] = info.Exec
		iconInfo[info.Package] = info.Icon
	}

	mergeDesktopIndex(execInfo, path.Join(baseDir, "package_exec.json"))
	mergeDesktopIndex(iconInfo, path.Join(baseDir, "package_icon.json"))

	return nil
}

var iconRegexp = regexp.MustCompile(`Icon=(.+)`)
var execRegexp = regexp.MustCompile("Exec=(.+)")

// ParseDesktopInfo 根据文件列表返回分析结果
func ParseDesktopInfo(packagesIndex map[string]string, dPath string) *DesktopInfo {
	f, err := os.Open(dPath)
	if err != nil {
		fmt.Println("ParseDesktopInfo:", err)
		return nil
	}
	defer f.Close()

	buf := bufio.NewReader(f)

	var icon, exec string

	var line string
	for err == nil {
		line, err = buf.ReadString('\n')
		rr := iconRegexp.FindStringSubmatch(line)
		if len(rr) == 2 {
			icon = rr[1]
		}
		rr = execRegexp.FindStringSubmatch(line)
		if len(rr) == 2 {
			exec = rr[1]
		}
		if icon != "" && exec != "" {
			break
		}
	}

	pkg := packagesIndex[path.Base(dPath)]
	if pkg == "" {
		pkg = path.Base(dPath)
	}
	info := DesktopInfo{
		Package: pkg,
		Icon:    icon,
		Exec:    exec,
	}

	return &info
}

func getDesktopFilePaths(listFilePath string) []string {
	f, err := os.Open(listFilePath)
	if err != nil {
		fmt.Println("getDesktopFilePaths:", err)
		return nil
	}
	defer f.Close()

	var r []string

	var line string
	buf := bufio.NewReader(f)
	for err == nil {
		line, err = buf.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".desktop") {
			r = append(r, line)
		}
	}
	return r
}

func getPackageName(name string) string {
	if len(name) <= 5 {
		return name
	}
	baseName := name[:len(name)-5]

	ns := strings.SplitN(baseName, ":", -1)
	if len(ns) != 0 {
		return ns[0]
	}
	return name
}

// ParsePackageInfos parsing the desktop files belong packages
// and the installing time of packages by parsing
// /var/lib/dpkg/info/$PKGNAME.list
func ParsePackageInfos() (map[string]string, map[string]int64) {
	var r = make(map[string]string)
	var t = make(map[string]int64)

	fs, err := ioutil.ReadDir("/var/lib/dpkg/info")
	if err != nil {
		log.Warnf("ParsePackageInfos :%v\n", err)
		return r, t
	}

	for _, finfo := range fs {
		name := finfo.Name()
		if strings.HasSuffix(name, ".list") {
			packageName := getPackageName(name)
			desktopFiles := getDesktopFilePaths(path.Join("/var/lib/dpkg/info", name))
			if len(desktopFiles) == 0 {
				continue
			}
			for _, f := range desktopFiles {
				r[f] = packageName
				r[path.Base(f)] = packageName
			}
			t[packageName] = finfo.ModTime().Unix()
		}
	}
	return r, t
}

func mergeDesktopIndex(infos map[string]string, fpath string) map[string]string {
	var old = make(map[string]string)
	if content, err := ioutil.ReadFile(fpath); err == nil {
		if err := json.Unmarshal(content, &old); err != nil {
			log.Warnf("mergeDesktopIndex:%q %v\n", fpath, err)
		}

	}
	for key, value := range infos {
		if key != "" && value != "" {
			old[key] = value
		}
	}
	writeData(fpath, old)
	return old
}
