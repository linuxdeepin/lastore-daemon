// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestParseOsRelease(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "pretty name",
			content: "NAME=\"Deepin\"\nVERSION_ID=\"23\"\nPRETTY_NAME=\"Deepin 23\"\n",
			want:    "Deepin 23",
		},
		{
			name:    "pretty name wins when it precedes name and version",
			content: "PRETTY_NAME=\"Deepin 23\"\nNAME=\"Deepin\"\nVERSION_ID=\"23\"\n",
			want:    "Deepin 23",
		},
		{
			name:    "name and version id",
			content: "NAME=\"Deepin\"\nVERSION_ID=\"23\"\n",
			want:    "Deepin 23",
		},
		{
			name:    "version id preferred over version",
			content: "NAME=\"Deepin\"\nVERSION=\"23 (Lotus)\"\nVERSION_ID=\"23\"\n",
			want:    "Deepin 23",
		},
		{
			name:    "version fallback",
			content: "NAME=\"Deepin\"\nVERSION=\"23 (Lotus)\"\n",
			want:    "Deepin 23 (Lotus)",
		},
		{
			name:    "name only",
			content: "NAME=Deepin\n",
			want:    "Deepin",
		},
		{
			name:    "single quoted values",
			content: "NAME='Deepin'\nVERSION_ID='23'\n",
			want:    "Deepin 23",
		},
		{
			name:    "comments blank and malformed lines are ignored",
			content: "# comment\n\nVERSION_ID=23\nnot a key value line\nNAME = Deepin \n",
			want:    "Deepin 23",
		},
		{
			name:    "carriage returns are ignored",
			content: "NAME=\"Deepin\"\r\nVERSION_ID=\"23\"\r\n",
			want:    "Deepin 23",
		},
		{
			name:    "value may contain equals sign",
			content: "PRETTY_NAME=\"Deepin 23 = Lotus\"\n",
			want:    "Deepin 23 = Lotus",
		},
		{
			name:    "control characters are stripped",
			content: "PRETTY_NAME=\"Deepin\\n23\"\n",
			want:    "Deepin23",
		},
		{
			name:    "no usable keys",
			content: "VERSION_ID=\"23\"\n",
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			// A blank PRETTY_NAME currently yields no description instead of
			// falling back to NAME. The test pins that behaviour so a change
			// has to be deliberate.
			name:    "empty pretty name",
			content: "PRETTY_NAME=\"\"\nNAME=\"Deepin\"\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOsRelease(tt.content); got != tt.want {
				t.Fatalf("parseOsRelease(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestOsReleaseUnquote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare", `Deepin 23`, "Deepin 23"},
		{"bare with padding", "  Deepin 23  ", "Deepin 23"},
		{"bare with embedded quotes", `Deepin "23"`, `Deepin "23"`},
		{"double quoted", `"Deepin 23"`, "Deepin 23"},
		{"empty double quoted", `""`, ""},
		{"empty single quoted", `''`, ""},
		{"single quoted does not decode escapes", `'a\nb'`, `a\nb`},
		{"escaped newline and tab", `"a\nb\tc"`, "a\nb\tc"},
		{"escaped double quote", `"a\"b"`, `a"b`},
		{"escaped backslash", `"a\\b"`, `a\b`},
		{"escaped dollar", `"a\$b"`, "a$b"},
		{"escaped backtick", "\"a\\`b\"", "a`b"},
		{"unknown escape kept verbatim", `"a\qb"`, `a\qb`},
		{"trailing backslash kept", `"ab\"`, `ab\`},
		{"unterminated quote returned as is", `"Deepin`, `"Deepin`},
		{"mismatched quotes returned as is", `"Deepin'`, `"Deepin'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := osReleaseUnquote(tt.in); got != tt.want {
				t.Fatalf("osReleaseUnquote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHeaderSafe(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Deepin 23", "Deepin 23"},
		{"empty", "", ""},
		{"newline", "Deepin\n23", "Deepin23"},
		{"tab and carriage return", "a\tb\rc", "abc"},
		{"nul and delete", "a\x00b\x7fc", "abc"},
		{"unicode kept", "深度 23", "深度 23"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headerSafe(tt.in); got != tt.want {
				t.Fatalf("headerSafe(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func writeOsReleaseFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func stubOsReleasePaths(t *testing.T, paths ...string) {
	t.Helper()
	old := osReleasePaths
	osReleasePaths = paths
	t.Cleanup(func() { osReleasePaths = old })
}

func TestOsReleaseDescription(t *testing.T) {
	t.Run("first file wins", func(t *testing.T) {
		dir := t.TempDir()
		first := writeOsReleaseFile(t, filepath.Join(dir, "etc-os-release"), "PRETTY_NAME=\"First\"\n")
		second := writeOsReleaseFile(t, filepath.Join(dir, "usr-os-release"), "PRETTY_NAME=\"Second\"\n")
		stubOsReleasePaths(t, first, second)

		if got := osReleaseDescription(); got != "First" {
			t.Fatalf("osReleaseDescription() = %q, want %q", got, "First")
		}
	})

	t.Run("missing file falls back to the next path", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing-os-release")
		second := writeOsReleaseFile(t, filepath.Join(dir, "usr-os-release"), "PRETTY_NAME=\"Second\"\n")
		stubOsReleasePaths(t, missing, second)

		if got := osReleaseDescription(); got != "Second" {
			t.Fatalf("osReleaseDescription() = %q, want %q", got, "Second")
		}
	})

	t.Run("unusable file falls back to the next path", func(t *testing.T) {
		dir := t.TempDir()
		first := writeOsReleaseFile(t, filepath.Join(dir, "etc-os-release"), "# only a comment\n")
		second := writeOsReleaseFile(t, filepath.Join(dir, "usr-os-release"), "NAME=Deepin\nVERSION_ID=23\n")
		stubOsReleasePaths(t, first, second)

		if got := osReleaseDescription(); got != "Deepin 23" {
			t.Fatalf("osReleaseDescription() = %q, want %q", got, "Deepin 23")
		}
	})

	t.Run("no usable file", func(t *testing.T) {
		dir := t.TempDir()
		stubOsReleasePaths(t,
			filepath.Join(dir, "missing-etc-os-release"),
			filepath.Join(dir, "missing-usr-os-release"))

		if got := osReleaseDescription(); got != "" {
			t.Fatalf("osReleaseDescription() = %q, want empty", got)
		}
	})
}

func resetUserAgentCache(t *testing.T) {
	t.Helper()
	userAgentOnce = sync.Once{}
	userAgentValue = ""
	t.Cleanup(func() {
		// Leave the cache empty so later tests rebuild it from the real files.
		userAgentOnce = sync.Once{}
		userAgentValue = ""
	})
}

func TestUserAgentCachesDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeOsReleaseFile(t, filepath.Join(dir, "os-release"), "PRETTY_NAME=\"Cached 23\"\n")
	stubOsReleasePaths(t, path)
	resetUserAgentCache(t)

	want := "detector/0.1.1 " + runtime.GOARCH + " Cached 23"
	if got := userAgent(); got != want {
		t.Fatalf("userAgent() = %q, want %q", got, want)
	}

	// The value was cached on the first call, so a later file change must not
	// leak into the User-Agent.
	writeOsReleaseFile(t, path, "PRETTY_NAME=\"Changed 23\"\n")
	if got := userAgent(); got != want {
		t.Fatalf("userAgent() after source change = %q, want cached %q", got, want)
	}
}

func TestUserAgentFallsBackWithoutOsRelease(t *testing.T) {
	dir := t.TempDir()
	stubOsReleasePaths(t, filepath.Join(dir, "missing-os-release"))
	resetUserAgentCache(t)

	got := userAgent()
	want := "detector/0.1.1 " + runtime.GOARCH + " deepin unknown"
	if got != want {
		t.Fatalf("userAgent() = %q, want %q", got, want)
	}
	if strings.ContainsAny(got, "\r\n\x00") {
		t.Fatalf("userAgent() = %q contains a control character", got)
	}
}

// machineID caches the value read on first use, so later calls must return the
// cached value without touching /etc/machine-id again.
func TestMachineIDReturnsCachedValue(t *testing.T) {
	const cached = "cached-machine-id"
	machineIDOnce = sync.Once{}
	machineIDValue = cached
	// Consume the once to simulate a completed first read.
	machineIDOnce.Do(func() {})
	t.Cleanup(func() {
		// Leave the cache empty so later tests read the real file again.
		machineIDOnce = sync.Once{}
		machineIDValue = ""
	})

	if got := machineID(); got != cached {
		t.Fatalf("machineID() = %q, want cached value %q", got, cached)
	}
}
