// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"
)

func TestSniffUnavailableCooldownExpires(t *testing.T) {
	mirror := "https://mirror.example/deepin/"
	s := &SmartMirror{
		sniffUnavailableAt: map[string]time.Time{
			mirror: time.Now().Add(-sniffUnavailableTTL - time.Second),
		},
	}

	if s.sniffUnavailableRecently(mirror) {
		t.Fatal("expired cooldown must not skip the mirror")
	}
	if _, ok := s.sniffUnavailableAt[mirror]; ok {
		t.Fatal("expired cooldown must be removed")
	}

	s.coolDownMirrorForSniff(mirror)
	if !s.sniffUnavailableRecently(mirror) {
		t.Fatal("fresh cooldown must skip the mirror")
	}
	if s.sniffBlocked(mirror) {
		t.Fatal("unavailable cooldown must not create a 404 block")
	}

	stale := "https://stale.example/deepin/"
	s.blockMirrorForSniff(stale)
	if s.sniffUnavailableRecently(stale) {
		t.Fatal("a 404 block must not create an unavailable cooldown")
	}
}

func TestSniffLocalUnavailableNotCached(t *testing.T) {
	old := sniffResourceFn
	t.Cleanup(func() { sniffResourceFn = old })
	sniffResourceFn = func(*aptProbe, string) sniffResult {
		return sniffLocalUnavailable
	}

	now := time.Now()
	mirror := "https://mirror.example/deepin/"
	s := &SmartMirror{
		sourcesURL: []string{mirror},
		mirrorQuality: MirrorQuality{
			QualityMap: QualityMap{mirror: &Quality{
				Probe4KAt:     now,
				Probe4KPassed: true,
				Probe2MNextAt: now.Add(time.Hour),
			}},
			weights: map[string]int{mirror: 1},
		},
		sniffBlockedAt:     make(map[string]time.Time),
		sniffUnavailableAt: make(map[string]time.Time),
	}

	const original = "https://official.example/deepin/pool/main/a/pkg.deb"
	const official = "https://official.example/deepin/"
	if got := s.makeChoice(original, official); got != original {
		t.Fatalf("makeChoice = %q, want official fallback %q", got, original)
	}
	if s.sniffUnavailableRecently(mirror) {
		t.Fatal("local APT unavailability must not cool down the mirror")
	}
	if s.sniffBlocked(mirror) {
		t.Fatal("local APT unavailability must not block the mirror")
	}
}
