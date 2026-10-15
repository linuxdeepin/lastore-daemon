// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
)

func TestPickStableProbeURIPrefersMainNativeArch(t *testing.T) {
	const official = "https://cdn.example.com/deepin/"
	output := `
https://cdn.example.com/deepin/beige/dists/crimson/main/source/Sources
https://cdn.example.com/deepin/beige/dists/crimson/main/binary-i386/Packages
https://cdn.example.com/deepin/beige/dists/crimson/commercial/binary-amd64/Packages
https://cdn.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages
https://other.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages
`
	got := pickStableProbeURI(output, official, "amd64")
	want := "https://cdn.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages"
	if got != want {
		t.Fatalf("pickStableProbeURI = %q, want %q", got, want)
	}
}

func TestPickStableProbeURIFallsBackToNonMain(t *testing.T) {
	const official = "https://cdn.example.com/deepin/"
	output := `
https://cdn.example.com/deepin/beige/dists/crimson/main/binary-i386/Packages
https://cdn.example.com/deepin/beige/dists/crimson/community/binary-amd64/Packages
`
	got := pickStableProbeURI(output, official, "amd64")
	want := "https://cdn.example.com/deepin/beige/dists/crimson/community/binary-amd64/Packages"
	if got != want {
		t.Fatalf("pickStableProbeURI = %q, want %q", got, want)
	}
}

func TestPickStableProbeURIAcceptsCompressedSuffix(t *testing.T) {
	const official = "https://cdn.example.com/deepin/"
	output := "https://cdn.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages.xz\n"
	got := pickStableProbeURI(output, official, "amd64")
	want := "https://cdn.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages.xz"
	if got != want {
		t.Fatalf("pickStableProbeURI = %q, want %q", got, want)
	}
}

func TestPickStableProbeURIFallsBackToOfficialWithoutNativeArch(t *testing.T) {
	const official = "https://cdn.example.com/deepin/"
	output := `
https://cdn.example.com/deepin/beige/dists/crimson/main/binary-arm64/Packages
https://other.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages
`
	got := pickStableProbeURI(output, official, "amd64")
	want := "https://cdn.example.com/deepin/beige/dists/crimson/main/binary-arm64/Packages"
	if got != want {
		t.Fatalf("pickStableProbeURI = %q, want %q", got, want)
	}
}

// A target from another archive cannot be rewritten onto the mirrors, so it must
// not be used as the probe base at all.
func TestPickStableProbeURIRejectsForeignArchive(t *testing.T) {
	const official = "https://cdn-community-packages.deepin.com/deepin/"
	output := `
https://community-packages.deepin.com/beige/dists/crimson/main/binary-amd64/Packages
https://community-packages.deepin.com/beige/dists/crimson/main/binary-amd64/Packages.gz
https://other.example.com/deepin/beige/dists/crimson/main/binary-amd64/Packages.xz
`
	if got := pickStableProbeURI(output, official, "amd64"); got != "" {
		t.Fatalf("pickStableProbeURI = %q, want \"\" for a foreign archive", got)
	}
}

func withDiscoverStableProbeURIFn(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	old := discoverStableProbeURIFn
	discoverStableProbeURIFn = fn
	t.Cleanup(func() { discoverStableProbeURIFn = old })
}

const (
	testOfficial = "https://cdn-community-packages.deepin.com/deepin/"
	testPackage  = "https://cdn-community-packages.deepin.com/deepin/beige/pool/main/d/d-feet/d-feet_0.3.16-3_all.deb"
	testMirror   = "https://mirrors.nju.edu.cn/deepin/"
)

func TestProbeBaseUsesStableIndexInsideOfficialPrefix(t *testing.T) {
	const stable = "https://cdn-community-packages.deepin.com/deepin/beige/dists/crimson/main/binary-amd64/Packages"
	var gotOfficial string
	withDiscoverStableProbeURIFn(t, func(official string) (string, error) {
		gotOfficial = official
		return stable, nil
	})

	s := &SmartMirror{}
	if got := s.probeBase(testPackage, testOfficial); got != stable {
		t.Fatalf("probeBase = %q, want the stable index %q", got, stable)
	}
	if gotOfficial != testOfficial {
		t.Fatalf("discovery official = %q, want %q", gotOfficial, testOfficial)
	}
}

func TestProbeBaseFallsBackToPackageURIForForeignStableIndex(t *testing.T) {
	// This is the shape of an environment whose sources.list host differs from
	// Acquire::SmartMirrors::MainSource.
	calls := 0
	withDiscoverStableProbeURIFn(t, func(string) (string, error) {
		calls++
		return "https://community-packages.deepin.com/beige/dists/crimson/main/binary-amd64/Packages", nil
	})

	s := &SmartMirror{}
	for i := 0; i < 2; i++ {
		if got := s.probeBase(testPackage, testOfficial); got != testPackage {
			t.Fatalf("probeBase = %q, want the package URI %q", got, testPackage)
		}
	}
	if calls != 1 {
		t.Fatalf("discovery calls = %d, want 1 (a rejected base must be cached too)", calls)
	}
	// The fallback base must still be rewritable, otherwise 2 MiB probes would
	// again be recorded as mirror failures.
	if target, ok := probeURI(testPackage, testOfficial, testMirror); !ok {
		t.Fatal("package URI must remain rewritable for a mirror")
	} else if want := "https://mirrors.nju.edu.cn/deepin/beige/pool/main/d/d-feet/d-feet_0.3.16-3_all.deb"; target != want {
		t.Fatalf("rewritten fallback = %q, want %q", target, want)
	}
}

func TestProbeBaseFallsBackWhenDiscoveryFails(t *testing.T) {
	withDiscoverStableProbeURIFn(t, func(string) (string, error) {
		return "", errors.New("apt-get indextargets unavailable")
	})

	s := &SmartMirror{}
	if got := s.probeBase(testPackage, testOfficial); got != testPackage {
		t.Fatalf("probeBase = %q, want the package URI %q", got, testPackage)
	}
}

func TestProbeBaseDiscoversAgainForAnotherOfficialPrefix(t *testing.T) {
	const stable = "https://cdn-community-packages.deepin.com/deepin/beige/dists/crimson/main/binary-amd64/Packages"
	otherOfficial := "https://community-packages.deepin.com/"
	otherPackage := "https://community-packages.deepin.com/beige/pool/main/d/d-feet/d-feet_0.3.16-3_all.deb"
	calls := 0
	withDiscoverStableProbeURIFn(t, func(official string) (string, error) {
		calls++
		if official == otherOfficial {
			return otherPackage, nil
		}
		return stable, nil
	})

	s := &SmartMirror{}
	if got := s.probeBase(testPackage, testOfficial); got != stable {
		t.Fatalf("probeBase = %q, want %q", got, stable)
	}
	if got := s.probeBase(otherPackage, otherOfficial); got != otherPackage {
		t.Fatalf("probeBase = %q, want %q", got, otherPackage)
	}
	if calls != 2 {
		t.Fatalf("discovery calls = %d, want 2", calls)
	}
}
