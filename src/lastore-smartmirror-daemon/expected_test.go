// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"
)

const expectedFixture = `Package: pkg
Version: 1.0
Architecture: amd64
Filename: pool/main/p/pkg/pkg_1.0_amd64.deb
Size: 1234
SHA256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
Description: fixture
 continuation line is not a field

Package: pkg
Status: install ok installed
Version: 0.9
`

func newFixtureResolver(show func(string) ([]byte, error)) *expectedResolver {
	return &expectedResolver{
		cache:    make(map[string]expectedCacheEntry),
		now:      time.Now,
		show:     show,
		max:      expectedCacheMaxItems,
		listsMod: func() time.Time { return time.Time{} },
	}
}

func TestExpectedResolverParsesMatchingRecord(t *testing.T) {
	var asked string
	r := newFixtureResolver(func(pkg string) ([]byte, error) {
		asked = pkg
		return []byte(expectedFixture), nil
	})
	size, ok := r.resolveSize("https://official.example/deepin/beige/pool/main/p/pkg/pkg_1.0_amd64.deb")
	if !ok {
		t.Fatal("resolveSize returned false")
	}
	if asked != "pkg" {
		t.Fatalf("apt-cache package = %q, want pkg", asked)
	}
	if size != 1234 {
		t.Fatalf("size = %d", size)
	}
}

func TestExpectedResolverIgnoresInstalledRecord(t *testing.T) {
	onlyStatus := "Package: pkg\nStatus: install ok installed\nVersion: 0.9\nSize: 99\n"
	r := newFixtureResolver(func(string) ([]byte, error) { return []byte(onlyStatus), nil })
	if _, ok := r.resolveSize("https://official.example/deepin/pool/main/p/pkg/pkg_0.9_amd64.deb"); ok {
		t.Fatal("installed-only record must not describe a download")
	}
}

func TestExpectedResolverRequiresSize(t *testing.T) {
	noSize := "Package: pkg\nFilename: pool/main/p/pkg/pkg_1.0_amd64.deb\nSHA256: aa\n"
	r := newFixtureResolver(func(string) ([]byte, error) { return []byte(noSize), nil })
	if _, ok := r.resolveSize("https://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb"); ok {
		t.Fatal("a record without Size must be inconclusive")
	}
}

func TestExpectedResolverRejectsOtherFilename(t *testing.T) {
	other := "Package: pkg\nFilename: pool/main/q/other/other_1.0_amd64.deb\nSize: 10\n"
	r := newFixtureResolver(func(string) ([]byte, error) { return []byte(other), nil })
	if _, ok := r.resolveSize("https://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb"); ok {
		t.Fatal("a record for another file must not match")
	}
}

func TestExpectedResolverRejectsInvalidURL(t *testing.T) {
	calls := 0
	r := newFixtureResolver(func(string) ([]byte, error) {
		calls++
		return []byte(expectedFixture), nil
	})
	for _, raw := range []string{
		"not a url",
		"ftp://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb",
		"https://user@official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb",
		"https://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.tar.gz",
		"https://official.example/deepin/pool/main/p/pkg/noextension",
		"https://official.example/deepin/pool/main/p/pkg/Bad_1.0_amd64.deb",
	} {
		if _, ok := r.resolveSize(raw); ok {
			t.Fatalf("resolveSize(%q) accepted", raw)
		}
	}
	if calls != 0 {
		t.Fatalf("apt-cache ran %d times for invalid URLs", calls)
	}
}

func TestExpectedResolverCachesByPath(t *testing.T) {
	calls := 0
	r := newFixtureResolver(func(string) ([]byte, error) {
		calls++
		return []byte(expectedFixture), nil
	})
	uri := "https://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb"
	if _, ok := r.resolveSize(uri); !ok {
		t.Fatal("first resolve failed")
	}
	if _, ok := r.resolveSize(uri); !ok {
		t.Fatal("second resolve failed")
	}
	if calls != 1 {
		t.Fatalf("apt-cache calls = %d, want 1", calls)
	}

	now := time.Now().Add(expectedCacheTTL + time.Second)
	r.now = func() time.Time { return now }
	if _, ok := r.resolveSize(uri); !ok {
		t.Fatal("resolve after expiry failed")
	}
	if calls != 2 {
		t.Fatalf("apt-cache calls after expiry = %d, want 2", calls)
	}
}

func TestExpectedResolverInvalidatesWhenIndexChanges(t *testing.T) {
	calls := 0
	listsAt := time.Unix(100, 0)
	r := newFixtureResolver(func(string) ([]byte, error) {
		calls++
		return []byte(expectedFixture), nil
	})
	r.listsMod = func() time.Time { return listsAt }

	uri := "https://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb"
	if _, ok := r.resolveSize(uri); !ok {
		t.Fatal("first resolve failed")
	}
	if _, ok := r.resolveSize(uri); !ok {
		t.Fatal("second resolve failed")
	}
	if calls != 1 {
		t.Fatalf("apt-cache calls = %d, want cache hit", calls)
	}

	// apt update rewrote /var/lib/apt/lists; the old rows must not be used.
	listsAt = listsAt.Add(time.Minute)
	if _, ok := r.resolveSize(uri); !ok {
		t.Fatal("resolve after index change failed")
	}
	if calls != 2 {
		t.Fatalf("apt-cache calls after index change = %d, want 2", calls)
	}
}

func TestExpectedResolverDisagreeingRecordsAreInconclusive(t *testing.T) {
	disagree := "Package: pkg\nFilename: pool/main/p/pkg/pkg_1.0_amd64.deb\nSize: 10\n\n" +
		"Package: pkg\nFilename: pool/main/p/pkg/pkg_1.0_amd64.deb\nSize: 11\n"
	r := newFixtureResolver(func(string) ([]byte, error) { return []byte(disagree), nil })
	if _, ok := r.resolveSize("https://official.example/deepin/pool/main/p/pkg/pkg_1.0_amd64.deb"); ok {
		t.Fatal("disagreeing records must not be picked arbitrarily")
	}
}

func TestExpectedResolverAcceptsEscapedPoolFilename(t *testing.T) {
	epoch := "Package: pkg\nFilename: pool/main/p/pkg/pkg_1%3a2.0_amd64.deb\nSize: 10\n"
	r := newFixtureResolver(func(string) ([]byte, error) { return []byte(epoch), nil })
	if _, ok := r.resolveSize("https://official.example/deepin/pool/main/p/pkg/pkg_1%3a2.0_amd64.deb"); !ok {
		t.Fatal("escaped epoch filename rejected")
	}
}

func TestParsePoolURL(t *testing.T) {
	tests := []struct {
		raw     string
		pkg     string
		wantErr bool
	}{
		{"https://official.example/deepin/pool/main/b/bash/bash_5.2_amd64.deb", "bash", false},
		{"https://official.example/deepin/pool/main/g/g++/g++_4.0_amd64.deb", "g++", false},
		{"https://official.example/deepin/pool/main/libx/libx-1/libx-1_1.0_amd64.deb", "libx-1", false},
		{"https://official.example/deepin/pool/main/b/bash/bash_5.2_amd64.deb?token=x", "bash", false},
		{"https://official.example/deepin/pool/main/b/bash/", "", true},
		{"https://official.example/deepin/pool/main/b/bash/bash.deb", "", true},
		{"https://official.example/deepin/pool/main/b/bash/bash_5.2_amd64.ddeb", "", true},
	}
	for _, test := range tests {
		_, pkg, err := parsePoolURL(test.raw)
		if test.wantErr {
			if err == nil {
				t.Fatalf("parsePoolURL(%q) accepted", test.raw)
			}
			continue
		}
		if err != nil || pkg != test.pkg {
			t.Fatalf("parsePoolURL(%q) = %q, %v", test.raw, pkg, err)
		}
	}
}
