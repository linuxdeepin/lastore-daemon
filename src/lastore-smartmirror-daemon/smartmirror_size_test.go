// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
	"time"
)

const sizeTestOriginal = "https://official.example/deepin/beige/pool/main/a/pkg/pkg_1.0_amd64.deb"
const sizeTestOfficial = "https://official.example/deepin/"

// newSizeTestMirror builds a mirror set that is fully gated and ranked, with
// the sniff seam reporting the declared size for each mirror.
func newSizeTestMirror(t *testing.T, mirrors []string, declared map[string]int64) (*SmartMirror, *int) {
	t.Helper()
	now := time.Now()
	quality := QualityMap{}
	for _, mirror := range mirrors {
		quality[mirror] = &Quality{
			Probe4KAt:     now,
			Probe4KPassed: true,
			Probe2MNextAt: now.Add(time.Hour),
		}
	}
	s := &SmartMirror{
		sourcesURL:    mirrors,
		mirrorQuality: MirrorQuality{QualityMap: quality, weights: map[string]int{}},
	}

	oldSniff, oldExpected := sniffResourceFn, expectedSizeFn
	t.Cleanup(func() {
		sniffResourceFn, expectedSizeFn = oldSniff, oldExpected
	})
	sniffResourceFn = func(_ *aptProbe, uri string) (sniffResult, int64) {
		for mirror, size := range declared {
			if strings.HasPrefix(uri, mirror) {
				return sniffOK, size
			}
		}
		return sniffNotFound, 0
	}
	expectedCalls := 0
	expectedSizeFn = func(*expectedResolver, string) (int64, bool) {
		expectedCalls++
		return 0, false
	}
	return s, &expectedCalls
}

func TestMakeChoiceRejectsWrongSize(t *testing.T) {
	stale := "https://stale.example/deepin/"
	good := "https://good.example/deepin/"
	s, _ := newSizeTestMirror(t, []string{stale, good}, map[string]int64{
		stale: 3 << 20,
		good:  4 << 20,
	})
	expectedSizeFn = func(*expectedResolver, string) (int64, bool) { return 4 << 20, true }

	got := s.makeChoice(sizeTestOriginal, sizeTestOfficial)
	want := good + "beige/pool/main/a/pkg/pkg_1.0_amd64.deb"
	if got != want {
		t.Fatalf("makeChoice = %q, want %q", got, want)
	}
	if !s.sniffBlocked(stale) {
		t.Fatal("wrong-size mirror was not blocked")
	}
	if s.sniffBlocked(good) {
		t.Fatal("matching mirror was blocked")
	}
}

func TestMakeChoiceWrongSizeFallsBackToOfficial(t *testing.T) {
	mirror := "https://stale.example/deepin/"
	s, _ := newSizeTestMirror(t, []string{mirror}, map[string]int64{mirror: 3 << 20})
	expectedSizeFn = func(*expectedResolver, string) (int64, bool) { return 4 << 20, true }

	if got := s.makeChoice(sizeTestOriginal, sizeTestOfficial); got != sizeTestOriginal {
		t.Fatalf("makeChoice = %q, want official fallback", got)
	}
	if !s.sniffBlocked(mirror) {
		t.Fatal("wrong-size mirror was not blocked")
	}
}

func TestMakeChoiceAcceptsMatchingSize(t *testing.T) {
	mirror := "https://good.example/deepin/"
	s, _ := newSizeTestMirror(t, []string{mirror}, map[string]int64{mirror: 4 << 20})
	expectedSizeFn = func(*expectedResolver, string) (int64, bool) { return 4 << 20, true }

	want := mirror + "beige/pool/main/a/pkg/pkg_1.0_amd64.deb"
	if got := s.makeChoice(sizeTestOriginal, sizeTestOfficial); got != want {
		t.Fatalf("makeChoice = %q, want %q", got, want)
	}
}

func TestMakeChoiceSkipsSizeCheckWithoutDeclaredSize(t *testing.T) {
	mirror := "https://good.example/deepin/"
	s, expectedCalls := newSizeTestMirror(t, []string{mirror}, map[string]int64{mirror: 0})

	want := mirror + "beige/pool/main/a/pkg/pkg_1.0_amd64.deb"
	if got := s.makeChoice(sizeTestOriginal, sizeTestOfficial); got != want {
		t.Fatalf("makeChoice = %q, want %q", got, want)
	}
	if *expectedCalls != 0 {
		t.Fatalf("metadata looked up %d times without a declared size", *expectedCalls)
	}
}

func TestMakeChoiceSkipsSizeCheckWithoutMetadata(t *testing.T) {
	mirror := "https://good.example/deepin/"
	s, expectedCalls := newSizeTestMirror(t, []string{mirror}, map[string]int64{mirror: 3 << 20})

	want := mirror + "beige/pool/main/a/pkg/pkg_1.0_amd64.deb"
	if got := s.makeChoice(sizeTestOriginal, sizeTestOfficial); got != want {
		t.Fatalf("makeChoice = %q, want %q", got, want)
	}
	if *expectedCalls != 1 {
		t.Fatalf("metadata lookups = %d, want 1", *expectedCalls)
	}
	if s.sniffBlocked(mirror) {
		t.Fatal("mirror blocked without metadata")
	}
}
