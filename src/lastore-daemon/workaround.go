package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GuestPackageDownloadSize parsing the total size of download archives when installing
// the pid package.
func GuestPackageDownloadSize(pid string) int {
	var needle = regexp.MustCompile("Need to get ([0-9,]+) kB of archives")
	var size = -1

	cmd := exec.Command("/usr/bin/apt-get", "install", pid, "-o", "Debug::NoLocking=1", "--assume-no")
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
		ms := needle.FindSubmatch(line)
		if len(ms) == 2 {
			l := strings.Replace(string(ms[1]), ",", "", -1)
			size, err = strconv.Atoi(l)
			if err == nil {
				break
			}
		}
	}

	cmd.Wait()
	timer.Stop()

	return size * 1024
}
