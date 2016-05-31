/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
/*
This is system package manager need implement for porting
lastore-daemon
*/
package system

import (
	"bufio"
	"fmt"
	"internal/utils"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ListPackageFile list files path contained in the packages
func ListPackageFile(packages ...string) []string {
	desktopFiles, err := utils.FilterExecOutput(
		exec.Command("dpkg", append([]string{"-L", "--"}, packages...)...),
		time.Second*2,
		func(string) bool { return true },
	)
	if err != nil {
		return nil
	}
	return desktopFiles
}

// QueryPackageDependencies return the directly dependencies
func QueryPackageDependencies(pkgId string) []string {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Depends}", "--", pkgId).CombinedOutput()
	if err != nil {
		return nil
	}
	baseName := guestBasePackageName(pkgId)

	var r []string
	for _, line := range strings.Fields(string(out)) {
		if strings.Contains(line, baseName) {
			r = append(r, strings.Trim(line, ","))
		}
	}
	return r
}

// QueryPackageDownloadSize parsing the total size of download archives when installing
// the packages.
func QueryPackageDownloadSize(packages ...string) (float64, error) {
	cmd := exec.Command("/usr/bin/apt-get",
		append([]string{"-d", "-o", "Debug::NoLocking=1", "--print-uris", "--assume-no", "install", "--"}, packages...)...)

	lines, err := utils.FilterExecOutput(cmd, time.Second*10, func(line string) bool {
		_, _err := parsePackageSize(line)
		return _err == nil
	})

	if len(lines) != 0 {
		return parsePackageSize(lines[0])
	}
	return SizeUnknown, err
}

// QueryPackageInstalled query whether the pkgId installed
func QueryPackageInstalled(pkgId string) bool {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Status}", "--", pkgId).CombinedOutput()
	if err != nil {
		return false
	}
	if strings.Contains(string(out), "ok not-installed") {
		return false
	} else if strings.Contains(string(out), "install ok installed") {
		return true
	}
	return false

}

// QueryPackageInstallable query whether the pkgId can be installed
func QueryPackageInstallable(pkgId string) bool {
	_, err := exec.Command("/usr/bin/apt-cache", "show", "--", pkgId).CombinedOutput()
	if err != nil {
		return false
	}
	return true
}

// SystemArchitectures return the system package manager supported architectures
func SystemArchitectures() ([]Architecture, error) {
	foreignArchs, err := exec.Command("dpkg", "--print-foreign-architectures").Output()
	if err != nil {
		return nil, fmt.Errorf("GetSystemArchitecture failed:%v %v\n", foreignArchs, err)
	}

	arch, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		return nil, fmt.Errorf("GetSystemArchitecture failed:%v %v\n", foreignArchs, err)
	}

	var r []Architecture
	if v := Architecture(strings.TrimSpace(string(arch))); v != "" {
		r = append(r, v)
	}
	for _, a := range strings.Split(strings.TrimSpace(string(foreignArchs)), "\n") {
		if v := Architecture(a); v != "" {
			r = append(r, v)
		}
	}
	return r, nil
}

var defaultRepoInfo = RepositoryInfo{
	Name:   "desktop",
	Url:    "http://packages.deepin.com/deepin",
	Mirror: "http://cdn.packages.deepin.com/deepin",
}

func init() {
	err := DecodeJson(path.Join(VarLibDir, "repository_info.json"), &RepoInfos)
	if err != nil {
		RepoInfos = []RepositoryInfo{defaultRepoInfo}
	}
}

func DetectDefaultRepoInfo(rInfos []RepositoryInfo) RepositoryInfo {
	f, err := os.Open("/etc/apt/sources.list")
	if err != nil {
		return defaultRepoInfo
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		bs, _, err := r.ReadLine()
		if err != nil {
			break
		}
		line := strings.TrimLeft(string(bs), " ")
		if strings.IndexByte(line, '#') == 0 {
			continue
		}

		for _, repo := range rInfos {
			if strings.Contains(line, " "+repo.Url+" ") {
				return repo
			}
		}
	}
	return defaultRepoInfo
}

func guestBasePackageName(pkgId string) string {
	for _, sep := range []string{"-", ":", "_"} {
		index := strings.LastIndex(pkgId, sep)
		if index != -1 {
			return pkgId[:index]
		}
	}
	return pkgId
}

// see the apt code of command-line/apt-get.c:895
var __ReDownloadSize__ = regexp.MustCompile("Need to get ([0-9,.]+) ([kMGTPEZY]?)B(/[0-9,.]+ [kMGTPEZY]?B)? of archives")

var __unitTable__ = map[byte]float64{
	'k': 1000,
	'M': 1000 * 1000,
	'G': 1000 * 1000 * 1000,
	'T': 1000 * 1000 * 1000 * 1000,
	'P': 1000 * 1000 * 1000 * 1000 * 1000,
	'E': 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	'Z': 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	'Y': 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
}

const SizeDownloaded = 0
const SizeUnknown = -1

func parsePackageSize(line string) (float64, error) {
	ms := __ReDownloadSize__.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 3, 4:
		l := strings.Replace(string(ms[1]), ",", "", -1)
		size, err := strconv.ParseFloat(l, 64)
		if err != nil {
			return SizeUnknown, fmt.Errorf("%q invalid : %v err")
		}
		if len(ms[2]) == 0 {
			return size, nil
		}
		unit := ms[2][0]
		return size * __unitTable__[unit], nil
	}
	return SizeUnknown, fmt.Errorf("%q invalid", line)
}
