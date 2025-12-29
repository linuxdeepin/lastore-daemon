// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	// "os/exec"
	"fmt"
	"strconv"
	"strings"

	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// func GetRootDiskFreeSpace() (uint64, error) {   //和通过df计算的有偏差
// 	var stat syscall.Statfs_t
// 	err := syscall.Statfs("/", &stat)
// 	if err != nil {
// 	return 0, err
// 	}

// 	// 计算剩余空间,以M为单位
// 	freeSpace := stat.Bavail * uint64(stat.Bsize) / 1024 / 1024
// 	return freeSpace, nil
// }

// func GetDataDiskFreeSpace() (uint64, error) {   //和通过df计算的有偏差
// 	var stat syscall.Statfs_t
// 	err := syscall.Statfs("/data", &stat)
// 	if err != nil {
// 	return 0, err
// 	}

// 	// 计算剩余空间,以M为单位
// 	freeSpace := stat.Bavail * uint64(stat.Bsize) / 1024 /1024
// 	return freeSpace, nil
// }

func parseDfAvailOuput(output string) (uint64, error) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected df output: %q", output)
	}
	return strconv.ParseUint(fields[len(fields)-1], 10, 64)
}

// TODO:(DingHao) fix to udisksctl
func GetRootDiskFreeSpace() (uint64, error) {
	freeSpace, err := runcmd.RunnerOutput(10, "df", "-l", "--output=avail", "/")
	if err != nil {
		return 0, err
	}

	return parseDfAvailOuput(freeSpace)
}

// TODO:(DingHao) fix to udisksctl
func GetDataDiskFreeSpace() (uint64, error) {
	if err := fs.CheckFileExistState("/data"); err != nil {
		sysSpace, err := GetRootDiskFreeSpace()
		if err != nil {
			return 0, err
		}
		return sysSpace, nil
	}

	freeSpace, err := runcmd.RunnerOutput(10, "df", "-l", "--output=avail", "/data")
	if err != nil {
		return 0, err
	}

	return parseDfAvailOuput(freeSpace)
}
