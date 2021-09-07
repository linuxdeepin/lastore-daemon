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
	"bytes"
	"internal/system"
	"internal/system/apt"
	"io"
	"os"
	"os/exec"
	"path"
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

func mapUpgradeInfo(lines []string, needle *regexp.Regexp, fn func(*regexp.Regexp, string) *system.UpgradeInfo) []system.UpgradeInfo {
	var infos []system.UpgradeInfo
	for _, line := range lines {
		info := fn(needle, line)
		if info == nil {
			continue
		}
		infos = append(infos, *info)
	}
	return infos
}

// return the pkgs from apt dist-upgrade
// NOTE: the result strim the arch suffix
func listDistUpgradePackages(useCustomConf bool) ([]string, error) {
	var cmd *exec.Cmd
	if useCustomConf {
		cmd = exec.Command("apt-get", "-c", system.LastoreAptV2ConfPath, "dist-upgrade", "--assume-no", "-o", "Debug::NoLocking=1")
	} else {
		cmd = exec.Command("apt-get", "dist-upgrade", "--assume-no", "-o", "Debug::NoLocking=1")
	}
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

func queryDpkgUpgradeInfoByAptList() ([]string, error) {
	var cmd *exec.Cmd
	// 判断自定义更新使用的list文件是否存在,sources.list.d可以为空
	var queryByCustomConf bool
	_, err := os.Stat("/var/lib/lastore/sources.list.d")
	if err != nil {
		config := struct {
			UpdateMode uint64
		}{}
		err := system.DecodeJson(path.Join(system.VarLibDir, "config.json"), &config)
		if err == nil {
			err := system.UpdateCustomSourceDir(config.UpdateMode)
			if err != nil {
				logger.Warning(err)
				queryByCustomConf = false
			} else {
				queryByCustomConf = true
			}
		} else {
			queryByCustomConf = false
		}
	} else {
		queryByCustomConf = true
	}

	ps, err := listDistUpgradePackages(queryByCustomConf)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, nil
	}
	if queryByCustomConf {
		cmd = exec.Command("apt", append([]string{"-c", system.LastoreAptV2ConfPath, "list", "--upgradable"}, ps...)...)
	} else {
		cmd = exec.Command("apt", append([]string{"list", "--upgradable"}, ps...)...)
	}

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
	lines, err := queryDpkgUpgradeInfoByAptList()
	if err != nil {
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
	data := mapUpgradeInfo(
		lines,
		buildUpgradeInfoRegex(getSystemArchitectures()),
		buildUpgradeInfo)
	return writeData(fpath, data)
}
