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

func UserAgent() string {
	const DetectVersion = "detector/0.1.1 " + runtime.GOARCH

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

func stripURLPath(u string) string {
	v, err := url.Parse(u)
	if err != nil {
		return u
	}
	return v.Hostname()
}

func MakeHeader(mirrorURL string) map[string]string {
	return map[string]string{
		"User-Agent": UserAgent(),
		"MID":        MachineID(),
		"M":          stripURLPath(mirrorURL),
	}
}

func BuildRequest(header map[string]string, method string, url string) *http.Request {
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	return r
}

func HandleRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	resp, err := httpClient.Do(r)
	if err != nil {
		return ""
	}
	resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return ""
	case 3:
		u, err := resp.Location()
		if err != nil {
			return r.URL.String()
		}
		return u.String()
	case 2, 1:
		return r.URL.String()
	default:
		return ""
	}
}
