// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fixpkg

import (
	"fmt"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
)

func CheckConfig() error {
	// dpkg -l | tail -n +6 | awk '{print $1,$2,$3}' | egrep '^iH'
	outMsg, err := runcmd.RunnerOutput(10, "bash", "-c", "dpkg -l | tail -n +6 | grep ^iF | awk '{print $1,$2,$3}'")
	if err != nil {
		return nil
	}
	if len(outMsg) > 4 {
		log.Debugf("Have package configure error: %v", outMsg)

		return fmt.Errorf("find package configure error: %v", outMsg)
	}
	return nil
}

func CheckDpkgListStat() error {
	// dpkg -l | tail -n +6 | awk '{print $1,$2,$3}' | egrep '^iH'
	outMsg, err := runcmd.RunnerOutput(10, "bash", "-c", "dpkg -l | tail -n +6 | grep -v ^ii | awk '{print $1,$2,$3}'")
	if err != nil {
		log.Debugf("check/sys load package info failed: %v", err)
		return fmt.Errorf("check/sys load package info failed: %v", err)
	}
	breakDepends := false
	var breakDependsError error
	if len(outMsg) > 1 {

		stateList := strings.Split(outMsg, "\n")
		if len(stateList) > 0 {
			for _, pkg := range stateList {
				if len(pkg) > 4 {
					if depFailed, err := cache.PkgState(strings.Split(pkg, " ")[0]).CheckFailed(); !depFailed && err == nil {
						//log.Debugf("skip pkg:%v", pkg)
						continue
					}
					// pkginfo := strings.Split(pkg, " ")
					breakDepends = true
					log.Debugf("pkg:%v", pkg)
					if breakDependsError != nil {
						breakDependsError = fmt.Errorf("%v pkg :%v", breakDependsError, pkg)
					} else {
						breakDependsError = fmt.Errorf("pkg :%v", pkg)
					}

				}
			}
		}
		if breakDepends {
			return fmt.Errorf("found package depends:%v", breakDependsError)
		}
		// log.Debugf("Have package configure error: %v", outMsg)

		// return fmt.Errorf("find package configure error: %v", outMsg)
	}
	log.Debugf("check/sys package status check passed")
	log.Debugf("check/apt status start")

	out, err := runcmd.RunnerOutputEnv(180, "/usr/bin/bash", []string{""}, "-c", "/usr/bin/apt-get check 2>&1")

	if err != nil {
		log.Debugf("check/apt status failed :\n %s %v", out, err)
		return fmt.Errorf("apt-get check :%v %v", out, err)
	}
	log.Debugf("fix/check :\n %v", out)

	return nil
}
