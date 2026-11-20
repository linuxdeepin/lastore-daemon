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
	stale := newRepoStub(t, http.StatusNotFound)
	good := newRepoStub(t, http.StatusOK)
	staleURL, goodURL := stale.srv.URL+"/", good.srv.URL+"/"

	setupState(t, false, []string{staleURL, goodURL}, []int{100000, 10000}, false)
	s := newSmartMirror(dbusutil.NewService(nil))
	original := official.srv.URL + "/beige/pool/main/a/pkg_1.0_all.deb"
	officialBase := official.srv.URL + "/"
	requireNativeAPT(t, official.srv.URL)

	// stale mirror: static replacement is rejected, official URL is kept.
	got, err := s.Query(original, officialBase, staleURL)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if got != original {
		t.Fatalf("Enable=false with stale mirror = %q, want the official %q", got, original)
	}

	// good mirror: static replacement applies.
	got, err = s.Query(original, officialBase, goodURL)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	want := goodURL + "beige/pool/main/a/pkg_1.0_all.deb"
	if got != want {
		t.Fatalf("Enable=false with good mirror = %q, want %q", got, want)
	}
}
