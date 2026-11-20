// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
)

type repoStub struct {
	srv    *httptest.Server
	hits   atomic.Int64
	status int
}

func newRepoStub(t *testing.T, status int) *repoStub {
	t.Helper()
	r := &repoStub{status: status}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits.Add(1)
		if r.status == http.StatusOK {
			w.Header().Set("Content-Length", "16")
			_, _ = w.Write([]byte("0123456789abcdef"))
			return
		}
		w.WriteHeader(r.status)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// setupState writes the state directory the daemon reads at construction.
func setupState(t *testing.T, enable bool, mirrors []string, weights []int, seedQuality bool) {
	t.Helper()
	dir := t.TempDir()
	old := stateDirectory
	stateDirectory = dir
	t.Cleanup(func() { stateDirectory = old })

	type source struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		Url    string `json:"url"`
		Weight int    `json:"weight"`
	}
	sources := make([]source, 0, len(mirrors))
	for i, u := range mirrors {
		sources = append(sources, source{Id: u, Name: u, Url: u, Weight: weights[i]})
	}
	writeJSONFile(t, filepath.Join(dir, "mirrors.json"), sources)
	writeJSONFile(t, filepath.Join(dir, "smartmirror_config.json"), map[string]bool{"Enable": enable})

	if seedQuality {
		now := time.Now()
		q := make(map[string]map[string]any)
		for _, u := range mirrors {
			q[u] = map[string]any{
				"probe_4k_at":      now.Format(time.RFC3339Nano),
				"probe_4k_passed":  true,
				"probe_4k_uri":     u + "old-package.deb", // passed on a DIFFERENT file
				"probe_2m_next_at": now.Add(24 * time.Hour).Format(time.RFC3339Nano),
			}
		}
		writeJSONFile(t, filepath.Join(dir, "smartmirror_quality.json"), q)
	}
}

// requireNativeAPT skips a sniff scenario when the system APT methods are not
// usable: the availability check would then report "unavailable" and the test
// would assert the wrong thing.
func requireNativeAPT(t *testing.T, uri string) {
	t.Helper()
	if !newAPTProbe("").available(uri) {
		t.Skip("native apt methods unavailable, sniff scenario skipped")
	}
}

func TestQueryIntegration_EnableTrue_SeededQuality(t *testing.T) {
	official := newRepoStub(t, http.StatusOK)
	stale := newRepoStub(t, http.StatusNotFound)
	good := newRepoStub(t, http.StatusOK)
	staleURL, goodURL := stale.srv.URL+"/", good.srv.URL+"/"
	requireNativeAPT(t, official.srv.URL)

	// Fresh 4K "passed on another file" => gate skips both, rank keeps both,
	// the sniff is the only thing that can catch the stale mirror.
	setupState(t, true, []string{staleURL, goodURL}, []int{100000, 10000}, true)

	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	got, err := s.Query(original, official.srv.URL+"/", "https://mirror.invalid/")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	want := goodURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if got != want {
		t.Fatalf("Query = %q, want %q", got, want)
	}
	if !s.sniffBlocked(staleURL) {
		t.Fatal("stale mirror must be blocked after the sniff returned 404")
	}
	if s.sniffBlocked(goodURL) {
		t.Fatal("good mirror must not be blocked")
	}
	if s.sniffUnavailableRecently(staleURL) {
		t.Fatal("a 404 must use the definitive-miss block, not the unavailable cooldown")
	}
	// one sniff per mirror: stale first (404), good second (200) => stop.
	if stale.hits.Load() != 1 || good.hits.Load() != 1 {
		t.Fatalf("hits stale=%d good=%d, want 1/1 (no extra traversal after the hit)",
			stale.hits.Load(), good.hits.Load())
	}
	// the stored 4K record must stay "passed" for the other file: the sniff
	// decision must not rewrite the gate result.
	q := s.mirrorQuality.QualityMap[staleURL]
	if q == nil || !q.Probe4KPassed {
		t.Fatal("sniff must not overwrite the 4K gate record")
	}
}

func TestQueryIntegration_EnableTrue_UnreachableCandidate(t *testing.T) {
	official := newRepoStub(t, http.StatusOK)
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL + "/"
	dead.Close() // connection refused => sniffUnavailable
	good := newRepoStub(t, http.StatusOK)
	goodURL := good.srv.URL + "/"
	requireNativeAPT(t, official.srv.URL)

	setupState(t, true, []string{deadURL, goodURL}, []int{100000, 10000}, true)
	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	got, err := s.Query(original, official.srv.URL+"/", "https://mirror.invalid/")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	want := goodURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if got != want {
		t.Fatalf("Query = %q, want %q", got, want)
	}
	if s.sniffBlocked(deadURL) {
		t.Fatal("an unreachable mirror must not be blacklisted")
	}
	if !s.sniffUnavailableRecently(deadURL) {
		t.Fatal("an unreachable mirror must be cooled down briefly")
	}
}

func TestQueryIntegration_EnableTrue_ServerErrorNotBlocked(t *testing.T) {
	official := newRepoStub(t, http.StatusOK)
	broken := newRepoStub(t, http.StatusInternalServerError)
	good := newRepoStub(t, http.StatusOK)
	brokenURL, goodURL := broken.srv.URL+"/", good.srv.URL+"/"
	requireNativeAPT(t, official.srv.URL)

	setupState(t, true, []string{brokenURL, goodURL}, []int{100000, 10000}, true)
	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	got, err := s.Query(original, official.srv.URL+"/", "https://mirror.invalid/")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	want := goodURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if got != want {
		t.Fatalf("Query = %q, want %q", got, want)
	}
	if s.sniffBlocked(brokenURL) {
		t.Fatal("a 5xx response is transient and must not block the mirror")
	}
	if !s.sniffUnavailableRecently(brokenURL) {
		t.Fatal("a 5xx response must use the short unavailable cooldown")
	}
}

func TestQueryIntegration_EnableTrue_UnavailableHeadCoolsDown(t *testing.T) {
	official := newRepoStub(t, http.StatusOK)
	requireNativeAPT(t, official.srv.URL)

	mirrors := make([]string, 0, sniffCandidates+1)
	weights := make([]int, 0, sniffCandidates+1)
	for i := 0; i < sniffCandidates; i++ {
		dead := httptest.NewServer(http.NotFoundHandler())
		mirrors = append(mirrors, dead.URL+"/")
		weights = append(weights, 100000-i*10000)
		dead.Close() // connection refused => sniffUnavailable
	}
	good := newRepoStub(t, http.StatusOK)
	goodURL := good.srv.URL + "/"
	mirrors = append(mirrors, goodURL)
	weights = append(weights, 10000)

	setupState(t, true, mirrors, weights, true)
	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	officialBase := official.srv.URL + "/"

	// First Query: the four head candidates each fail with a transport error
	// and consume the sniff budget before the fifth mirror is reached.
	first, err := s.Query(original, officialBase, "https://mirror.invalid/")
	if err != nil {
		t.Fatalf("first Query error: %v", err)
	}
	if first != original {
		t.Fatalf("first Query = %q, want official fallback %q", first, original)
	}
	if good.hits.Load() != 0 {
		t.Fatalf("fifth mirror must not be sniffed while the head consumes the budget, got %d hits", good.hits.Load())
	}
	for _, deadURL := range mirrors[:sniffCandidates] {
		if !s.sniffUnavailableRecently(deadURL) {
			t.Fatalf("unavailable mirror %q must be cooling down", deadURL)
		}
		if s.sniffBlocked(deadURL) {
			t.Fatalf("transport error for %q must not use the 404 block", deadURL)
		}
	}

	// Second Query: the four unavailable candidates are skipped by the short
	// cooldown, so the fifth mirror is checked and selected.
	second, err := s.Query(original, officialBase, "https://mirror.invalid/")
	if err != nil {
		t.Fatalf("second Query error: %v", err)
	}
	want := goodURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if second != want {
		t.Fatalf("second Query = %q, want %q", second, want)
	}
	if good.hits.Load() != 1 {
		t.Fatalf("fifth mirror hits = %d, want 1", good.hits.Load())
	}
}

func TestQueryIntegration_EnableTrue_NoQuality_GateAbsorbs(t *testing.T) {
	official := newRepoStub(t, http.StatusOK)
	stale := newRepoStub(t, http.StatusNotFound)
	good := newRepoStub(t, http.StatusOK)
	staleURL, goodURL := stale.srv.URL+"/", good.srv.URL+"/"

	requireNativeAPT(t, official.srv.URL)
	// No quality file: the 4K gate probes the due mirrors with this URL first.
	setupState(t, true, []string{staleURL, goodURL}, []int{100000, 10000}, false)

	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	got, err := s.Query(original, official.srv.URL+"/", "https://mirror.invalid/")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	want := goodURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if got != want {
		t.Fatalf("Query = %q, want %q", got, want)
	}
	q := s.mirrorQuality.QualityMap[staleURL]
	if q == nil || q.Probe4KPassed {
		t.Fatalf("gate should have marked the stale mirror failed, got %+v", q)
	}
	if s.sniffBlocked(staleURL) {
		t.Fatal("control: the stale mirror was filtered by the gate, so it must not have been sniffed")
	}
}

func TestQueryIntegration_EnableFalse(t *testing.T) {
	official := newRepoStub(t, http.StatusOK)
	mirror := newRepoStub(t, http.StatusNotFound)
	mirrorURL := mirror.srv.URL + "/"

	setupState(t, false, []string{mirrorURL}, []int{100000}, false)
	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	officialBase := official.srv.URL + "/"

	// Enable=false is the caller's explicit mirror choice: the URL is rewritten
	// statically and no availability check runs, even though the mirror would
	// answer 404. Per-object verification belongs to the enabled mode, where
	// candidates come from the configured mirror list and the host cannot be
	// chosen by the caller.
	got, err := s.Query(original, officialBase, mirrorURL)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	want := mirrorURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if got != want {
		t.Fatalf("Enable=false = %q, want %q", got, want)
	}
	if mirror.hits.Load() != 0 {
		t.Fatalf("Enable=false must not contact the mirror, got %d requests", mirror.hits.Load())
	}
	if official.hits.Load() != 0 {
		t.Fatalf("Enable=false must not contact the official host, got %d requests", official.hits.Load())
	}
}
