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

	// sniffCandidates bounds how many ranked mirrors one request checks before
	// it falls back to the official URL. The ranked head already holds the
	// fastest mirrors and the official CDN is authoritative, so walking the
	// whole mirror list would only add latency.
	sniffCandidates = 4

	// sniffBlockTTL keeps a mirror that returned a definitive miss out of the
	// selection. The block is deliberately keyed by mirror only, not by mirror
	// and object: a 404 for one archive path normally means the site's whole
	// archive is behind on sync (or its proxy/ACL rejects the path), so the
	// objects that follow are expected to miss there for the same reason.
	// Blocking the site briefly avoids one failed check per object during
	// exactly that window, while 15 minutes is short enough for a mirror that
	// finishes syncing to come back without a daemon restart.
	sniffBlockTTL = 15 * time.Minute

	// sniffUnavailableTTL keeps a mirror that failed the availability check
	// with a transport error out of the selection for a short while. Such a
	// failure does not prove the object is missing, so it must not use the
	// 15-minute 404 block, and it must not mark the 4 KiB gate failed: that
	// would exclude the mirror for qualityProbeTTL. The short cooldown only
	// stops every object in the same burst from re-paying the sniff timeout
	// while a blackholed or temporarily broken mirror is still ranked ahead
	// of healthy ones.
	sniffUnavailableTTL = 5 * time.Minute
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

	// expectedOnce guards the lazy metadata resolver used to compare the size a
	// mirror declares with the size the archive publishes. The resolver has its
	// own lock and cache.
	expectedOnce sync.Once
	expected     *expectedResolver

	probeStableMu       sync.Mutex
	probeStableURI      string
	probeStableOfficial string

	// sniffBlockedAt holds mirrors that returned a definitive miss recently,
	// keyed by mirror URL. The site-wide scope is intentional; see
	// blockMirrorForSniff. sniffUnavailableAt holds the separate, much shorter
	// cooldown for transport errors; see sniffUnavailableTTL.
	sniffMu            sync.Mutex
	sniffBlockedAt     map[string]time.Time
	sniffUnavailableAt map[string]time.Time
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

		sniffBlockedAt:     make(map[string]time.Time),
		sniffUnavailableAt: make(map[string]time.Time),
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
		// The static replacement only applies to URLs inside the official
		// archive. This mode is the caller's explicit mirror choice, so Query
		// must not contact mirrorHost, or any other host, here: Query is
		// reachable from the system bus by any local user, and a caller-supplied
		// replacement host would otherwise turn this handler into an outbound
		// request primitive (blind SSRF against loopback or private services).
		// Availability of the object is verified only in the enabled mode, where
		// the candidates come from the configured mirror list.
		if source != original && !strings.HasPrefix(original, officialMirror) {
			logger.Warningf("smartmirror Query[%d] static replace rejected original=%q replaced=%q reason=outside-official-archive fallback=official", id, original, source)
			return original, nil
		}
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
	// Probe results order the mirrors, but they only prove that a mirror
	// answered for some object at some earlier time. Before handing apt a
	// mirror URL, confirm that this mirror starts serving this exact object: a
	// mirror that has not synced the newest packages returns 404 while the
	// official index already lists them. The first mirror that answers wins;
	// after sniffCandidates misses the official URL is the authoritative
	// fallback.
	attempts := 0
	for _, mirror := range s.mirrorQuality.rank(s.sourcesURL, time.Now()) {
		if attempts >= sniffCandidates {
			break
		}
		if s.sniffBlocked(mirror) || s.sniffUnavailableRecently(mirror) {
			continue
		}
		target, ok := probeURI(original, officialMirror, mirror)
		if !ok {
			continue
		}
		attempts++
		result, declared := s.sniff(target)
		switch result {
		case sniffOK:
			// The object exists, but a mirror that is behind on sync or has a
			// damaged copy can serve an object of the wrong size. apt would
			// reject it against the archive Size, so reject it here instead and
			// let the next candidate or the official URL take the download.
			// Without a declared size there is nothing to compare, so the
			// metadata lookup is skipped entirely.
			if declared > 0 {
				if expected, ok := s.expectedSize(original); ok && declared != expected {
					s.blockMirrorForSniff(mirror)
					logger.Warningf("smartmirror select source=%q original=%q result=%q sniff=size-mismatch declared=%d expected=%d fallback=next", mirror, original, target, declared, expected)
					continue
				}
			}
			logger.Infof("smartmirror select source=%q original=%q result=%q sniff=ok declared=%d", mirror, original, target, declared)
			return target
		case sniffNotFound:
			// A definitive miss: the site's archive is behind on sync (or its
			// proxy/ACL rejects the path). Block the whole site for a while
			// instead of only this object: the objects that follow are expected
			// to miss for the same reason, so re-checking each one would only
			// add latency and reproduce the 404 storm this check exists to
			// stop.
			s.blockMirrorForSniff(mirror)
			logger.Debugf("smartmirror select source=%q original=%q result=%q sniff=not-found fallback=next", mirror, original, target)
		case sniffUnavailable:
			// Network error/timeout: a mirror may only be temporarily slow, so
			// this is not a definitive miss. A short cooldown is enough to stop
			// every object in the same burst from re-paying the sniff timeout.
			s.coolDownMirrorForSniff(mirror)
			logger.Debugf("smartmirror select source=%q original=%q result=%q sniff=unavailable cooldown=%s fallback=next", mirror, original, target, sniffUnavailableTTL)
		case sniffLocalUnavailable:
			// The local APT methods are unusable, which says nothing about this
			// mirror, so it must not be cached.
			logger.Debugf("smartmirror select source=%q original=%q result=%q sniff=local-unavailable fallback=next", mirror, original, target)
		}
	}
	logger.Infof("smartmirror select source=%q original=%q result=%q fallback=official", "", original, original)
	return original
}

// sniff runs the per-request availability check through the native APT probe so
// the check shares the proxy, TLS and authentication configuration of the
// download it gates. It also returns the object size the server declared, which
// makeChoice compares with the archive metadata. The seam keeps tests
// independent of the system apt methods.
func (s *SmartMirror) sniff(uri string) (sniffResult, int64) {
	return sniffResourceFn(s.prober, uri)
}

// expectedSize returns the size the archive publishes for rawURL. A false
// result means the metadata is unavailable and no size comparison can be made;
// it must not make the mirror unusable.
func (s *SmartMirror) expectedSize(rawURL string) (int64, bool) {
	return expectedSizeFn(s.resolver(), rawURL)
}

func (s *SmartMirror) resolver() *expectedResolver {
	s.expectedOnce.Do(func() {
		s.expected = newExpectedResolver()
	})
	return s.expected
}

// sniffBlocked reports whether mirror returned a definitive miss recently. The
// entry is per mirror by design; see blockMirrorForSniff for why.
func (s *SmartMirror) sniffBlocked(mirror string) bool {
	s.sniffMu.Lock()
	defer s.sniffMu.Unlock()
	at, ok := s.sniffBlockedAt[mirror]
	if !ok {
		return false
	}
	if time.Since(at) >= sniffBlockTTL {
		delete(s.sniffBlockedAt, mirror)
		return false
	}
	return true
}

// blockMirrorForSniff records that mirror could not serve the file just
// requested, keeping the site out of the selection for sniffBlockTTL. A single
// definitive miss is treated as evidence that the site is currently syncing its
// archive (or rejecting the path through a proxy/ACL), so the next objects are
// expected to miss there too. Keying the entry by object would re-run one native
// transfer per object during exactly that window, which is the burst of failing
// checks this block is meant to prevent.
func (s *SmartMirror) blockMirrorForSniff(mirror string) {
	s.sniffMu.Lock()
	defer s.sniffMu.Unlock()
	if s.sniffBlockedAt == nil {
		s.sniffBlockedAt = make(map[string]time.Time)
	}
	s.sniffBlockedAt[mirror] = time.Now()
}

// sniffUnavailableRecently reports whether mirror failed the availability check
// with a transport error recently. The cooldown is separate from the 404 block:
// a transport error does not prove the object is missing, so it must not keep
// the mirror out for sniffBlockTTL or mark the 4 KiB gate failed.
func (s *SmartMirror) sniffUnavailableRecently(mirror string) bool {
	s.sniffMu.Lock()
	defer s.sniffMu.Unlock()
	at, ok := s.sniffUnavailableAt[mirror]
	if !ok {
		return false
	}
	if time.Since(at) >= sniffUnavailableTTL {
		delete(s.sniffUnavailableAt, mirror)
		return false
	}
	return true
}

// coolDownMirrorForSniff records that mirror failed the availability check with
// a transport error. See sniffUnavailableTTL for why this is not the 404 block.
func (s *SmartMirror) coolDownMirrorForSniff(mirror string) {
	s.sniffMu.Lock()
	defer s.sniffMu.Unlock()
	if s.sniffUnavailableAt == nil {
		s.sniffUnavailableAt = make(map[string]time.Time)
	}
	s.sniffUnavailableAt[mirror] = time.Now()
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

// recordActualDownload receives one completed apt download and updates local
// state only. It is deliberately unexported: no component reports downloads
// since 58b2bda removed the lastore daemon reporter, so the actual-speed fields
// stay at their defaults and ranking uses the probe speed and the mirror weight.
// The signature keeps the reporter's fields so the sample path can be re-wired
// when a producer is available again.
func (s *SmartMirror) recordActualDownload(uri, usedMirror string, bytes, elapsedMs, waitMs int64, failed, bytesKnown bool) *dbus.Error {
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
