// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
)

func buildUpgradeInfoRegex(archs []system.Architecture) *regexp.Regexp {
	archAlphabet := "all"
	for _, arch := range archs {
		archAlphabet = archAlphabet + string(arch)
	}
	s := `^(.*)\/.*\s+(.*)\s+([` + archAlphabet + `]+)\s+\[upgradable from:\s+(.*)\s?\]$`
	return regexp.MustCompile(s)
}

func buildUpgradeInfo(needle *regexp.Regexp, line string) *system.UpgradeInfo {
	ms := needle.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 5:
		return &system.UpgradeInfo{
			Package:        string(ms[1]),
			CurrentVersion: string(ms[4]),
			LastVersion:    string(ms[2]),
		}
	}
	return nil
}

func mapUpgradeInfo(lines []string, needle *regexp.Regexp, fn func(*regexp.Regexp, string) *system.UpgradeInfo, category string) []system.UpgradeInfo {
	var infos []system.UpgradeInfo
	for _, line := range lines {
		info := fn(needle, line)
		if info == nil {
			continue
		}
		info.Category = category
		infos = append(infos, *info)
	}
	return infos
}

func parseAptShowList(r io.Reader, title string) []string {
	buf := bufio.NewReader(r)

	var p []string

	var line string
	in := false

	var err error
	for err == nil {
		line, err = buf.ReadString('\n')
		if strings.TrimSpace(title) == strings.TrimSpace(line) {
			in = true
			continue
		}

		if !in {
			continue
		}

		if !strings.HasPrefix(line, " ") {
			break
		}

		for _, f := range strings.Fields(line) {
			p = append(p, strings.Split(f, ":")[0])
		}
	}

	return p
}

func queryDpkgUpgradeInfoByAptList(sourcePath string) ([]string, error) {
	ps, err := apt.ListDistUpgradePackages(sourcePath, nil)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, nil
	}
	args := []string{
		"-c", system.LastoreAptV2CommonConfPath,
	}
	if info, err := os.Stat(sourcePath); err == nil {
		if info.IsDir() {
			args = append(args, "-o", "Dir::Etc::SourceList=/dev/null")
			args = append(args, "-o", "Dir::Etc::SourceParts="+sourcePath)
		} else {
			args = append(args, "-o", "Dir::Etc::SourceList="+sourcePath)
			args = append(args, "-o", "Dir::Etc::SourceParts=/dev/null")
		}
	}
	args = append(args, []string{"list", "--upgradable"}...)
	cmd := exec.Command("apt", append(args, ps...)...) // #nosec G204
	cmd.Env = append(os.Environ(), "IMMUTABLE_DISABLE_REMOUNT=true")
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		logger.Errorf("LockDo: %v\n", err)
	}
	timer := time.AfterFunc(time.Second*120, func() {
		_ = cmd.Process.Signal(syscall.SIGINT)
	})

	buf := bytes.NewBuffer(nil)

	_, err = buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	var lines []string
	var line string
	for ; err == nil; line, err = buf.ReadString('\n') {
		lines = append(lines, strings.TrimSpace(line))
	}
	err = cmd.Wait()
	if err != nil {
		return nil, err
	}
	timer.Stop()
	return lines, nil
}

func getSystemArchitectures() []system.Architecture {
	cmd := exec.Command("dpkg", "--print-foreign-architectures")
	cmd.Env = append(os.Environ(), "IMMUTABLE_DISABLE_REMOUNT=true")
	foreignArchs, err := cmd.Output()
	if err != nil {
		logger.Warningf("GetSystemArchitecture failed:%v\n", foreignArchs)
	}

	arch, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		logger.Warningf("GetSystemArchitecture failed:%v\n", foreignArchs)
	}

	var r []system.Architecture
	if v := system.Architecture(strings.TrimSpace(string(arch))); v != "" {
		r = append(r, v)
	}
	for _, a := range strings.Split(strings.TrimSpace(string(foreignArchs)), "\n") {
		if v := system.Architecture(a); v != "" {
			r = append(r, v)
		}
	}
	return r
}

func GenerateUpdateInfos(outputPath string) error {
	var upgradeInfo []system.UpgradeInfo
	for _, category := range system.AllInstallUpdateType() {
		sourcePath := system.GetCategorySourceMap()[category]
		lines, err := queryDpkgUpgradeInfoByAptList(sourcePath)
		if err != nil {
			if os.IsNotExist(err) { // 该类型源文件不存在时,无需将错误写入到文件中
				logger.Info(err)
			} else {
				// 错误写到error_update_infos.json文件中
				outputErrorPath := fmt.Sprintf("error_%v", outputPath)
				var updateInfoErr system.UpdateInfoError
				var pkgSysErr *system.JobError
				ok := errors.As(err, &pkgSysErr)
				if ok {
					updateInfoErr.Type = pkgSysErr.GetType()
					updateInfoErr.Detail = pkgSysErr.GetDetail()
				} else {
					updateInfoErr.Type = "unknown"
					updateInfoErr.Detail = err.Error()
				}
				_ = writeData(outputErrorPath, updateInfoErr)
			}
		} else {
			upgradeInfo = append(upgradeInfo, mapUpgradeInfo(
				lines,
				buildUpgradeInfoRegex(getSystemArchitectures()),
				buildUpgradeInfo,
				category.JobType())...)
		}
	}
	return writeData(outputPath, upgradeInfo)
}
