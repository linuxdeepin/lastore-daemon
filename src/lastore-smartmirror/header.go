package main

import (
	"io/ioutil"
	"os/exec"
	"strings"
)

const DetectVersion = "detector/0.1"

func UserAgent() string {
	r, _ := exec.Command("lsb_release", "-ds").Output()
	if len(r) == 0 {
		return DetectVersion + " " + "deepin unknown"
	}
	return DetectVersion + " " + strings.TrimSpace(string(r))
}

func MachineID() string {
	bs, _ := ioutil.ReadFile("/etc/machine-id")
	return strings.TrimSpace(string(bs))
}

func MakeHeader(mirrorURL string) map[string]string {
	return map[string]string{
		"User-Agent": UserAgent(),
		"MID":        MachineID(),
		"M":          mirrorURL,
	}
}
