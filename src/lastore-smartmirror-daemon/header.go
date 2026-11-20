// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"net/http"
	"os"
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
// because it is built for every probe and sniff request and the underlying
// value does not change while the daemon runs.
var (
	userAgentOnce  sync.Once
	userAgentValue string
)

// userAgent fill local info to string
func userAgent() string {
	userAgentOnce.Do(func() {
		const DetectVersion = "detector/0.1.1 " + runtime.GOARCH
		desc := osReleaseDescription()
		if desc == "" {
			desc = "deepin unknown"
		}
		userAgentValue = DetectVersion + " " + desc
	})
	return userAgentValue
}

// osReleasePaths lists the os-release(5) identification files, most specific
// first. /etc/os-release overrides the vendor copy in /usr/lib/os-release, the
// same precedence lsb_release applies.
var osReleasePaths = []string{"/etc/os-release", "/usr/lib/os-release"}

// osReleaseDescription returns the distribution description, or "" when no
// file yields one. Reading the file directly instead of forking lsb_release
// removes the subprocess and the "No LSB modules are available." diagnostics
// some builds print on stdout; a newline smuggled into the User-Agent header
// used to make every probe and sniff request fail with "invalid header field
// value".
func osReleaseDescription() string {
	for _, path := range osReleasePaths {
		bs, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if desc := parseOsRelease(string(bs)); desc != "" {
			return desc
		}
	}
	return ""
}

// parseOsRelease extracts the description from os-release(5) content: the
// PRETTY_NAME value, falling back to NAME plus VERSION_ID/VERSION like the
// reference lsb_release does. Control characters are dropped so a malformed
// name cannot break the HTTP header.
func parseOsRelease(content string) string {
	var name, version, versionID string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		switch key {
		case "PRETTY_NAME":
			return headerSafe(osReleaseUnquote(value))
		case "NAME":
			name = osReleaseUnquote(value)
		case "VERSION_ID":
			versionID = osReleaseUnquote(value)
		case "VERSION":
			version = osReleaseUnquote(value)
		}
	}
	if name == "" {
		return ""
	}
	v := versionID
	if v == "" {
		v = version
	}
	if v == "" {
		return headerSafe(name)
	}
	return headerSafe(name + " " + v)
}

// osReleaseUnquote decodes an os-release(5) value: bare, single-quoted
// (no escapes) or double-quoted (shell-style \" \\ \$ \` \n \t escapes).
func osReleaseUnquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		var b strings.Builder
		body := v[1 : len(v)-1]
		for i := 0; i < len(body); i++ {
			c := body[i]
			if c == '\\' && i+1 < len(body) {
				i++
				switch body[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case '"', '\\', '$', '`':
					b.WriteByte(body[i])
				default:
					b.WriteByte('\\')
					b.WriteByte(body[i])
				}
				continue
			}
			b.WriteByte(c)
		}
		return b.String()
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return v[1 : len(v)-1]
	}
	return v
}

// headerSafe drops control characters so a value stays a valid header token.
func headerSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, s)
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
