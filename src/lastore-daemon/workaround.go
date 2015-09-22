package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var __needle__ = regexp.MustCompile("Need to get ([0-9,]+) ([kMGTPEZY]?)B(/[0-9,]+ [kMGTPEZY]?B)? of archives")
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

func parsePackageSize(line []byte) float64 {
	ms := __needle__.FindSubmatch(line)
	switch len(ms) {
	case 3, 4:
		l := strings.Replace(string(ms[1]), ",", "", -1)
		size, err := strconv.ParseFloat(l, 64)
		if err != nil {
			return -1
		}
		if len(ms[2]) == 0 {
			return size
		}
		unit := ms[2][0]
		return size * __unitTable__[unit]
	}
	return -1
}

// GuestPackageDownloadSize parsing the total size of download archives when installing
// the pid package.
func GuestPackageDownloadSize(pid string) float64 {
	var size float64 = -1

	cmd := exec.Command("/usr/bin/apt-get", "install", pid, "-o", "Debug::NoLocking=1", "--assume-no")
	// clean environments like LC_ALL, LANGUAGE and LANG
	cmd.Env = make([]string, 0)

	r, err := cmd.StdoutPipe()
	if err != nil {
		return -1
	}
	timer := time.AfterFunc(3*time.Second, func() {
		cmd.Process.Kill()
	})
	cmd.Start()
	buf := bytes.NewBuffer(nil)

	buf.ReadFrom(r)

	var line []byte
	for ; err == nil; line, err = buf.ReadBytes('\n') {
		size = parsePackageSize(line)
		if size != -1 {
			break
		}
	}

	cmd.Wait()
	timer.Stop()

	return size
}
