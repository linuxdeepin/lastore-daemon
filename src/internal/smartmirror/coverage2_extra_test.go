// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeMethodDir creates a temporary MethodsDir containing an executable
// script named <name> that speaks the APT method protocol: it sends
// 100 Capabilities, reads two messages (601 Config + 600 URI Acquire)
// from stdin, then writes the given response to stdout.
func fakeMethodDir(t *testing.T, name, response string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf '100 Capabilities\\n\\n'\n" +
		"blank=0\n" +
		"while IFS= read -r line; do\n" +
		"  [ -z \"$line\" ] && blank=$((blank + 1))\n" +
		"  [ \"$blank\" -ge 2 ] && break\n" +
		"done\n" +
		"printf '" + response + "'\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake method: %v", err)
	}
	return dir
}

func TestReadMessageLeadingBlankLine(t *testing.T) {
	in := "\n600 URI Acquire\n\n"
	m, err := ReadMessage(bufio.NewReader(strings.NewReader(in)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Code != 600 {
		t.Fatalf("code = %d, want 600", m.Code)
	}
}

func TestReadMessageInvalidField(t *testing.T) {
	_, err := ReadMessage(bufio.NewReader(strings.NewReader("600 URI\nNoColonHere\n\n")))
	if err == nil {
		t.Fatal("accepted field line without colon")
	}
}

func TestOncePrlimitNotFound(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	n := Native{Config: nativeConfig(), Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		request(t, "http://example.invalid/file", nil),
		Budget{Total: 2 * time.Second, Bytes: 1 << 20}, nil, nil)
	if res.Err == nil {
		t.Fatal("expected error when prlimit is not found")
	}
}

func TestFetchSlowCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "65536")
		w.Write([]byte("hello"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	slow := func(bytes int64, elapsed time.Duration) bool { return true }
	res := (Native{Config: nativeConfig(), Stderr: io.Discard}).
		Fetch(context.Background(), request(t, srv.URL, nil),
			Budget{Total: 5 * time.Second}, nil, slow)
	if res.Err == nil {
		t.Fatal("expected slow callback error")
	}
	if !strings.Contains(res.Err.Error(), "slower") {
		t.Fatalf("unexpected error: %v", res.Err)
	}
}

func TestFetchInvalidRedirectURI(t *testing.T) {
	dir := fakeMethodDir(t, "http", "103 Redirect\\nNew-URI: ftp://bad.example/file\\n\\n")
	n := Native{MethodsDir: dir, Config: nativeConfig(), Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		request(t, "http://example.invalid/file", nil),
		Budget{Total: 5 * time.Second}, nil, nil)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "invalid redirect") {
		t.Fatalf("expected invalid redirect error, got: %v", res.Err)
	}
}

func TestFetchHTTPSRedirectToHTTPDisabled(t *testing.T) {
	dir := fakeMethodDir(t, "https", "103 Redirect\\nNew-URI: http://plain.example/file\\n\\n")
	cfg := nativeConfig()
	cfg.Fields = append(cfg.Fields, Field{"Config-Item", "Acquire::https::AllowRedirect=false"})
	n := Native{MethodsDir: dir, Config: cfg, Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		request(t, "https://example.invalid/file", nil),
		Budget{Total: 5 * time.Second}, nil, nil)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "redirect disabled") {
		t.Fatalf("expected redirect disabled error, got: %v", res.Err)
	}
}

func TestFetchTooManyRedirects(t *testing.T) {
	dir := fakeMethodDir(t, "http", "103 Redirect\\nNew-URI: http://example.invalid/file\\n\\n")
	n := Native{MethodsDir: dir, Config: nativeConfig(), Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		request(t, "http://example.invalid/file", nil),
		Budget{Total: 30 * time.Second}, nil, nil)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "too many redirects") {
		t.Fatalf("expected too many redirects error, got: %v", res.Err)
	}
}

func TestOnceUnsupportedMessageCode(t *testing.T) {
	dir := fakeMethodDir(t, "http", "999 Unknown\\n\\n")
	n := Native{MethodsDir: dir, Config: nativeConfig(), Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		request(t, "http://example.invalid/file", nil),
		Budget{Total: 5 * time.Second}, nil, nil)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "unsupported native message") {
		t.Fatalf("expected unsupported message error, got: %v", res.Err)
	}
}

func TestOnceMethodStopsBeforeCompletion(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf '100 Capabilities\\n\\n'\n" +
		"blank=0\n" +
		"while IFS= read -r line; do\n" +
		"  [ -z \"$line\" ] && blank=$((blank + 1))\n" +
		"  [ \"$blank\" -ge 2 ] && break\n" +
		"done\n"
	path := filepath.Join(dir, "http")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake method: %v", err)
	}
	n := Native{MethodsDir: dir, Config: nativeConfig(), Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		request(t, "http://example.invalid/file", nil),
		Budget{Total: 5 * time.Second}, nil, nil)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "stopped before completion") {
		t.Fatalf("expected stopped before completion error, got: %v", res.Err)
	}
}
