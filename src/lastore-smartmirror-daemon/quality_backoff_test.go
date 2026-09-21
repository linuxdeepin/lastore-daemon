// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
)

func TestProbe2MCandidatesOrder(t *testing.T) {
	mq := newTestQuality(map[string]int{"never": 1, "no-actual": 3, "with-actual": 2})
	now := time.Now()
	probe4k := nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}
	mq.recordProbe4K("never", "u1", probe4k, nil)
	mq.recordProbe4K("no-actual", "u2", probe4k, nil)
	mq.recordProbe4K("with-actual", "u3", probe4k, nil)

	due := now.Add(-time.Hour)
	mq.QualityMap["no-actual"].Probe2MAt = now.Add(-2 * time.Hour)
	mq.QualityMap["no-actual"].Probe2MNextAt = due
	mq.QualityMap["with-actual"].Probe2MAt = now.Add(-2 * time.Hour)
	mq.QualityMap["with-actual"].Probe2MNextAt = due
	mq.QualityMap["with-actual"].ActualSpeed = 1 << 20
	mq.QualityMap["with-actual"].ActualSpeedAt = now

	got := mq.probe2MCandidates([]string{"with-actual", "never", "no-actual"}, now, 10)
	want := []string{"never", "no-actual", "with-actual"}
	if len(got) != len(want) {
		t.Fatalf("probe2MCandidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("probe2MCandidates = %v, want %v", got, want)
		}
	}
}

func TestProbe4KRefreshUsesSevenDays(t *testing.T) {
	mq := newTestQuality(map[string]int{"fresh": 1, "stale": 2})
	now := time.Now()
	mq.QualityMap["fresh"] = &Quality{Probe4KAt: now.Add(-24 * time.Hour)}
	mq.QualityMap["stale"] = &Quality{Probe4KAt: now.Add(-8 * 24 * time.Hour)}

	got := mq.probeCandidates([]string{"fresh", "stale"}, now, 10)
	if len(got) != 1 || got[0] != "stale" {
		t.Fatalf("probeCandidates = %v, want [stale]", got)
	}
}

func TestProbe4KMetadata(t *testing.T) {
	mq := newTestQuality(nil)
	now := time.Now()
	sample := nativesmartmirror.Sample{Bytes: probe4KBytes, Body: 3 * time.Millisecond, At: now}
	mq.recordProbe4K("m", "https://mirror.example/Packages", sample, nil)

	q := mq.QualityMap["m"]
	if q.Probe4KURI != "https://mirror.example/Packages" {
		t.Fatalf("uri = %q", q.Probe4KURI)
	}
	if q.Probe4KBytes != probe4KBytes {
		t.Fatalf("bytes = %d", q.Probe4KBytes)
	}
	if q.Probe4KBodyUs != 3000 {
		t.Fatalf("body us = %d", q.Probe4KBodyUs)
	}
}

func TestProbe2MBackoff(t *testing.T) {
	mq := newTestQuality(nil)
	mirror := "m"
	incomplete := nativesmartmirror.Sample{Bytes: probe2MBytes / 2, Body: time.Second, At: time.Now()}

	mq.recordProbe2M(mirror, "u", incomplete, nil)
	q := mq.QualityMap[mirror]
	if q.Probe2MFailures != 1 {
		t.Fatalf("failures = %d, want 1", q.Probe2MFailures)
	}
	if d := time.Until(q.Probe2MNextAt); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("first retry = %v, want ~24h", d)
	}

	mq.recordProbe2M(mirror, "u", incomplete, nil)
	if q.Probe2MFailures != 2 {
		t.Fatalf("failures = %d, want 2", q.Probe2MFailures)
	}
	if d := time.Until(q.Probe2MNextAt); d < 3*24*time.Hour || d > 5*24*time.Hour {
		t.Fatalf("second retry = %v, want ~4d", d)
	}

	complete := nativesmartmirror.Sample{Bytes: probe2MBytes, Body: time.Second, At: time.Now()}
	mq.recordProbe2M(mirror, "u", complete, nil)
	if q.Probe2MFailures != 0 {
		t.Fatalf("failures after success = %d, want 0", q.Probe2MFailures)
	}
	if d := time.Until(q.Probe2MNextAt); d < 15*24*time.Hour || d > 17*24*time.Hour {
		t.Fatalf("success retry without actual = %v, want ~16d", d)
	}

	q.ActualSpeed = 1 << 20
	q.ActualSpeedAt = time.Now()
	mq.recordProbe2M(mirror, "u", complete, nil)
	if d := time.Until(q.Probe2MNextAt); d < 29*24*time.Hour || d > 31*24*time.Hour {
		t.Fatalf("success retry with actual = %v, want ~30d", d)
	}
}

func TestQualityFreshnessUsesThirtyDays(t *testing.T) {
	now := time.Now()
	if !freshAt(now.Add(-29*24*time.Hour), now) {
		t.Fatal("29-day-old sample should be fresh")
	}
	if freshAt(now.Add(-31*24*time.Hour), now) {
		t.Fatal("31-day-old sample should not be fresh")
	}
}

func TestProbeCandidatesLazyOrder(t *testing.T) {
	mq := newTestQuality(map[string]int{"never": 1, "old": 2, "new": 3})
	now := time.Now()
	mq.QualityMap["old"] = &Quality{Probe4KAt: now.Add(-8 * 24 * time.Hour)}
	mq.QualityMap["new"] = &Quality{Probe4KAt: now.Add(-time.Hour)}

	got := mq.probeCandidates([]string{"new", "old", "never"}, now, 10)
	want := []string{"never", "old"}
	if len(got) != len(want) {
		t.Fatalf("probeCandidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("probeCandidates = %v, want %v", got, want)
		}
	}
}
