// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
	"time"

	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
)

func newTestQuality(weights map[string]int) *MirrorQuality {
	return &MirrorQuality{QualityMap: QualityMap{}, weights: weights}
}

func TestProbeSpeedIsClamped(t *testing.T) {
	if got := clampSpeed(1, time.Second); got != minSpeedBytesPerSecond {
		t.Fatalf("minimum speed = %v", got)
	}
	if got := clampSpeed(1<<40, time.Millisecond); got != maxSpeedBytesPerSecond {
		t.Fatalf("maximum speed = %v", got)
	}
}

func TestRankPrefersActualSpeed(t *testing.T) {
	mq := newTestQuality(nil)
	now := time.Now()
	mq.recordProbe4K("probe", "https://mirror.example/probe", nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}, nil)
	mq.recordProbe4K("actual", "https://mirror.example/actual", nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}, nil)
	mq.recordProbe2M("probe", "https://mirror.example/probe", nativesmartmirror.Sample{Bytes: probe2MBytes, Body: time.Second, At: now}, nil)
	mq.recordActualMeasured("actual", 20<<20, time.Second, false, true)

	ranked := mq.rank([]string{"probe", "actual"}, time.Now())
	if len(ranked) != 2 || ranked[0] != "actual" {
		t.Fatalf("rank = %v", ranked)
	}
}

func TestRankSkipsMirrorThatFailedFreshGate(t *testing.T) {
	mq := newTestQuality(nil)
	mq.recordProbe4K("bad", "https://mirror.example/bad", nativesmartmirror.Sample{Bytes: probe4KBytes, At: time.Now()}, errors.New("probe failed"))

	ranked := mq.rank([]string{"bad", "untested"}, time.Now())
	if len(ranked) != 1 || ranked[0] != "untested" {
		t.Fatalf("rank = %v", ranked)
	}
}

func TestSmallCompletedProbeKeepsItsSpeed(t *testing.T) {
	mq := newTestQuality(nil)
	mq.recordProbe2M("small", "https://mirror.example/small", nativesmartmirror.Sample{Bytes: 64 << 10, Body: time.Second, At: time.Now()}, nil)

	if q := mq.QualityMap["small"]; q.Probe2MSpeed != 64<<10 {
		t.Fatalf("small completed probe = %+v", q)
	}
}

// The gate probes the object apt is currently fetching, so a completed transfer
// of an object smaller than the 4 KiB budget proves the mirror serves it: it must
// not be recorded as a failing gate, otherwise rank() drops every probed mirror
// for probe4KRefresh and the 2 MiB stage never runs.
func TestProbe4KGateAcceptsSmallCompletedObject(t *testing.T) {
	mq := newTestQuality(map[string]int{"small": 1})
	now := time.Now()
	mq.recordProbe4K("small", "https://mirror.example/tiny.deb",
		nativesmartmirror.Sample{Bytes: 2048, ExpectedBytes: 2048, Body: 20 * time.Millisecond, At: now}, nil)

	if q := mq.QualityMap["small"]; !q.Probe4KPassed {
		t.Fatalf("small completed probe marked failed: %+v", q)
	}
	if ranked := mq.rank([]string{"small"}, now); len(ranked) != 1 {
		t.Fatalf("rank dropped the mirror: %v", ranked)
	}
	if due := mq.probe2MCandidates([]string{"small"}, now, 5); len(due) != 1 || due[0] != "small" {
		t.Fatalf("2 MiB stage not reached: %v", due)
	}
}

// The gate must prove that the object apt asked for was served: an object
// smaller than the byte budget only passes when the whole object arrived, an
// object larger than the budget passes when it filled the budget.
func TestProbe4KGateCriteria(t *testing.T) {
	cases := []struct {
		name          string
		bytes         int64
		expectedBytes int64
		err           error
		want          bool
	}{
		{"small object delivered whole", 2048, 2048, nil, true},
		{"small object truncated", 1024, 2048, nil, false},
		{"object larger than the budget fills it", probe4KBytes, 1 << 20, nil, true},
		{"object larger than the budget stops short", 1024, 1 << 20, nil, false},
		{"object exactly the budget", probe4KBytes, probe4KBytes, nil, true},
		{"unknown size completed", 2048, 0, nil, true},
		{"unknown size without a body", 0, 0, nil, false},
		{"failed probe", 2048, 2048, errors.New("404 Not Found"), false},
		{"failed probe after the budget", probe4KBytes, 1 << 20, errors.New("500 Connection failed"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mq := newTestQuality(nil)
			mq.recordProbe4K("m", "https://mirror.example/a.deb",
				nativesmartmirror.Sample{Bytes: tc.bytes, ExpectedBytes: tc.expectedBytes, At: time.Now()}, tc.err)

			if got := mq.QualityMap["m"].Probe4KPassed; got != tc.want {
				t.Fatalf("Probe4KPassed = %v, want %v", got, tc.want)
			}
		})
	}
}

// A mirror that did not deliver the object stays out of the ranking until the
// gate is refreshed.
func TestProbe4KGateRejectionExcludesTheMirror(t *testing.T) {
	mq := newTestQuality(nil)
	now := time.Now()
	mq.recordProbe4K("truncated", "https://mirror.example/a.deb",
		nativesmartmirror.Sample{Bytes: 1024, ExpectedBytes: 3072, At: now}, nil)
	mq.recordProbe4K("missing", "https://mirror.example/b.deb",
		nativesmartmirror.Sample{At: now}, errors.New("404 Not Found"))

	ranked := mq.rank([]string{"truncated", "missing", "untested"}, now)
	if len(ranked) != 1 || ranked[0] != "untested" {
		t.Fatalf("rank = %v, want [untested]", ranked)
	}
}

func TestProbe2MRecordsExactTarget(t *testing.T) {
	mq := newTestQuality(nil)
	now := time.Now()
	mq.recordProbe2M("mirror", "https://mirror.example/pkg.deb",
		nativesmartmirror.Sample{Bytes: 64 << 10, Body: 2 * time.Second, At: now}, nil)

	q := mq.QualityMap["mirror"]
	if q.Probe2MURI != "https://mirror.example/pkg.deb" {
		t.Fatalf("probe uri = %q", q.Probe2MURI)
	}
	if q.Probe2MBytes != 64<<10 {
		t.Fatalf("probe bytes = %d", q.Probe2MBytes)
	}
	if q.Probe2MBodyUs != (2 * time.Second).Microseconds() {
		t.Fatalf("probe body us = %d", q.Probe2MBodyUs)
	}
	if q.Probe2MComplete {
		t.Fatal("small probe must not be marked complete")
	}
}

func TestProbe2MCompleteFlag(t *testing.T) {
	mq := newTestQuality(nil)
	mq.recordProbe2M("mirror", "https://mirror.example/large.deb",
		nativesmartmirror.Sample{Bytes: probe2MBytes, Body: time.Second, At: time.Now()}, nil)

	if q := mq.QualityMap["mirror"]; !q.Probe2MComplete {
		t.Fatalf("complete probe not marked: %+v", q)
	}
}

func TestActualUnknownBytesDoNotBecomeSpeed(t *testing.T) {
	mq := newTestQuality(nil)
	mq.recordActualMeasured("mirror", 0, 0, false, false)

	if q := mq.QualityMap["mirror"]; q.ActualSpeed != 0 || !q.ActualSpeedAt.IsZero() {
		t.Fatalf("unknown actual = %+v", q)
	}
}

func TestActualWindowKeepsAllSamplesUnderByteCap(t *testing.T) {
	mq := newTestQuality(nil)
	const sampleBytes = 1 << 20
	for i := 0; i < 30; i++ {
		mq.recordActualMeasured("mirror", sampleBytes, time.Second, false, true)
	}

	q := mq.QualityMap["mirror"]
	if len(q.ActualSamples) != 30 {
		t.Fatalf("samples = %d, want 30", len(q.ActualSamples))
	}
	if q.ActualSpeed != sampleBytes {
		t.Fatalf("actual speed = %v, want %d", q.ActualSpeed, sampleBytes)
	}
}

func TestActualWindowTrimsWhenBothLimitsExceeded(t *testing.T) {
	mq := newTestQuality(nil)
	const sampleBytes = 8 << 20
	for i := 0; i < 30; i++ {
		mq.recordActualMeasured("mirror", sampleBytes, time.Second, false, true)
	}

	q := mq.QualityMap["mirror"]
	if len(q.ActualSamples) > actualWindowMaxSamples {
		t.Fatalf("samples = %d, want <= %d", len(q.ActualSamples), actualWindowMaxSamples)
	}
	if len(q.ActualSamples) > actualWindowMaxSamples && q.windowActualBytes() > actualWindowMaxBytes {
		t.Fatalf("window still exceeds both limits: samples=%d bytes=%d", len(q.ActualSamples), q.windowActualBytes())
	}
	if q.ActualSpeed != sampleBytes {
		t.Fatalf("actual speed = %v, want %d", q.ActualSpeed, sampleBytes)
	}
}

func TestActualWindowHardMaxAppliesToTinySamples(t *testing.T) {
	mq := newTestQuality(nil)
	for i := 0; i < actualWindowHardMax+50; i++ {
		mq.recordActualMeasured("mirror", 1<<10, time.Millisecond, false, true)
	}

	q := mq.QualityMap["mirror"]
	if len(q.ActualSamples) > actualWindowHardMax {
		t.Fatalf("samples = %d, want <= %d", len(q.ActualSamples), actualWindowHardMax)
	}
}

func TestEffectiveSpeedPrefersActualOverHigherProbe(t *testing.T) {
	mq := newTestQuality(nil)
	now := time.Now()
	mq.recordProbe4K("mirror", "https://mirror.example/mirror", nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}, nil)
	mq.recordProbe2M("mirror", "https://mirror.example/pkg", nativesmartmirror.Sample{Bytes: probe2MBytes, Body: time.Millisecond, At: now}, nil)
	mq.recordActualMeasured("mirror", 2<<20, time.Second, false, true)

	q := mq.QualityMap["mirror"]
	if q.Probe2MSpeed <= q.ActualSpeed {
		t.Fatalf("test setup: probe=%v actual=%v", q.Probe2MSpeed, q.ActualSpeed)
	}
	if got := q.effectiveSpeed(time.Now()); got != q.ActualSpeed {
		t.Fatalf("effective speed = %v, want actual %v", got, q.ActualSpeed)
	}
}

func TestProbeCandidatesSkipFreshAndOrderByWeight(t *testing.T) {
	mq := newTestQuality(map[string]int{"a": 1, "b": 2, "c": 3})
	now := time.Now()
	mq.recordProbe4K("c", "https://mirror.example/c", nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}, nil)

	got := mq.probeCandidates([]string{"a", "b", "c"}, now, 5)
	if len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("candidates = %v", got)
	}
}

func TestProbe2MCandidatesNeedPassingFreshGate(t *testing.T) {
	mq := newTestQuality(nil)
	now := time.Now()
	mq.recordProbe4K("pass", "https://mirror.example/pass", nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}, nil)
	mq.recordProbe4K("fail", "https://mirror.example/fail", nativesmartmirror.Sample{Bytes: probe4KBytes, At: now}, errors.New("probe failed"))

	got := mq.probe2MCandidates([]string{"pass", "fail"}, now, 5)
	if len(got) != 1 || got[0] != "pass" {
		t.Fatalf("stage2 candidates = %v", got)
	}

	mq.recordProbe2M("pass", "https://mirror.example/pass", nativesmartmirror.Sample{Bytes: probe2MBytes, Body: time.Second, At: now}, nil)
	if got := mq.probe2MCandidates([]string{"pass"}, now, 5); len(got) != 0 {
		t.Fatalf("fresh stage2 probe repeated: %v", got)
	}
}
