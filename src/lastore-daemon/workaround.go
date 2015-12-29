package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// see the apt code of command-line/apt-get.c:895
var __ReDownloadSize__ = regexp.MustCompile("Need to get ([0-9,.]+) ([kMGTPEZY]?)B(/[0-9,.]+ [kMGTPEZY]?B)? of archives")

var __unitTable__ = map[byte]float64{
	'k': 1000,
	'M': 1000 * 1000,
	'G': 1000 * 1000 * 1000,
	'T': 1000 * 1000 * 1000 * 1000,
	'P': 1000 * 1000 * 1000 * 1000 * 1000,
	'E': 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	'Z': 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	'Y': 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
}

const SizeDownloaded = 0
const SizeUnknown = -1

func parsePackageSize(line string) float64 {
	ms := __ReDownloadSize__.FindSubmatch(([]byte)(line))
	switch len(ms) {
	case 3, 4:
		l := strings.Replace(string(ms[1]), ",", "", -1)
		size, err := strconv.ParseFloat(l, 64)
		if err != nil {
			return SizeUnknown
		}
		if len(ms[2]) == 0 {
			return size
		}
		unit := ms[2][0]
		return size * __unitTable__[unit]
	}
	return SizeUnknown
}

// QueryPackageDownloadSize parsing the total size of download archives when installing
// the packages.
func QueryPackageDownloadSize(packages ...string) float64 {
	cmd := exec.Command("/usr/bin/apt-get", append([]string{"-d", "-o", "Debug::NoLocking=1", "--print-uris", "--assume-no", "install"}, packages...)...)

	lines, err := filterExecOutput(cmd, time.Second*3, func(line string) bool {
		return parsePackageSize(line) != SizeUnknown
	})

	if len(lines) != 0 {
		return parsePackageSize(lines[0])
	}

	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			if status.ExitStatus() == 1 {
				// --assume-no will cause apt-get exit with code 1 when successfully
				return SizeDownloaded
			}
		}
	}
	return SizeUnknown
}

func guestBasePackageName(pkgId string) string {
	for _, sep := range []byte{'-', ':', '_'} {
		index := strings.LastIndexByte(pkgId, sep)
		if index != -1 {
			return pkgId[:index]
		}
	}
	return pkgId
}

// QueryPackageSameNameDepends guest packages may be contain the desktop files.
// e.g.
//    stardict-gtk --> stardict-common
//    stardict-gnome --> stardict-common
//    evince --> evince-common
//    evince-gtk --> evince, evince-common  Note: (recursion guest)
func QueryPackageSameNameDepends(pkgId string) []string {
	out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Depends}", pkgId).CombinedOutput()
	if err != nil {
		return nil
	}

	baseName := guestBasePackageName(pkgId)

	var r = []string{pkgId}
	for _, name := range strings.Fields(string(out)) {
		if strings.Contains(name, baseName) {
			// TODO: Batch query the exist. And Merge this code into apt.go
			out, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Status}", name).CombinedOutput()
			if err != nil {
				continue
			}
			if !strings.Contains(string(out), "install ok installed") {
				continue
			}

			r = append(r, name)
			r = append(r, QueryPackageSameNameDepends(name)...)
		}
	}
	return r
}

func QueryDesktopPath(packages ...string) string {
	cmd := exec.Command("dpkg", append([]string{"-L"}, packages...)...)

	desktopFiles, err := filterExecOutput(
		cmd,
		time.Second*2,
		func(line string) bool {
			return strings.HasSuffix(line, ".desktop")
		},
	)
	if err != nil || len(desktopFiles) == 0 {
		return ""
	}
	return DesktopFiles(desktopFiles).BestOne()
}

func filterExecOutput(cmd *exec.Cmd, timeout time.Duration, filter func(line string) bool) ([]string, error) {
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	timer := time.AfterFunc(timeout, func() {
		cmd.Process.Kill()
	})
	cmd.Start()
	buf := bytes.NewBuffer(nil)

	buf.ReadFrom(r)

	var lines []string
	var line string
	for ; err == nil; line, err = buf.ReadString('\n') {
		line = strings.TrimSpace(line)
		if filter(line) {
			lines = append(lines, line)
		}
	}

	err = cmd.Wait()
	timer.Stop()
	return lines, err
}

func Touch(arch, region string, packages ...string) {
	for _, pkg := range packages {
		url := fmt.Sprintf("http://download.lastore.deepin.org/get/%s/%s?&f=%s",
			arch,
			pkg,
			region,
		)
		resp, err := http.Get(url)
		if err != nil {
			return
		}
		resp.Body.Close()
	}
}
