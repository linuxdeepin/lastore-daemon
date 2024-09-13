// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// httpClient is the default http client
var httpClient = http.Client{
	Timeout: time.Second * 2,
}

// userAgent fill local info to string
func userAgent() string {
	const DetectVersion = "detector/0.1.1 " + runtime.GOARCH

	r, _ := exec.Command("lsb_release", "-ds").Output()
	if len(r) == 0 {
		return DetectVersion + " " + "deepin unknown"
	}
	return DetectVersion + " " + strings.TrimSpace(string(r))
}

// machineID return content of /etc/machine-id
func machineID() string {
	bs, _ := os.ReadFile("/etc/machine-id")
	return strings.TrimSpace(string(bs))
}

func stripURLPath(u string) string {
	v, err := url.Parse(u)
	if err != nil {
		return u
	}
	return getUrlHostname(v)
}

func getUrlHostname(u *url.URL) string {
	return stripPort(u.Host)
}

// copy from go source net/url/url.go
func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}

func makeHeader() map[string]string {
	return map[string]string{
		"User-Agent": userAgent(),
		"MID":        machineID(),
	}
}

func makeReportHeader(reports []Report) map[string]string {
	m1 := []string{}
	for _, r := range reports {
		status := fmt.Sprintf("T%v", r.Delay)
		if r.Failed {
			status = fmt.Sprintf("E%v", r.StatusCode)
		}
		m1 = append(m1, stripURLPath(r.Mirror)+":"+status)
	}
	return map[string]string{
		"User-Agent": userAgent(),
		"MID":        machineID(),
		"M1":         strings.Join(m1, ";"),
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
func handleRequest(r *http.Request) (string, int) {
	if r == nil {
		return "", -1
	}
	resp, err := httpClient.Do(r)
	if err != nil {
		return "", -2
	}
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
