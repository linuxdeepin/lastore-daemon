/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"bufio"
	"bytes"
	log "github.com/cihub/seelog"
	"internal/system"
	"io"
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

// distupgradeList return the pkgs from apt dist-upgrade
// NOTE: the result strim the arch suffix
func distupgradList() []string {
	cmd := exec.Command("apt-get", "dist-upgrade", "--assume-no", "-o", "Debug::NoLocking=1")
	bs, _ := cmd.Output()
	const upgraded = "The following packages will be upgraded:"
	const newInstalled = "The following NEW packages will be installed:"
	p := parseAptShowList(bytes.NewBuffer(bs), upgraded)
	p = append(p, parseAptShowList(bytes.NewBuffer(bs), newInstalled)...)
	return p
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

func queryDpkgUpgradeInfoByAptList() []string {
	ps := distupgradList()
	if len(ps) == 0 {
		return nil
	}
	cmd := exec.Command("apt", append([]string{"list", "--upgradable"}, ps...)...)

	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	err = cmd.Start()
	if err != nil {
		log.Errorf("LockDo: %v\n", err)
	}
	timer := time.AfterFunc(time.Second*10, func() {
		cmd.Process.Signal(syscall.SIGINT)
	})

	buf := bytes.NewBuffer(nil)

	buf.ReadFrom(r)

	var lines []string
	var line string
	for ; err == nil; line, err = buf.ReadString('\n') {
		lines = append(lines, strings.TrimSpace(line))
	}
	cmd.Wait()
	timer.Stop()
	return lines
}

func getSystemArchitectures() []system.Architecture {
	foreignArchs, err := exec.Command("dpkg", "--print-foreign-architectures").Output()
	if err != nil {
		log.Warnf("GetSystemArchitecture failed:%v\n", foreignArchs)
	}

	arch, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		log.Warnf("GetSystemArchitecture failed:%v\n", foreignArchs)
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
	data := mapUpgradeInfo(
		queryDpkgUpgradeInfoByAptList(),
		buildUpgradeInfoRegex(getSystemArchitectures()),
		buildUpgradeInfo)
	return writeData(fpath, data)
}
