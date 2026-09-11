// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSaveMirrorInfos(t *testing.T) {
	infos := []MirrorInfo{
		{Name: "server1", Progress: 0.5, Support2014: true, Support2015: false},
		{Name: "server2", Progress: 1.0, Support2014: true, Support2015: true},
	}
	var buf bytes.Buffer
	err := SaveMirrorInfos(infos, &buf)
	assert.NoError(t, err)

	var decoded []MirrorInfo
	err = json.Unmarshal(buf.Bytes(), &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded, 2)
	assert.Equal(t, "server1", decoded[0].Name)
}

func TestSaveMirrorInfosEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := SaveMirrorInfos(nil, &buf)
	assert.NoError(t, err)
}

func TestU2014(t *testing.T) {
	assert.Equal(t, "http://example.com/dists/trusty/Release", u2014("http://example.com"))
	assert.Equal(t, "http://example.com/dists/trusty/Release", u2014("http://example.com/"))
}

func TestU2015(t *testing.T) {
	assert.Equal(t, "http://example.com/dists/unstable/Release", u2015("http://example.com"))
	assert.Equal(t, "http://example.com/dists/unstable/Release", u2015("http://example.com/"))
}

func TestUGuards(t *testing.T) {
	guards := []string{"g0", "g1", "g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9"}
	result := uGuards("http://example.com", guards)
	assert.Len(t, result, 2)
	assert.Equal(t, "http://example.com/g0", result[0])
	assert.Equal(t, "http://example.com/g5", result[1])
}

func TestUGuardsEmpty(t *testing.T) {
	result := uGuards("http://example.com", nil)
	assert.Empty(t, result)
}

func TestCheckURLExistsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "lastore-tools", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.True(t, result.Result)
	assert.Equal(t, http.StatusOK, result.ResultCode)
	assert.Equal(t, srv.URL, result.URL)
	assert.GreaterOrEqual(t, result.Latency, int64(0))
}

func TestCheckURLExistsRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.True(t, result.Result)
	assert.Equal(t, http.StatusMovedPermanently, result.ResultCode)
}

func TestCheckURLExistsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.False(t, result.Result)
	assert.Equal(t, http.StatusNotFound, result.ResultCode)
}

func TestCheckURLExistsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.False(t, result.Result)
	assert.Equal(t, http.StatusInternalServerError, result.ResultCode)
}

func TestCheckURLExistsInvalidURL(t *testing.T) {
	result := CheckURLExists("://invalid-url")
	assert.False(t, result.Result)
	assert.Equal(t, 0, result.ResultCode)
}

func TestCheckURLExistsConnectionError(t *testing.T) {
	result := CheckURLExists("http://127.0.0.1:1")
	assert.False(t, result.Result)
	assert.Equal(t, 0, result.ResultCode)
}

func TestParseIndex(t *testing.T) {
	indexData := []string{
		"http://mirror1.com",
		"http://mirror2.com",
		"http://mirror1.com",
		"http://mirror3.com",
		"http://mirror2.com",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(indexData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	result, err := ParseIndex(srv.URL)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Contains(t, result, "http://mirror1.com")
	assert.Contains(t, result, "http://mirror2.com")
	assert.Contains(t, result, "http://mirror3.com")
}

func TestParseIndexEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	result, err := ParseIndex(srv.URL)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseIndexInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{bad json}"))
	}))
	defer srv.Close()

	_, err := ParseIndex(srv.URL)
	assert.Error(t, err)
}

func TestParseIndexConnectionError(t *testing.T) {
	_, err := ParseIndex("http://127.0.0.1:1")
	assert.Error(t, err)
}

func TestParseIndexBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ParseIndex(srv.URL)
	assert.Error(t, err)
}

func TestFetchLastSyncValidHeader(t *testing.T) {
	want := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", want.Format(time.RFC1123))
	}))
	defer srv.Close()

	got := fetchLastSync(srv.URL)
	assert.True(t, got.Equal(want), "got %v, want %v", got, want)
}

func TestFetchLastSyncMissingHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	got := fetchLastSync(srv.URL)
	assert.Equal(t, time.Time{}, got)
}

func TestFetchLastSyncConnectionError(t *testing.T) {
	got := fetchLastSync("http://127.0.0.1:1")
	assert.Equal(t, time.Time{}, got)
}

func TestDetectServer(t *testing.T) {
	lastMod := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.json" {
			w.Header().Set("Last-Modified", lastMod.Format(time.RFC1123))
			_, _ = w.Write([]byte(`["__GUARD__1700000000"]`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	infos := DetectServer(2, "index.json", srv.URL, nil)

	require.Len(t, infos, 1)
	assert.Equal(t, srv.URL, infos[0].Name)
	assert.True(t, infos[0].Support2014)
	assert.True(t, infos[0].Support2015)
	assert.Equal(t, 1.0, infos[0].Progress)
	assert.True(t, infos[0].LastSync.Equal(lastMod))
}
