// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
)

const (
	qualityProbeTTL        = 30 * 24 * time.Hour
	probe4KRefresh         = 7 * 24 * time.Hour
	minSpeedBytesPerSecond = 1 << 10
	maxSpeedBytesPerSecond = 500 << 20

	// probe2MShortRetry is when a probe whose target ended before the 2 MiB
	// window is repeated. Such a probe is inconclusive, not a mirror failure, so
	// it must not feed the failure backoff.
	probe2MShortRetry = 24 * time.Hour

	// Actual download samples are kept in a bounded recent window. Old samples
	// are only dropped when both the sample count and the byte volume exceed
	// their thresholds; the hard maximum protects against pathological runs
	// made of many tiny packages.
	actualWindowMaxSamples = 20
	actualWindowMaxBytes   = 128 << 20
	actualWindowHardMax    = 200
)

// ActualSample is one successful real download used for byte-weighted speed
// estimation.
type ActualSample struct {
	Bytes     int64     `json:"bytes"`
	ElapsedUs int64     `json:"elapsed_us"`
	At        time.Time `json:"at"`
}

// Quality is the latest measurement of one mirror. All samples expire after
// qualityProbeTTL, so an entry that is older than the TTL is equivalent to a
// mirror that has never been measured.
type Quality struct {
	Probe4KAt     time.Time `json:"probe_4k_at,omitempty"`
	Probe4KPassed bool      `json:"probe_4k_passed,omitempty"`
	Probe4KURI    string    `json:"probe_4k_uri,omitempty"`
	Probe4KBytes  int64     `json:"probe_4k_bytes,omitempty"`
	Probe4KBodyUs int64     `json:"probe_4k_body_us,omitempty"`

	Probe2MAt       time.Time `json:"probe_2m_at,omitempty"`
	Probe2MSpeed    float64   `json:"probe_2m_speed,omitempty"`
	Probe2MURI      string    `json:"probe_2m_uri,omitempty"`
	Probe2MBytes    int64     `json:"probe_2m_bytes,omitempty"`
	Probe2MBodyUs   int64     `json:"probe_2m_body_us,omitempty"`
	Probe2MComplete bool      `json:"probe_2m_complete,omitempty"`
	Probe2MNextAt   time.Time `json:"probe_2m_next_at,omitempty"`
	Probe2MFailures int       `json:"probe_2m_failures,omitempty"`

	ActualSpeed   float64        `json:"actual_speed,omitempty"`
	ActualSpeedAt time.Time      `json:"actual_speed_at,omitempty"`
	ActualSamples []ActualSample `json:"actual_samples,omitempty"`
}

type QualityMap map[string]*Quality

type MirrorQuality struct {
	QualityMap
	weights   map[string]int
	persistMu sync.Mutex
	mux       sync.Mutex
}

func (mq *MirrorQuality) baseWeight(mirror string) int {
	return mq.weights[mirror]
}

func (mq *MirrorQuality) getQuality(mirror string) *Quality {
	if q := mq.QualityMap[mirror]; q != nil {
		return q
	}
	q := &Quality{}
	mq.QualityMap[mirror] = q
	return q
}

func freshAt(at, now time.Time) bool {
	return !at.IsZero() && !now.Before(at) && now.Sub(at) < qualityProbeTTL
}

func refreshDue(at, now time.Time, interval time.Duration) bool {
	return at.IsZero() || !now.Before(at) && now.Sub(at) >= interval
}

func probe2MFresh(q *Quality, now time.Time) bool {
	return q != nil && freshAt(q.Probe2MAt, now)
}

func probe2MDue(q *Quality, now time.Time) bool {
	if q == nil {
		return false
	}
	return q.Probe2MNextAt.IsZero() || !now.Before(q.Probe2MNextAt)
}

func (q *Quality) hasFreshActual(now time.Time) bool {
	return q != nil && q.ActualSpeed > 0 && freshAt(q.ActualSpeedAt, now)
}

// sampleTime keeps the observation time of a probe, falling back to now when
// the APT method transfer did not report one.
func sampleTime(sample nativesmartmirror.Sample) time.Time {
	if sample.At.IsZero() {
		return time.Now()
	}
	return sample.At
}

// prune drops entries whose newest sample is older than the probe TTL. Such an
// entry can no longer influence selection, so keeping it only grows the file.
func (mq *MirrorQuality) prune(now time.Time) {
	mq.mux.Lock()
	defer mq.mux.Unlock()
	for mirror, q := range mq.QualityMap {
		q.pruneActualSamples(now)
		latest := q.Probe4KAt
		if q.Probe2MAt.After(latest) {
			latest = q.Probe2MAt
		}
		if q.ActualSpeedAt.After(latest) {
			latest = q.ActualSpeedAt
		}
		if latest.IsZero() || now.Sub(latest) >= qualityProbeTTL {
			delete(mq.QualityMap, mirror)
		}
	}
}

func clampSpeed(bytes int64, elapsed time.Duration) float64 {
	if bytes <= 0 || elapsed <= 0 {
		return 0
	}
	speed := float64(bytes) / elapsed.Seconds()
	return math.Min(maxSpeedBytesPerSecond, math.Max(minSpeedBytesPerSecond, speed))
}

// pruneActualSamples drops samples outside the TTL and recomputes the
// byte-weighted actual speed.
func (q *Quality) pruneActualSamples(now time.Time) {
	if len(q.ActualSamples) == 0 {
		return
	}
	kept := q.ActualSamples[:0]
	for _, sample := range q.ActualSamples {
		if now.Sub(sample.At) < qualityProbeTTL {
			kept = append(kept, sample)
		}
	}
	q.ActualSamples = kept
	if len(q.ActualSamples) == 0 {
		q.ActualSpeed, q.ActualSpeedAt = 0, time.Time{}
		return
	}
	q.ActualSpeed = q.windowActualSpeed()
	q.ActualSpeedAt = q.ActualSamples[len(q.ActualSamples)-1].At
}

// windowActualSpeed returns the byte-weighted speed over the retained window.
func (q *Quality) windowActualSpeed() float64 {
	var bytes int64
	var elapsedUs int64
	for _, sample := range q.ActualSamples {
		bytes += sample.Bytes
		elapsedUs += sample.ElapsedUs
	}
	if bytes <= 0 || elapsedUs <= 0 {
		return 0
	}
	return clampSpeed(bytes, time.Duration(elapsedUs)*time.Microsecond)
}

func (q *Quality) windowActualBytes() int64 {
	var bytes int64
	for _, sample := range q.ActualSamples {
		bytes += sample.Bytes
	}
	return bytes
}

// trimActualSamples keeps the recent window bounded by both sample count and
// byte volume, with a hard maximum for pathological tiny-package runs.
func (q *Quality) trimActualSamples() {
	for len(q.ActualSamples) > actualWindowHardMax ||
		(len(q.ActualSamples) > actualWindowMaxSamples && q.windowActualBytes() > actualWindowMaxBytes) {
		q.ActualSamples = q.ActualSamples[1:]
	}
}

// effectiveSpeed prefers the measured speed of a real download and falls back
// to the 2 MiB probe while that probe is fresh.
func (q *Quality) effectiveSpeed(now time.Time) float64 {
	if freshAt(q.ActualSpeedAt, now) && q.ActualSpeed > 0 {
		return q.ActualSpeed
	}
	if probe2MFresh(q, now) {
		return q.Probe2MSpeed
	}
	return 0
}

// probeCandidates returns the mirrors whose 4 KiB gate needs a probe, at most
// limit of them. Lazy expansion order: never probed first, then oldest 4K
// sample, then highest configured weight. Equal entries are shuffled.
func (mq *MirrorQuality) probeCandidates(mirrors []string, now time.Time, limit int) []string {
	if limit <= 0 {
		return nil
	}
	mq.mux.Lock()
	defer mq.mux.Unlock()
	type candidate struct {
		mirror string
		at     time.Time
		weight int
	}
	due := make([]candidate, 0, len(mirrors))
	for _, mirror := range mirrors {
		q := mq.QualityMap[mirror]
		if q != nil && !refreshDue(q.Probe4KAt, now, probe4KRefresh) {
			continue
		}
		var at time.Time
		if q != nil {
			at = q.Probe4KAt
		}
		due = append(due, candidate{mirror: mirror, at: at, weight: mq.baseWeight(mirror)})
	}
	rand.Shuffle(len(due), func(i, j int) { due[i], due[j] = due[j], due[i] })
	sort.SliceStable(due, func(i, j int) bool {
		a, b := due[i], due[j]
		aNever, bNever := a.at.IsZero(), b.at.IsZero()
		if aNever != bNever {
			return aNever
		}
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		return a.weight > b.weight
	})
	if len(due) > limit {
		due = due[:limit]
	}
	out := make([]string, 0, len(due))
	for _, item := range due {
		out = append(out, item.mirror)
	}
	return out
}

// probe2MCandidates returns the mirrors that passed a fresh 4 KiB gate and are
// due for a 2 MiB speed probe. The order is:
//
//  1. never probed 2M
//  2. no fresh actual speed
//  3. oldest probe_2m_next_at
//  4. highest configured weight
func (mq *MirrorQuality) probe2MCandidates(mirrors []string, now time.Time, limit int) []string {
	if limit <= 0 {
		return nil
	}
	mq.mux.Lock()
	defer mq.mux.Unlock()
	type candidate struct {
		mirror string
		q      *Quality
		weight int
	}
	all := make([]candidate, 0, len(mirrors))
	for _, mirror := range mirrors {
		q := mq.QualityMap[mirror]
		if q == nil || !freshAt(q.Probe4KAt, now) || !q.Probe4KPassed || !probe2MDue(q, now) {
			continue
		}
		all = append(all, candidate{mirror: mirror, q: q, weight: mq.baseWeight(mirror)})
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		aNever, bNever := a.q.Probe2MAt.IsZero(), b.q.Probe2MAt.IsZero()
		if aNever != bNever {
			return aNever
		}
		aFresh, bFresh := a.q.hasFreshActual(now), b.q.hasFreshActual(now)
		if aFresh != bFresh {
			return !aFresh
		}
		if !a.q.Probe2MNextAt.Equal(b.q.Probe2MNextAt) {
			return a.q.Probe2MNextAt.Before(b.q.Probe2MNextAt)
		}
		return a.weight > b.weight
	})
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]string, 0, len(all))
	for _, item := range all {
		out = append(out, item.mirror)
	}
	return out
}

// probe4KPassed reports whether a gate probe proves that the mirror served the
// object apt asked for. Filling the byte budget proves it for an object larger
// than the budget; an object smaller than the budget proves it only when the
// whole object arrived. When the method reported no size, a transfer that
// completed without filling the budget delivered the whole object.
func probe4KPassed(sample nativesmartmirror.Sample, err error) bool {
	if err != nil {
		return false
	}
	switch {
	case sample.Bytes >= probe4KBytes:
		return true
	case sample.ExpectedBytes > 0:
		return sample.Bytes == sample.ExpectedBytes
	default:
		return sample.Bytes > 0
	}
}

// recordProbe4K stores one 4 KiB gate probe. The gate answers "can this mirror
// serve the object apt is fetching", so the object size decides the criterion:
// requiring probe4KBytes from every transfer would mark every mirror as failing
// whenever the object itself is smaller than the probe budget, and rank() would
// drop them all until the gate is refreshed.
func (mq *MirrorQuality) recordProbe4K(mirror, uri string, sample nativesmartmirror.Sample, err error) {
	mq.mux.Lock()
	defer mq.mux.Unlock()
	q := mq.getQuality(mirror)
	q.Probe4KAt = sampleTime(sample)
	q.Probe4KPassed = probe4KPassed(sample, err)
	q.Probe4KURI = uri
	q.Probe4KBytes = sample.Bytes
	q.Probe4KBodyUs = sample.Body.Microseconds()
}

// recordProbe2M stores one 2 MiB speed probe. The probe speed is kept even when
// the probe stopped before the byte cap: the bytes and time already observed are
// still a measurement, they only do not open the 4 KiB gate. A probe whose
// target ended before the window - a Packages index smaller than 2 MiB, or a
// package URI used when no stable index is available - was served completely, so
// it is inconclusive rather than a mirror failure: probe_2m_failures stays and
// the probe is retried after probe2MShortRetry. Only a probe that really failed
// escalates the 1/4/16/30 day backoff. The probed URI, byte count and body time
// are persisted.
func (mq *MirrorQuality) recordProbe2M(mirror, uri string, sample nativesmartmirror.Sample, err error) {
	mq.mux.Lock()
	defer mq.mux.Unlock()
	q := mq.getQuality(mirror)
	now := time.Now()
	q.Probe2MAt = sampleTime(sample)
	q.Probe2MURI = uri
	q.Probe2MBytes = sample.Bytes
	q.Probe2MBodyUs = sample.Body.Microseconds()
	q.Probe2MSpeed = clampSpeed(sample.Bytes, sample.Body)
	complete := err == nil && sample.Bytes >= probe2MBytes
	q.Probe2MComplete = complete
	switch {
	case complete:
		q.Probe2MFailures = 0
		if q.hasFreshActual(now) {
			q.Probe2MNextAt = now.Add(30 * 24 * time.Hour)
		} else {
			q.Probe2MNextAt = now.Add(16 * 24 * time.Hour)
		}
	case err == nil:
		q.Probe2MNextAt = now.Add(probe2MShortRetry)
	default:
		q.Probe2MFailures++
		switch {
		case q.Probe2MFailures <= 1:
			q.Probe2MNextAt = now.Add(24 * time.Hour)
		case q.Probe2MFailures == 2:
			q.Probe2MNextAt = now.Add(4 * 24 * time.Hour)
		case q.Probe2MFailures == 3:
			q.Probe2MNextAt = now.Add(16 * 24 * time.Hour)
		default:
			q.Probe2MNextAt = now.Add(30 * 24 * time.Hour)
		}
	}
}

// recordActualMeasured stores one completed download. A sample only updates the
// speed when it succeeded, its byte count is known and it carries an elapsed
// time; the elapsed time is the method-side monotonic duration, never the time
// at which lastore-daemon observed the event. The speed is byte-weighted over
// the retained recent window.
func (mq *MirrorQuality) recordActualMeasured(mirror string, bytes int64, elapsed time.Duration, failed, bytesKnown bool) {
	mq.mux.Lock()
	defer mq.mux.Unlock()
	q := mq.getQuality(mirror)
	if failed || !bytesKnown || bytes <= 0 || elapsed <= 0 {
		return
	}
	q.ActualSamples = append(q.ActualSamples, ActualSample{
		Bytes:     bytes,
		ElapsedUs: elapsed.Microseconds(),
		At:        time.Now(),
	})
	q.trimActualSamples()
	q.ActualSpeed = q.windowActualSpeed()
	q.ActualSpeedAt = q.ActualSamples[len(q.ActualSamples)-1].At
}

// rank orders the mirrors best first. A mirror that failed a fresh 4 KiB gate
// is left out until the gate is refreshed; the rest are ordered by effective
// speed and then by configured weight.
func (mq *MirrorQuality) rank(mirrors []string, now time.Time) []string {
	mq.mux.Lock()
	defer mq.mux.Unlock()
	out := make([]string, 0, len(mirrors))
	for _, mirror := range mirrors {
		if q := mq.QualityMap[mirror]; q != nil && freshAt(q.Probe4KAt, now) && !q.Probe4KPassed {
			continue
		}
		out = append(out, mirror)
	}
	score := func(mirror string) float64 {
		q := mq.QualityMap[mirror]
		if q == nil {
			return 0
		}
		speed := q.effectiveSpeed(now)
		if speed <= 0 {
			return 0
		}
		return speed + float64(mq.baseWeight(mirror))*0.001
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := score(out[i]), score(out[j])
		if a != b {
			return a > b
		}
		return mq.baseWeight(out[i]) > mq.baseWeight(out[j])
	})
	return out
}
