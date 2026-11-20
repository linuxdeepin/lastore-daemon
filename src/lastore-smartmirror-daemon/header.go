// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// httpClient is the default http client
var httpClient = http.Client{
	Timeout: time.Second * 2,
}

// userAgent returns the value sent in the User-Agent header. It is cached
// because it is built for every probe and sniff request and lsb_release is a
// short-lived child process.
var (
	userAgentOnce  sync.Once
	userAgentValue string
)

// userAgent fill local info to string
func userAgent() string {
	userAgentOnce.Do(func() {
		const DetectVersion = "detector/0.1.1 " + runtime.GOARCH
		r, _ := exec.Command("lsb_release", "-ds").Output()
		if len(r) == 0 {
			userAgentValue = DetectVersion + " " + "deepin unknown"
			return
		}
		userAgentValue = DetectVersion + " " + strings.TrimSpace(string(r))
	})
	return userAgentValue
}

// machineID returns the content of /etc/machine-id. It is cached for the same
// reason as userAgent: the value does not change while the daemon runs.
var (
	machineIDOnce  sync.Once
	machineIDValue string
)

func machineID() string {
	machineIDOnce.Do(func() {
		bs, _ := os.ReadFile("/etc/machine-id")
		machineIDValue = strings.TrimSpace(string(bs))
	})
	return machineIDValue
}

func makeHeader() map[string]string {
	return map[string]string{
		"User-Agent": userAgent(),
		"MID":        machineID(),
		"Range":      "bytes=0-1023",
	}
}

// buildRequest with header
func buildRequest(header map[string]string, method string, url string) *http.Request {
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	return r
}

// handleRequest wait request reply and close connection quickly
func handleRequest(r *http.Request) (resultURL string, statusCode int) {
	if r == nil {
		return "", -1
	}
	defer func() {
		logger.Debugf("handleRequest: r.Url=%s, return url=%s, statusCode=%v", r.URL, resultURL, statusCode)
	}()

	resp, err := httpClient.Do(r)
	if err != nil {
		return "", -2
	}
	// Read and discard a small amount of the body to allow connection reuse.
	// Limit to 4KB to avoid downloading large files when the server ignores Range.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return "", resp.StatusCode
	case 3:
		u, err := resp.Location()
		if err != nil {
			return r.URL.String(), resp.StatusCode
		}
		return u.String(), resp.StatusCode
	case 2, 1:
		return r.URL.String(), resp.StatusCode
	default:
		return "", -3
	}
}
