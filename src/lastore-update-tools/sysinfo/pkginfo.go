// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"fmt"
	"strings"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

var logger = log.NewLogger("lastore/update-tools/sysinfo")

// CheckAppIsExist
func CheckAppIsExist(app string) (bool, error) {
	if err := fs.CheckFileExistState(app); err != nil {
		return false, err
	}
	return true, nil
}

func GetCurrInstPkgStat(pkgs map[string]*cache.AppTinyInfo) error {
	// clear pkgs map
	for pkgName := range pkgs {
		delete(pkgs, pkgName)
	}

	// bash -c "dpkg -l | tail -n +6 | awk '{print $1,$2,$3}'"
	outputStream, err := runcmd.RunnerOutput(
		10,
		"dpkg-query",
		"-W",
		"-f=${db:Status-Abbrev} ${Package} ${Version}\n",
	)
	if err != nil {
		return fmt.Errorf("failed to run dpkg-query: %w", err)
	}

	logger.Debugf("dpkg-query output: %s", outputStream)
	outputLines := strings.Split(outputStream, "\n")

	for _, line := range outputLines {
		spv := strings.Fields(line)
		if len(spv) != 3 {
			logger.Debugf("skip line: %v", spv)
			continue
		}
		appInfo := cache.AppTinyInfo{
			Name:    strings.Split(spv[1], ":")[0],
			Version: spv[2],
			State:   cache.PkgState(spv[0]),
		}
		logger.Debugf("pkg:%+v", appInfo)

		pkgs[appInfo.Name] = &appInfo
		pkgs[fmt.Sprintf("%s#%s", appInfo.Name, appInfo.Version)] = &appInfo

	}

	return nil
}

// ToDo:(DingHao)替换成袁老师的hash函数
func GetSysPkgStateAndVersion(pkgname string) (string, string, error) {
	output, err := runcmd.RunnerOutput(
		10,
		"dpkg-query",
		"-W",
		"-f=${db:Status-Abbrev} ${Version}\n",
		pkgname,
	)
	if err != nil {
		return "", "", err
	}

	pkgInfo := strings.Fields(output)
	if len(pkgInfo) < 2 {
		return "", "", fmt.Errorf("failed format: %s len: %d", pkgInfo, len(pkgInfo))
	}
	return pkgInfo[0], pkgInfo[1], nil
}
