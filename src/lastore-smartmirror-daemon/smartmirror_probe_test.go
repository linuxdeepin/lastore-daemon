// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

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

func TestPickStableProbeURIFallsBackToBestRemainingTarget(t *testing.T) {
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
