// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"

	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
)

const (
	qualityDataFilepath = "smartmirror_quality.json"
	configDataFilepath  = "smartmirror_config.json"

	// qualityFlushDelay debounces quality file writes. A job can complete
	// hundreds of downloads and the file only needs the latest sample of every
	// mirror, so one write per burst keeps the D-Bus handler off disk I/O.
	qualityFlushDelay = 500 * time.Millisecond
)

// The STATE_DIRECTORY environment variable is set by systemd when starting this service.
var stateDirectory = os.Getenv("STATE_DIRECTORY")

// SmartMirror handle core smart mirror data
type SmartMirror struct {
	Enable bool

	config         *config
	service        *dbusutil.Service
	mirrorQuality  MirrorQuality
	sources        []system.MirrorSource
	sourcesURL     []string
	prober         *aptProbe
	qualityPath    string
	qualityDirty   chan struct{}
	probeQueueMu   sync.Mutex
	probeQueueCond *sync.Cond
	probeQueue     []probeRequest
	probeQueueOnce sync.Once
	taskMu         sync.Mutex
	taskCount      int
	queryID        atomic.Uint64

	probeStableMu       sync.Mutex
	probeStableURI      string
	probeStableOfficial string
}

type probeRequest struct {
	original, official string
	done               chan struct{}
}

// GetInterfaceName export dbus interface name
func (s *SmartMirror) GetInterfaceName() string {
	return "org.deepin.dde.Lastore1.Smartmirror"
}

// newSmartMirror return a object with dbus
func newSmartMirror(service *dbusutil.Service) *SmartMirror {
	s := &SmartMirror{
		service:   service,
		taskCount: 0,
		config:    newConfig(path.Join(stateDirectory, configDataFilepath)),
		mirrorQuality: MirrorQuality{
			QualityMap: make(QualityMap),
			weights:    make(map[string]int),
		},
		qualityPath:  path.Join(stateDirectory, qualityDataFilepath),
		qualityDirty: make(chan struct{}, 1),
	}
	s.prober = newAPTProbe(stateDirectory)
	go s.qualityWriter()

	s.Enable = s.config.Enable

	err := system.DecodeJson(path.Join(stateDirectory, qualityDataFilepath), &s.mirrorQuality.QualityMap)
	if nil != err {
		logger.Info("load quality.json failed", err)
	}
	s.mirrorQuality.prune(time.Now())

	err = system.DecodeJson(path.Join(stateDirectory, "mirrors.json"), &s.sources)
	if nil != err {
		logger.Error(err)
	}

	for _, source := range s.sources {
		s.sourcesURL = append(s.sourcesURL, source.Url)
		s.mirrorQuality.weights[source.Url] = source.Weight
	}
	// mirrors.json is the source of truth for candidate archive roots. Mirror
	// bases are upstream-style archive prefixes without the suite component
	// (for example https://mirrors.example/deepin/), so /beige/ remains in the
	// APT original URI and is preserved by prefix rewriting.
	return s
}

// SetEnable the best source
func (s *SmartMirror) SetEnable(enable bool) *dbus.Error {
	s.service.DelayAutoQuit()
	changed := s.Enable != enable

	s.Enable = enable
	err := s.config.setEnable(enable)
	if nil != err {
		logger.Errorf("save config failed: %v", err)
		return dbus.NewError(err.Error(), nil)
	}

	if changed {
		err := s.service.EmitPropertyChanged(s, "Enable", enable)
		if nil != err {
			return dbus.NewError(err.Error(), nil)
		}
	}
	return nil
}

// Query the best source
func (s *SmartMirror) Query(original, officialMirror, mirrorHost string) (url string, busErr *dbus.Error) {
	id := s.queryID.Add(1)
	// Ensure that both officialMirror and mirrorHost end with "/" to maintain URL concatenation consistency
	if !strings.HasSuffix(officialMirror, "/") {
		officialMirror += "/"
	}
	if !strings.HasSuffix(mirrorHost, "/") {
		mirrorHost += "/"
	}
	logger.Infof("smartmirror Query[%d] start original=%q official=%q mirror=%q enabled=%t", id, original, officialMirror, mirrorHost, s.Enable)
	s.service.DelayAutoQuit()
	if !s.Enable {
		source := strings.Replace(original, officialMirror, mirrorHost, 1)
		logger.Infof("smartmirror Query[%d] result=%q mode=disabled changed=%t", id, source, source != original)
		return source, nil
	}
	result := s.route(original, officialMirror)
	logger.Infof("smartmirror Query[%d] result=%q changed=%t", id, result, result != original)
	return result, nil
}

func (s *SmartMirror) canQuit() bool {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	return s.taskCount <= 0
}

func (s *SmartMirror) beginTask() {
	s.taskMu.Lock()
	s.taskCount++
	s.taskMu.Unlock()
}

func (s *SmartMirror) endTask() {
	s.taskMu.Lock()
	if s.taskCount > 0 {
		s.taskCount--
	}
	s.taskMu.Unlock()
}

// route select new url by file path
func (s *SmartMirror) route(original, officialMirror string) string {
	if !utils.ValidURL(original) || !utils.ValidURL(officialMirror) {
		// Just return raw url if there has any invalid input
		return original
	}

	if !strings.HasPrefix(original, officialMirror) {
		return original
	}

	if strings.Contains(original, "/pool/") {
		return s.makeChoice(original, officialMirror)
	} else if strings.Contains(original, "/dists/") && strings.HasSuffix(original, "Release") {
		// Get Release from Release
		url, _ := handleRequest(buildRequest(makeHeader(), "GET", original))
		return url
	} else if strings.Contains(original, "/dists/") && strings.Contains(original, "/by-hash/") {
		return s.makeChoice(original, officialMirror)
	}
	return original
}

// makeChoice select best mirror by http request
func (s *SmartMirror) makeChoice(original, officialMirror string) string {
	// A due evaluation is queued here and this helper waits for its own batch.
	// When cached results are still valid, runProbeBatch finds no work and
	// returns immediately.
	s.beginTask()
	defer s.endTask()
	s.runProbeBatch(original, officialMirror)
	for _, mirror := range s.mirrorQuality.rank(s.sourcesURL, time.Now()) {
		if target, ok := probeURI(original, officialMirror, mirror); ok {
			logger.Infof("smartmirror select source=%q original=%q result=%q", mirror, original, target)
			return target
		}
	}
	logger.Infof("smartmirror select source=%q original=%q result=%q fallback=official", "", original, original)
	return original
}

// runProbeBatch queues one evaluation for this Query and waits for it. Query
// must never return before the probe of its own batch has finished, otherwise
// the download would start on an unevaluated mirror.
func (s *SmartMirror) runProbeBatch(original, officialMirror string) {
	candidates := s.mirrorQuality.probeCandidates(s.sourcesURL, time.Now(), probeBatchSize)
	stage2Candidates := s.mirrorQuality.probe2MCandidates(s.sourcesURL, time.Now(), probeSpeedBatch)
	if len(candidates) == 0 && len(stage2Candidates) == 0 {
		return
	}
	s.ensureProbeQueue()
	s.beginTask()
	done := make(chan struct{})
	s.probeQueueMu.Lock()
	s.probeQueue = append(s.probeQueue, probeRequest{original: original, official: officialMirror, done: done})
	s.probeQueueCond.Signal()
	s.probeQueueMu.Unlock()
	<-done
}

func (s *SmartMirror) ensureProbeQueue() {
	s.probeQueueOnce.Do(func() {
		s.probeQueueMu.Lock()
		s.probeQueueCond = sync.NewCond(&s.probeQueueMu)
		s.probeQueueMu.Unlock()
		go s.probeWorker()
	})
}

func (s *SmartMirror) probeWorker() {
	var (
		backgroundRounds int
		lastOriginal     string
		lastOfficial     string
	)
	runOneRound := func(original, official string) bool {
		now := time.Now()
		candidates := s.mirrorQuality.probeCandidates(s.sourcesURL, now, probeBatchSize)
		stage2 := s.mirrorQuality.probe2MCandidates(s.sourcesURL, now, probeSpeedBatch)
		if len(candidates) == 0 && len(stage2) == 0 {
			return false
		}
		s.runProbes(original, official, candidates, stage2)
		return true
	}

	for {
		s.probeQueueMu.Lock()
		for len(s.probeQueue) == 0 && backgroundRounds == 0 {
			s.probeQueueCond.Wait()
		}
		if len(s.probeQueue) > 0 {
			req := s.probeQueue[0]
			s.probeQueue[0] = probeRequest{}
			s.probeQueue = s.probeQueue[1:]
			s.probeQueueMu.Unlock()

			// Each Query waits for exactly one round.
			runOneRound(req.original, req.official)
			lastOriginal, lastOfficial = req.original, req.official
			if backgroundRounds == 0 {
				backgroundRounds = probeRoundsPerBatch - 1
			}
			s.endTask()
			close(req.done)
			continue
		}

		// No Query waiting: keep running the background batch so coverage
		// continues in parallel with real downloads.
		original, official := lastOriginal, lastOfficial
		s.probeQueueMu.Unlock()

		s.beginTask()
		if runOneRound(original, official) {
			backgroundRounds--
		} else {
			backgroundRounds = 0
		}
		s.endTask()
	}
}

func (s *SmartMirror) runProbes(original, officialMirror string, candidates, stage2Candidates []string) {
	// The 4 KiB gates run in parallel and are joined before the 2 MiB probes,
	// so a slow mirror cannot delay the candidates that pass the gate.
	done := make(chan struct{}, len(candidates))
	for _, mirror := range candidates {
		mirror := mirror
		go func() {
			target, ok := probeURI(original, officialMirror, mirror)
			if !ok {
				logger.Warningf("smartmirror 4k probe skipped: mirror base %q cannot be rewritten for %q (official %q)", mirror, original, officialMirror)
				s.mirrorQuality.recordProbe4K(mirror, "", nativesmartmirror.Sample{At: time.Now()},
					fmt.Errorf("mirror base %q is not a usable http(s) URL", mirror))
				done <- struct{}{}
				return
			}
			sample, err := s.prober.run(target, probe4KBytes)
			if !errors.Is(err, errProbeLocal) {
				s.mirrorQuality.recordProbe4K(mirror, target, sample, err)
			}
			done <- struct{}{}
		}()
	}
	for range candidates {
		<-done
	}
	if len(candidates) > 0 {
		stage2Candidates = s.mirrorQuality.probe2MCandidates(s.sourcesURL, time.Now(), probeSpeedBatch)
	}

	// Speed probes use a stable, package-independent index file when one that
	// belongs to the official archive is available, so a small package cannot
	// finish early and produce an unrealistically high speed. Without such an
	// index the package URI itself is measured.
	base := s.probeBase(original, officialMirror)

	var wg sync.WaitGroup
	for _, mirror := range stage2Candidates {
		mirror := mirror
		wg.Add(1)
		go func() {
			defer wg.Done()
			target, ok := probeURI(base, officialMirror, mirror)
			if !ok {
				logger.Warningf("smartmirror 2m probe skipped: mirror base %q cannot be rewritten for %q (official %q)", mirror, base, officialMirror)
				s.mirrorQuality.recordProbe2M(mirror, "", nativesmartmirror.Sample{At: time.Now()},
					fmt.Errorf("mirror base %q is not a usable http(s) URL", mirror))
				return
			}
			sample, err := s.prober.run(target, probe2MBytes)
			if !errors.Is(err, errProbeLocal) {
				s.mirrorQuality.recordProbe2M(mirror, target, sample, err)
			}
		}()
	}
	wg.Wait()
	if len(candidates)+len(stage2Candidates) > 0 {
		if err := persistQualityData(&s.mirrorQuality, s.qualityPath); err != nil {
			logger.Warning("write probe quality failed:", err)
		}
	}
}

type stableProbeCandidate struct {
	uri         string
	official    bool
	nativeArch  bool
	packages    bool
	main        bool
	compression int
}

func newStableProbeCandidate(uri, officialMirror, arch string) stableProbeCandidate {
	c := stableProbeCandidate{
		uri:        uri,
		official:   strings.HasPrefix(uri, officialMirror),
		nativeArch: strings.Contains(uri, "/binary-"+arch+"/"),
		packages:   strings.Contains(uri, "/Packages"),
		main:       strings.Contains(uri, "/main/"),
	}
	switch {
	case strings.HasSuffix(uri, "/Packages"):
		c.compression = 0
	case strings.HasSuffix(uri, "/Packages.gz"):
		c.compression = 1
	case strings.HasSuffix(uri, "/Packages.xz"):
		c.compression = 2
	default:
		c.compression = 3
	}
	return c
}

func lessStableProbe(a, b stableProbeCandidate) bool {
	if a.official != b.official {
		return a.official
	}
	if a.nativeArch != b.nativeArch {
		return a.nativeArch
	}
	if a.packages != b.packages {
		return a.packages
	}
	if a.main != b.main {
		return a.main
	}
	if a.compression != b.compression {
		return a.compression < b.compression
	}
	return a.uri < b.uri
}

// pickStableProbeURI selects a stable Packages index URI for speed probes. It
// only returns index targets that belong to officialMirror, preferring the
// native architecture, a Packages target and the main component. A target from
// another archive is useless here: Rewrite rejects it, so every mirror would be
// recorded as a failed probe. Returning "" makes the caller fall back to the
// package URI it was given.
func pickStableProbeURI(output, officialMirror, arch string) string {
	var candidates []stableProbeCandidate
	for _, line := range strings.Split(output, "\n") {
		uri := strings.TrimSpace(line)
		if uri == "" {
			continue
		}
		candidates = append(candidates, newStableProbeCandidate(uri, officialMirror, arch))
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return lessStableProbe(candidates[i], candidates[j])
	})
	for _, c := range candidates {
		if c.official && c.nativeArch && c.packages {
			return c.uri
		}
	}
	for _, c := range candidates {
		if c.official {
			return c.uri
		}
	}
	return ""
}

func nativeArchitecture() (string, error) {
	out, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		return "", err
	}
	arch := strings.TrimSpace(string(out))
	if arch == "" {
		return "", fmt.Errorf("empty architecture")
	}
	return arch, nil
}

// discoverStableProbeURIFn is an injectable seam so tests can exercise the
// probe base validation without apt installed.
var discoverStableProbeURIFn = discoverStableProbeURI

// discoverStableProbeURI asks apt for the index targets of the configured
// sources and picks a Packages index for the native architecture. This keeps
// the probe target independent of the distribution suite, component and CPU
// architecture: apt reports the correct path for whatever sources are enabled.
func discoverStableProbeURI(officialMirror string) (string, error) {
	arch, err := nativeArchitecture()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "apt-get", "indextargets", "--format", "$(URI)").Output()
	if err != nil {
		return "", err
	}
	uri := pickStableProbeURI(string(out), officialMirror, arch)
	if uri == "" {
		return "", fmt.Errorf("no Packages index for architecture %s under %s", arch, officialMirror)
	}
	return uri, nil
}

// stableProbeOriginal returns the stable probe target for officialMirror,
// discovering it on first use. Only bases that belong to officialMirror are
// adopted: a base on another host or path cannot be rewritten onto the mirrors
// (Rewrite requires the same scheme, host and path prefix), so every 2 MiB probe
// would fail and be recorded against the mirrors instead of being measured.
// An undiscoverable or unusable base returns "", and the caller keeps using the
// package URI it was given. The result - including "" - is remembered per
// officialMirror so a broken environment does not re-run apt for every batch.
func (s *SmartMirror) stableProbeOriginal(officialMirror string) string {
	s.probeStableMu.Lock()
	defer s.probeStableMu.Unlock()
	if s.probeStableOfficial == officialMirror {
		return s.probeStableURI
	}
	s.probeStableOfficial = officialMirror
	s.probeStableURI = ""
	uri, err := discoverStableProbeURIFn(officialMirror)
	if err != nil {
		logger.Warningf("discover stable probe URI failed: %v", err)
		return ""
	}
	if !nativesmartmirror.SameBase(uri, officialMirror) {
		logger.Warningf("ignore stable probe URI %q: outside official prefix %q, using the package URI", uri, officialMirror)
		return ""
	}
	logger.Infof("smartmirror stable probe base=%q official=%q", uri, officialMirror)
	s.probeStableURI = uri
	return uri
}

// probeBase returns the source URI used for 2 MiB speed probes: the stable
// package-independent index when it belongs to officialMirror, otherwise the
// caller's package URI.
func (s *SmartMirror) probeBase(original, officialMirror string) string {
	if stable := s.stableProbeOriginal(officialMirror); stable != "" {
		return stable
	}
	return original
}

// RecordActualDownload receives one completed apt download. The lastore daemon
// reports asynchronously, so this method only updates local state.
func (s *SmartMirror) RecordActualDownload(uri, usedMirror string, bytes, elapsedMs, waitMs int64, failed, bytesKnown bool) *dbus.Error {
	if s.service != nil {
		s.service.DelayAutoQuit()
	}
	mirror := s.resolveActualMirror(uri, usedMirror)
	logger.Infof("smartmirror actual uri=%q used_mirror=%q bytes=%d elapsed_ms=%d wait_ms=%d failed=%t bytes_known=%t resolved=%q", uri, usedMirror, bytes, elapsedMs, waitMs, failed, bytesKnown, mirror)
	if mirror == "" {
		logger.Warningf("ignore actual smartmirror download with unknown mirror: uri=%q used=%q", uri, usedMirror)
		return nil
	}
	s.mirrorQuality.recordActualMeasured(mirror, bytes, time.Duration(elapsedMs)*time.Millisecond, failed, bytesKnown)
	s.markQualityDirty()
	return nil
}

// markQualityDirty schedules a debounced quality file write. Updates that
// arrive while a write is pending are covered by that write.
func (s *SmartMirror) markQualityDirty() {
	select {
	case s.qualityDirty <- struct{}{}:
	default:
	}
}

func (s *SmartMirror) qualityWriter() {
	for range s.qualityDirty {
		time.Sleep(qualityFlushDelay)
		s.flushQuality()
	}
}

// flushQuality writes the current quality state. It is called on daemon exit so
// the last reported samples are never lost.
func (s *SmartMirror) flushQuality() {
	if err := persistQualityData(&s.mirrorQuality, s.qualityPath); err != nil {
		logger.Warning("write actual smartmirror quality failed:", err)
	}
}

func (s *SmartMirror) resolveActualMirror(uri, usedMirror string) string {
	for _, candidate := range s.sourcesURL {
		if actualMirrorMatches(candidate, usedMirror) || actualMirrorMatches(candidate, uri) {
			return candidate
		}
	}
	return ""
}

func actualMirrorMatches(candidate, value string) bool {
	if candidate == "" || value == "" {
		return false
	}
	candidateURL, err := url.Parse(nativesmartmirror.NormalizeBase(candidate))
	if err != nil || candidateURL.Host == "" {
		return false
	}
	valueURL, err := url.Parse(value)
	if err != nil {
		return value == strings.TrimRight(candidate, "/") || value == candidateURL.Host
	}
	if valueURL.Scheme == "" && valueURL.Host == "" {
		return strings.Trim(value, "/") == candidateURL.Host
	}
	return valueURL.Scheme == candidateURL.Scheme && valueURL.Host == candidateURL.Host &&
		strings.HasPrefix(valueURL.EscapedPath(), candidateURL.EscapedPath())
}
