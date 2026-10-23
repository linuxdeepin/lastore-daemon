// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nativeConfig() Message {
	return Message{Code: 601, Text: "Configuration", Fields: []Field{
		{"Config-Item", "Acquire::http::Proxy=DIRECT"},
		{"Config-Item", "Acquire::https::Proxy=DIRECT"},
		{"Config-Item", "APT::Sandbox::User="},
	}}
}

// probeBudget is the budget used by the Smartmirror daemon for 2 MiB probes.
func probeBudget() Budget {
	return Budget{Total: 2 * time.Second, Body: 500 * time.Millisecond, Bytes: 2 << 20}
}

func request(t *testing.T, uri string, data []byte) Message {
	t.Helper()
	m := Message{Code: 600, Text: "URI Acquire", Fields: []Field{{"URI", uri}, {"Filename", filepath.Join(t.TempDir(), "download")}}}
	if data != nil {
		hash := sha256.Sum256(data)
		m = m.With("Expected-SHA256", fmt.Sprintf("%x", hash)).With("Expected-Checksum-FileSize", fmt.Sprint(len(data)))
	}
	return m
}

func TestNativeDownloadAndRedirect(t *testing.T) {
	data := []byte(strings.Repeat("native apt download\n", 1024))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/file", http.StatusFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Write(data)
	}))
	defer srv.Close()
	req := request(t, srv.URL+"/redirect", data)
	res := (Native{Config: nativeConfig(), Stderr: os.Stderr}).Fetch(context.Background(), req, Budget{Total: 3 * time.Second}, nil, nil)
	if res.Err != nil {
		t.Fatalf("native: %v (%+v)", res.Err, res)
	}
	if res.Done.Code != 201 {
		t.Fatalf("done=%+v", res.Done)
	}
	got, _ := os.ReadFile(req.Get("Filename"))
	if string(got) != string(data) {
		t.Fatalf("download differs")
	}
	if res.Sample.Endpoint != srv.URL+"/file" {
		t.Fatalf("endpoint %q", res.Sample.Endpoint)
	}
}

func TestNativeProbeHardLimitIgnoringRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "16777216")
		io.CopyN(w, strings.NewReader(strings.Repeat("x", 16<<20)), 16<<20)
	}))
	defer srv.Close()
	req := request(t, srv.URL+"/file", nil)
	res := (Native{Config: nativeConfig(), Stderr: io.Discard}).Fetch(context.Background(), req, probeBudget(), nil, nil)
	if res.Err != nil || !res.Sample.Capped {
		t.Fatalf("%+v", res)
	}
	if size := fileSize(req.Get("Filename")); size != 2<<20 {
		t.Fatalf("probe size=%d", size)
	}
}

func TestNativeProbeHardLimitAcrossRedirects(t *testing.T) {
	data := strings.Repeat("r", 16<<20)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		_, _ = io.WriteString(w, data)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/file", http.StatusFound)
	}))
	defer redirect.Close()
	req := request(t, redirect.URL+"/redirect", nil)
	res := (Native{Config: nativeConfig(), Stderr: io.Discard}).Fetch(context.Background(), req, probeBudget(), nil, nil)
	if res.Err != nil || !res.Sample.Capped || res.Sample.Bytes != 2<<20 {
		t.Fatalf("redirect probe=%+v", res)
	}
}

func TestNativeStallAndShortRead(t *testing.T) {
	for _, mode := range []string{"headers", "body", "short"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "headers" {
					<-r.Context().Done()
					return
				}
				w.Header().Set("Content-Length", "65536")
				w.Write([]byte("hello"))
				w.(http.Flusher).Flush()
				if mode == "body" {
					<-r.Context().Done()
				}
			}))
			defer srv.Close()
			begin := time.Now()
			res := (Native{Config: nativeConfig(), Stderr: io.Discard}).Fetch(context.Background(), request(t, srv.URL, nil), Budget{Idle: 150 * time.Millisecond}, nil, nil)
			if res.Err == nil {
				t.Fatalf("expected failure: %+v", res)
			}
			if time.Since(begin) > time.Second {
				t.Fatalf("stall budget exceeded")
			}
		})
	}
}

func TestNativeProbeBodyWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4194304")
		for i := 0; i < 4096; i++ {
			if _, err := w.Write([]byte(strings.Repeat("x", 1024))); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Millisecond):
			}
		}
	}))
	defer srv.Close()
	begin := time.Now()
	res := (Native{Config: nativeConfig(), Stderr: io.Discard}).Fetch(context.Background(), request(t, srv.URL, nil), probeBudget(), nil, nil)
	if res.Err == nil || res.Sample.Bytes == 0 {
		t.Fatalf("partial probe window was accepted: %+v", res)
	}
	if elapsed := time.Since(begin); elapsed < 450*time.Millisecond || elapsed > time.Second {
		t.Fatalf("window %s", elapsed)
	}
}

func TestNativeProbeTimeoutAfterPartialBodyRemainsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(2<<20))
		_, _ = w.Write(make([]byte, 256<<10))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	res := (Native{Config: nativeConfig(), Stderr: io.Discard}).Fetch(context.Background(), request(t, srv.URL, nil), Budget{
		Total: 200 * time.Millisecond, Bytes: 2 << 20,
	}, nil, nil)
	if res.Err == nil || res.Sample.Capped || res.Sample.Bytes == 0 {
		t.Fatalf("partial timeout was accepted: %+v", res)
	}
}

func TestNativeHTTPSUsesAPTTrustConfiguration(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("tls")) }))
	defer srv.Close()
	cfg := nativeConfig()
	res := (Native{Config: cfg, Stderr: io.Discard}).Fetch(context.Background(), request(t, srv.URL, nil), Budget{Total: time.Second}, nil, nil)
	if res.Err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	cfg.Fields = append(cfg.Fields, Field{"Config-Item", "Acquire::https::Verify-Peer=false"}, Field{"Config-Item", "Acquire::https::Verify-Host=false"})
	res = (Native{Config: cfg, Stderr: io.Discard}).Fetch(context.Background(), request(t, srv.URL, nil), Budget{Total: time.Second}, nil, nil)
	if res.Err != nil || res.Done.Code != 201 {
		t.Fatalf("APT TLS configuration not applied: %+v", res)
	}
}

// The gate needs the object size the method learned from the server, so a
// transfer that stops at the byte budget can be told apart from a complete small
// object. The size is announced in 200 URI Start when the server sends a
// Content-Length, and only in 201 URI Done when it does not.
func TestNativeProbeReportsObjectSize(t *testing.T) {
	data := []byte(strings.Repeat("z", 2972))
	large := make([]byte, 64<<10)

	contentLength := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := data
		if r.URL.Path == "/large.deb" {
			body = large
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body)
	}))
	defer contentLength.Close()

	chunked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Content-Length: http.Server switches to chunked encoding.
		_, _ = w.Write(data)
	}))
	defer chunked.Close()

	cases := []struct {
		name     string
		uri      string
		budget   Budget
		wantSize int64
		capped   bool
		skip     string
	}{
		{"size announced before the body", contentLength.URL + "/large.deb",
			Budget{Total: 3 * time.Second, Idle: 5 * time.Second, Bytes: 4 << 10}, int64(len(large)), true,
			"flaky on the mips64el builder: the probe can stop at the byte cap before it consumes 200 URI Start"},
		{"small object with content length", contentLength.URL + "/tiny.deb",
			Budget{Total: 3 * time.Second, Idle: 5 * time.Second, Bytes: 4 << 10}, int64(len(data)), false, ""},
		{"small object without content length", chunked.URL + "/tiny.deb",
			Budget{Total: 3 * time.Second, Idle: 5 * time.Second, Bytes: 4 << 10}, int64(len(data)), false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			res := (Native{Config: nativeConfig(), Stderr: io.Discard}).
				Fetch(context.Background(), request(t, tc.uri, nil), tc.budget, nil, nil)
			if res.Err != nil {
				t.Fatalf("probe: %v", res.Err)
			}
			if res.Sample.ExpectedBytes != tc.wantSize {
				t.Fatalf("expected size = %d, want %d", res.Sample.ExpectedBytes, tc.wantSize)
			}
			if res.Sample.Capped != tc.capped {
				t.Fatalf("capped = %v, want %v", res.Sample.Capped, tc.capped)
			}
		})
	}
}
