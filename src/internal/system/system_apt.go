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
	"fmt"
	"internal/utils"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ListPackageFile list files path contained in the packages
func ListPackageFile(packages ...string) []string {
	desktopFiles, err := utils.FilterExecOutput(
		exec.Command("dpkg", append([]string{"-L"}, packages...)...),
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
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Depends}", pkgId).CombinedOutput()
	if err != nil {
		return nil
	}
	baseName := guestBasePackageName(pkgId)

	var r []string
	for _, line := range strings.Fields(string(out)) {
		if strings.Contains(line, baseName) {
			r = append(r, line)
		}
	}
	return r
}

// QueryPackageDownloadSize parsing the total size of download archives when installing
// the packages.
func QueryPackageDownloadSize(packages ...string) float64 {
	cmd := exec.Command("/usr/bin/apt-get", append([]string{"-d", "-o", "Debug::NoLocking=1", "--print-uris", "--assume-no", "install"}, packages...)...)

	lines, err := utils.FilterExecOutput(cmd, time.Second*3, func(line string) bool {
		return parsePackageSize(line) != SizeUnknown
	})

	if len(lines) != 0 {
		return parsePackageSize(lines[0])
	}

	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			if status.ExitStatus() == 1 {
				// --assume-no will cause apt-get exit with code 1 when successfully
				return SizeDownloaded
			}
		}
	}
	return SizeUnknown
}

// QueryPackageInstalled query whether the pkgId installed
func QueryPackageInstalled(pkgId string) bool {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Status}", pkgId).CombinedOutput()
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
	_, err := exec.Command("/usr/bin/apt-cache", "show", pkgId).CombinedOutput()
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

func parsePackageSize(line string) float64 {
	ms := __ReDownloadSize__.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 3, 4:
		l := strings.Replace(string(ms[1]), ",", "", -1)
		size, err := strconv.ParseFloat(l, 64)
		if err != nil {
			return SizeUnknown
		}
		if len(ms[2]) == 0 {
			return size
		}
		unit := ms[2][0]
		return size * __unitTable__[unit]
	}
	return SizeUnknown
}
