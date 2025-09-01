// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	// "os/exec"
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

// TODO:(DingHao) fix to udisksctl
func GetRootDiskFreeSpace() (uint64, error) {
	freeSpace, err := runcmd.RunnerOutput(10, "bash", "-c", "df -l --output=avail / | tail -n 1")
	if err != nil {
		return 0, err
	}

	// 将输出结果转换为字符串并去除空格
	freeSpaceStr := strings.TrimSpace(string(freeSpace))

	// 将字符串转换为uint64类型的整数
	freeSpaceInt, err := strconv.ParseUint(freeSpaceStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return freeSpaceInt, nil
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

	freeSpace, err := runcmd.RunnerOutput(10, "bash", "-c", "df -l --output=avail /data | tail -n 1")
	if err != nil {
		return 0, err
	}

	// 将输出结果转换为字符串并去除空格
	freeSpaceStr := strings.TrimSpace(string(freeSpace))

	// 将字符串转换为uint64类型的整数
	freeSpaceInt, err := strconv.ParseUint(freeSpaceStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return freeSpaceInt, nil
}
