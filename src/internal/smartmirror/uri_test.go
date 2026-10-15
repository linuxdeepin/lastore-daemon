// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import "testing"

func TestRewritePreservesSuitePath(t *testing.T) {
	original := "https://cdn-community-packages.deepin.com/deepin/beige/pool/main/a/a.deb"
	official := "https://cdn-community-packages.deepin.com/deepin/"
	mirror := "https://mirrors.kernel.org/deepin/"

	got, ok := Rewrite(original, official, mirror)
	if !ok {
		t.Fatal("Rewrite rejected valid URL")
	}
	want := "https://mirrors.kernel.org/deepin/beige/pool/main/a/a.deb"
	if got != want {
		t.Fatalf("Rewrite() = %q, want %q", got, want)
	}
}

func TestRewriteRejectsMismatchedOfficialBase(t *testing.T) {
	if _, ok := Rewrite("https://other.example/deepin/beige/pool/a.deb",
		"https://cdn-community-packages.deepin.com/deepin/",
		"https://mirrors.kernel.org/deepin/"); ok {
		t.Fatal("Rewrite accepted URL outside official base")
	}
}

func TestSameBase(t *testing.T) {
	const official = "https://cdn.example.com/deepin/"
	cases := []struct {
		name     string
		original string
		want     bool
	}{
		{"inside official prefix", "https://cdn.example.com/deepin/beige/pool/main/a/a.deb", true},
		{"official base itself", "https://cdn.example.com/deepin/", true},
		// The official path must be a real path prefix: "/deepin" is not
		// "/deepin/", exactly as route() and Rewrite treat it.
		{"official path without trailing slash", "https://cdn.example.com/deepin", false},
		{"other host", "https://community.example.com/deepin/beige/pool/a.deb", false},
		{"other scheme", "http://cdn.example.com/deepin/beige/pool/a.deb", false},
		{"outside official path", "https://cdn.example.com/beige/pool/a.deb", false},
		{"credentials", "https://user:pw@cdn.example.com/deepin/a.deb", false},
		{"invalid URL", "://cdn.example.com/deepin/a.deb", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameBase(tc.original, official); got != tc.want {
				t.Fatalf("SameBase(%q, %q) = %v, want %v", tc.original, official, got, tc.want)
			}
		})
	}
}

// SameBase is the check Rewrite applies, so anything it accepts must be
// rewritable for a usable mirror entry.
func TestSameBaseMatchesRewriteAcceptance(t *testing.T) {
	const (
		official = "https://cdn.example.com/deepin/"
		mirror   = "https://mirrors.example/deepin/"
	)
	for _, original := range []string{
		"https://cdn.example.com/deepin/beige/pool/main/a/a.deb",
		"https://community.example.com/deepin/beige/pool/a.deb",
		"https://cdn.example.com/other/pool/a.deb",
	} {
		_, ok := Rewrite(original, official, mirror)
		if got := SameBase(original, official); got != ok {
			t.Fatalf("SameBase(%q) = %v, Rewrite accepted = %v", original, got, ok)
		}
	}
}
