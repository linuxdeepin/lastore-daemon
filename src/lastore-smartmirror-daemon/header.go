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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
	bs, _ := ioutil.ReadFile("/etc/machine-id")
	return strings.TrimSpace(string(bs))
}

func stripURLPath(u string) string {
	v, err := url.Parse(u)
	if err != nil {
		return u
	}
	return v.Hostname()
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
