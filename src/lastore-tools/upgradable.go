// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
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

// return the pkgs from apt dist-upgrade
// NOTE: the result strim the arch suffix
func listDistUpgradePackages(sourcePath string) ([]string, error) {
	args := []string{
		"-c", system.LastoreAptV2CommonConfPath,
		"dist-upgrade", "--assume-no",
		"-o", "Debug::NoLocking=1",
	}
	if info, err := os.Stat(sourcePath); err == nil {
		if info.IsDir() {
			args = append(args, "-o", "Dir::Etc::SourceList=/dev/null")
			args = append(args, "-o", "Dir::Etc::SourceParts="+sourcePath)
		} else {
			args = append(args, "-o", "Dir::Etc::SourceList="+sourcePath)
			args = append(args, "-o", "Dir::Etc::SourceParts=/dev/null")
		}
	} else {
		return nil, err
	}

	cmd := exec.Command("apt-get", args...) // #nosec G204
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	// NOTE: 这里不能使用命令的退出码来判断，因为 --assume-no 会让命令的退出码为 1
	_ = cmd.Run()

	const upgraded = "The following packages will be upgraded:"
	const newInstalled = "The following NEW packages will be installed:"
	if bytes.Contains(outBuf.Bytes(), []byte(upgraded)) ||
		bytes.Contains(outBuf.Bytes(), []byte(newInstalled)) {

		p := parseAptShowList(bytes.NewReader(outBuf.Bytes()), upgraded)
		p = append(p, parseAptShowList(bytes.NewReader(outBuf.Bytes()), newInstalled)...)
		return p, nil
	}

	err := apt.ParsePkgSystemError(outBuf.Bytes(), errBuf.Bytes())
	return nil, err
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
	ps, err := listDistUpgradePackages(sourcePath)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, nil
	}
	cmd := exec.Command("apt", append([]string{"-c", system.LastoreAptV2CommonConfPath,
		"-o", fmt.Sprintf("Dir::Etc::SourceList=%s", system.SystemSourceFile),
		"-o", fmt.Sprintf("Dir::Etc::SourceParts=%s", system.OriginSourceDir),
		"list", "--upgradable"}, ps...)...) // #nosec G204
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		logger.Errorf("LockDo: %v\n", err)
	}
	timer := time.AfterFunc(time.Second*10, func() {
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
	foreignArchs, err := exec.Command("dpkg", "--print-foreign-architectures").Output()
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

func GenerateUpdateInfos(fpath string) error {
	err := system.UpdateUnknownSourceDir()
	if err != nil {
		logger.Warning(err)
	}

	err = system.UpdateSystemSourceDir()
	if err != nil {
		logger.Warning(err)
	}

	var upgradeInfo []system.UpgradeInfo
	for category, sourcePath := range system.GetCategorySourceMap() {
		lines, err := queryDpkgUpgradeInfoByAptList(sourcePath)
		if err != nil {
			if os.IsNotExist(err) { // 该类型源文件不存在时,无需将错误写入到文件中
				logger.Info(err)
			} else {
				var updateInfoErr system.UpdateInfoError
				pkgSysErr, ok := err.(*system.PkgSystemError)
				if ok {
					updateInfoErr.Type = pkgSysErr.GetType()
					updateInfoErr.Detail = pkgSysErr.GetDetail()
				} else {
					updateInfoErr.Type = "unknown"
					updateInfoErr.Detail = err.Error()
				}
				return writeData(fpath, updateInfoErr)
			}
		} else {
			upgradeInfo = append(upgradeInfo, mapUpgradeInfo(
				lines,
				buildUpgradeInfoRegex(getSystemArchitectures()),
				buildUpgradeInfo,
				category.JobType())...)
		}
	}
	return writeData(fpath, upgradeInfo)
}
