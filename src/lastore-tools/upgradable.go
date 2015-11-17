package main

import (
	"bytes"
	log "github.com/cihub/seelog"
	"internal/system"
	"io/ioutil"
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
	s := `^(.*)\/unknown\s+(.*)\s+([` + archAlphabet + `]+)\s+\[upgradable from:\s+(.*)\s?\]$`
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

func queryDpkgUpgradeInfoByAptList() []string {
	cmd := exec.Command("apt", "list", "--upgradable")

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
	bs, err := ioutil.ReadFile("/var/lib/dpkg/arch")
	if err != nil {
		log.Error("Can't detect system architectures:", err)
		return nil
	}
	var r []system.Architecture
	for _, arch := range strings.Split(string(bs), "\n") {
		i := strings.TrimSpace(arch)
		if i == "" {
			continue
		}
		r = append(r, system.Architecture(i))
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
