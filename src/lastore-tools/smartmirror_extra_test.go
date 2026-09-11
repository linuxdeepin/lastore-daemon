// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendSuffix(t *testing.T) {
	tests := []struct {
		r, suffix, want string
	}{
		{"http://example.com", "/", "http://example.com/"},
		{"http://example.com/", "/", "http://example.com/"},
		{"http://example.com", "/dists/", "http://example.com/dists/"},
		{"http://example.com/dists/", "/dists/", "http://example.com/dists/"},
	}
	for _, tt := range tests {
		got := appendSuffix(tt.r, tt.suffix)
		assert.Equal(t, tt.want, got)
	}
}

func TestGetMirrorListLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mirrors.json")
	content := `[{"url":"https://a.example.com"},{"url":"https://b.example.com"}]`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	urls, err := getMirrorList(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://a.example.com", "https://b.example.com"}, urls)
}

func TestGetMirrorListMissingFile(t *testing.T) {
	_, err := getMirrorList(filepath.Join(t.TempDir(), "nope.json"))
	assert.Error(t, err)
}

func TestGetMirrorListMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{not-json}`), 0644))

	_, err := getMirrorList(path)
	assert.Error(t, err)
}

func TestGetMirrorListHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"urlHttps":"a.example.com"},{"urlHttp":"b.example.com"}]`)
	}))
	defer server.Close()

	urls, err := getMirrorList(server.URL)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://a.example.com", "http://b.example.com"}, urls)
}

func TestGetMirrorListHTTPNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := getMirrorList(server.URL)
	assert.Error(t, err)
}
