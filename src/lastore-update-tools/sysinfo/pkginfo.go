// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

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
	outputStream, err := runcmd.RunnerOutput(10, "bash", "-c", "dpkg -l | tail -n +6 | awk '{print $1,$2,$3}'")
	if err != nil {
		return err
	}
	// logger.Debug(outputStream)

	outputLines := strings.Split(outputStream, "\n")

	// logger.Debugf("out:+%v", outputLines)

	for _, line := range outputLines {
		spv := strings.Split(line, " ")
		if len(spv) != 3 {
			// logger.Debugf("skip line: %v", spv)
			continue
		}
		appInfo := cache.AppTinyInfo{
			Name:    strings.Split(spv[1], ":")[0],
			Version: spv[2],
			State:   cache.PkgState(spv[0]),
		}
		// logger.Debugf("pkg:%+v", appInfo)

		pkgs[appInfo.Name] = &appInfo
		pkgs[fmt.Sprintf("%s#%s", appInfo.Name, appInfo.Version)] = &appInfo

	}

	return nil
}

// ToDo:(DingHao)替换成袁老师的hash函数
func GetSysPkgStateAndVersion(pkgname string) (string, string, error) {
	command := "bash"
	arg1 := "-c"
	arg2 := "dpkg -l | tail -n +6 | awk '{print $1,$2,$3}'|grep \"^.. " + pkgname + " \""
	cmd := exec.Command(command, arg1, arg2)
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	pkgInfo := strings.Split(string(output), " ")
	if len(pkgInfo) != 3 {
		// log.Debugf("failed format: %s len: %d", pkgInfo, len(pkgInfo))
		return "", "", fmt.Errorf("failed format: %s len: %d", pkgInfo, len(pkgInfo))
	}
	return pkgInfo[0], pkgInfo[2], nil
}
