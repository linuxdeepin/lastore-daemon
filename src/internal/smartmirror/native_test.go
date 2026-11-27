// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// requireAPTMethod skips tests that exercise the real apt method when apt is not
// installed, so a minimal build container does not fail; the daemon itself
// always runs with apt.
func requireAPTMethod(t *testing.T, scheme string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(defaultMethodsDir, scheme)); err != nil {
		t.Skipf("APT %s method not installed: %v", scheme, err)
	}
}

// requirePrlimit skips tests that rely on the kernel-enforced byte cap.
func requirePrlimit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed (util-linux)")
	}
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
	requireAPTMethod(t, "http")

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
	requireAPTMethod(t, "http")
	requirePrlimit(t)

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
	requireAPTMethod(t, "http")
	requirePrlimit(t)

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
	requireAPTMethod(t, "http")

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
	requireAPTMethod(t, "http")
	requirePrlimit(t)

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
	requireAPTMethod(t, "http")
	requirePrlimit(t)

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

// trustFile stores cert as a PEM trust anchor. The HTTPS tests configure APT's
// CaInfo option with it so trust comes from the test itself rather than from
// the build host's ca-certificates package.
func trustFile(t *testing.T, cert *x509.Certificate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trust.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write trust anchor: %v", err)
	}
	return path
}

// unrelatedCertificate returns a self-signed certificate that cannot have
// signed the test server's certificate.
func unrelatedCertificate(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "unrelated test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func TestNativeHTTPSUsesAPTTrustConfiguration(t *testing.T) {
	requireAPTMethod(t, "https")

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("tls")) }))
	defer srv.Close()

	cases := []struct {
		name          string
		ca            string
		disableVerify bool
		wantSuccess   bool
	}{
		{"CA configured through APT", trustFile(t, srv.Certificate()), false, true},
		{"unrelated CA rejected", trustFile(t, unrelatedCertificate(t)), false, false},
		{"verification disabled without a system store", "", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := nativeConfig()
			if tc.ca != "" {
				cfg.Fields = append(cfg.Fields, Field{"Config-Item", "Acquire::https::CaInfo=" + tc.ca})
			}
			if tc.disableVerify {
				cfg.Fields = append(cfg.Fields, Field{"Config-Item", "Acquire::https::Verify-Peer=false"}, Field{"Config-Item", "Acquire::https::Verify-Host=false"})
			}
			res := (Native{Config: cfg, Stderr: io.Discard}).Fetch(context.Background(), request(t, srv.URL, nil), Budget{Total: time.Second}, nil, nil)
			if tc.wantSuccess {
				if res.Err != nil || res.Done.Code != 201 {
					t.Fatalf("APT TLS configuration not applied: %+v", res)
				}
				return
			}
			if res.Err == nil {
				t.Fatalf("untrusted TLS certificate accepted: %+v", res)
			}
		})
	}
}

// APT reports a missing system trust store as a 104 Warning and then continues.
// Warnings must not abort the transfer, or HTTPS fails on hosts without
// ca-certificates even when verification is disabled.
func TestNativeWarningDoesNotAbortTransfer(t *testing.T) {
	dir := t.TempDir()
	method := filepath.Join(dir, "https")
	script := "#!/bin/sh\n" +
		"printf '104 Warning\\nMessage: No system certificates available. Try installing ca-certificates.\\n\\n'\n" +
		"printf '201 URI Done\\nURI: https://127.0.0.1:1/file\\nSize: 3\\n\\n'\n"
	if err := os.WriteFile(method, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake method: %v", err)
	}
	res := (Native{MethodsDir: dir, Config: nativeConfig(), Stderr: io.Discard}).
		Fetch(context.Background(), request(t, "https://127.0.0.1:1/file", nil), Budget{Total: time.Second}, nil, nil)
	if res.Err != nil || res.Done.Code != 201 {
		t.Fatalf("warning aborted the transfer: %+v", res)
	}
}

// The gate needs the object size the method learned from the server, so a
// transfer that stops at the byte budget can be told apart from a complete small
// object. The size is announced in 200 URI Start when the server sends a
// Content-Length, and only in 201 URI Done when it does not.
func TestNativeProbeReportsObjectSize(t *testing.T) {
	requireAPTMethod(t, "http")
	requirePrlimit(t)

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
