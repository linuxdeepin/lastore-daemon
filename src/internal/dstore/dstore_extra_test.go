// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dstore

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-ini/ini"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	s := NewStore()
	require.NotNil(t, s)
}

func TestNewStoreFallbackToDefault(t *testing.T) {
	origPath, origDefault := appstoreConfPath, appstoreConfPathDefault
	defer func() { appstoreConfPath, appstoreConfPathDefault = origPath, origDefault }()

	appstoreConfPath = filepath.Join(t.TempDir(), "missing.ini")
	appstoreConfPathDefault = filepath.Join(t.TempDir(), "settings.ini.default")
	require.NoError(t, os.WriteFile(appstoreConfPathDefault, []byte("[general]\nmetadata_server = http://example.invalid\n"), 0o644))

	s := NewStore()
	require.NotNil(t, s)
	require.NotNil(t, s.sysCfg)
}

func TestNewStoreBothMissing(t *testing.T) {
	origPath, origDefault := appstoreConfPath, appstoreConfPathDefault
	defer func() { appstoreConfPath, appstoreConfPathDefault = origPath, origDefault }()

	appstoreConfPath = filepath.Join(t.TempDir(), "missing.ini")
	appstoreConfPathDefault = filepath.Join(t.TempDir(), "also-missing.ini")

	s := NewStore()
	require.NotNil(t, s)
	assert.Nil(t, s.sysCfg)
}

func TestNewStoreWithConfig(t *testing.T) {
	cfg := ini.Empty()
	s := NewStoreWithConfig(cfg)
	require.NotNil(t, s)
	assert.Same(t, cfg, s.sysCfg)
}

func TestGetMetadataServerNilConfig(t *testing.T) {
	s := &Store{sysCfg: nil}
	server, err := s.GetMetadataServer()
	assert.Error(t, err)
	assert.Empty(t, server)
}

func TestGetMetadataServerMissingKey(t *testing.T) {
	// An empty ini file has no [General]/Server key.
	s := &Store{sysCfg: ini.Empty()}
	server, err := s.GetMetadataServer()
	assert.Error(t, err)
	assert.Empty(t, server)
}

func TestGetMetadataServerSuccess(t *testing.T) {
	f := ini.Empty()
	sec, err := f.NewSection("General")
	require.NoError(t, err)
	_, err = sec.NewKey("Server", "https://metadata.example.com")
	require.NoError(t, err)

	s := &Store{sysCfg: f}
	server, err := s.GetMetadataServer()
	require.NoError(t, err)
	assert.Equal(t, "https://metadata.example.com", server)
}

func TestGetMetadataServerFromFile(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "settings.ini")
	err := os.WriteFile(confPath, []byte("[General]\nServer = https://file.example.com\n"), 0644)
	require.NoError(t, err)

	f, err := ini.Load(confPath)
	require.NoError(t, err)

	s := &Store{sysCfg: f}
	server, err := s.GetMetadataServer()
	require.NoError(t, err)
	assert.Equal(t, "https://file.example.com", server)
}

func TestGetPackageApplicationNoServer(t *testing.T) {
	s := &Store{sysCfg: nil}
	v, err := s.GetPackageApplication(filepath.Join(t.TempDir(), "packages"))
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestGetPackageApplication(t *testing.T) {
	apps := packageApps{
		"dpk://deb/com.example.graphics": {
			Name:     "Graphics",
			Category: "graphics",
		},
		"dpk://deb/com.example.office": {
			Name:     "Office",
			Category: "office",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/public/packages", r.URL.Path)
		_ = json.NewEncoder(w).Encode(apps)
	}))
	defer srv.Close()

	f := ini.Empty()
	sec, err := f.NewSection("General")
	require.NoError(t, err)
	_, err = sec.NewKey("Server", srv.URL)
	require.NoError(t, err)

	s := &Store{sysCfg: f}
	path := filepath.Join(t.TempDir(), "packages")
	v, err := s.GetPackageApplication(path)
	require.NoError(t, err)
	require.Len(t, v, 2)

	byName := map[string]*PackageInfo{}
	for _, p := range v {
		byName[p.PackageName] = p
	}

	g, ok := byName["com.example.graphics"]
	require.True(t, ok)
	assert.Equal(t, "Graphics", g.Name)
	assert.Equal(t, "dpk://deb/com.example.graphics", g.PackageURI)

	o, ok := byName["com.example.office"]
	require.True(t, ok)
	assert.Equal(t, "Office", o.Name)
	assert.Equal(t, "dpk://deb/com.example.office", o.PackageURI)

	// The cache file should have been written next to path.
	_, err = os.Stat(path + ".cache.json")
	assert.NoError(t, err)
}

func TestCacheFetchJSONFreshCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")
	content := `{"dpk://deb/com.example.cached":{"name":"Cached","category":"utils"}}`
	err := os.WriteFile(cachePath, []byte(content), 0644)
	require.NoError(t, err)
	// Keep the cache file recent so it is considered fresh.
	now := time.Now()
	require.NoError(t, os.Chtimes(cachePath, now, now))

	var v packageApps
	// The URL is unreachable; a fresh cache must be used without hitting it.
	err = cacheFetchJSON(&v, "http://127.0.0.1:1", cachePath, expireDelay)
	require.NoError(t, err)
	require.Contains(t, v, "dpk://deb/com.example.cached")
	assert.Equal(t, "Cached", v["dpk://deb/com.example.cached"].Name)
}

func TestCacheFetchJSONFetchAndWrite(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(packageApps{
			"dpk://deb/com.example.fetched": {Name: "Fetched", Category: "utils"},
		})
	}))
	defer srv.Close()

	var v packageApps
	err := cacheFetchJSON(&v, srv.URL, cachePath, expireDelay)
	require.NoError(t, err)
	require.Contains(t, v, "dpk://deb/com.example.fetched")
	assert.Equal(t, "Fetched", v["dpk://deb/com.example.fetched"].Name)

	data, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "com.example.fetched")
}

func TestCacheFetchJSONNotModified(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")
	content := `{"dpk://deb/com.example.old":{"name":"Old","category":"utils"}}`
	err := os.WriteFile(cachePath, []byte(content), 0644)
	require.NoError(t, err)

	// Make the cache file expired.
	oldMtime := time.Now().Add(-2 * expireDelay)
	require.NoError(t, os.Chtimes(cachePath, oldMtime, oldMtime))

	// The server reports a Last-Modified older than the cached file,
	// so the cache is considered not-modified and the old file is decoded.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", oldMtime.Add(-time.Hour).Format(time.RFC1123))
		_ = json.NewEncoder(w).Encode(packageApps{
			"dpk://deb/com.example.new": {Name: "New", Category: "utils"},
		})
	}))
	defer srv.Close()

	var v packageApps
	err = cacheFetchJSON(&v, srv.URL, cachePath, expireDelay)
	require.NoError(t, err)
	require.Contains(t, v, "dpk://deb/com.example.old")
	assert.Equal(t, "Old", v["dpk://deb/com.example.old"].Name)
}

func TestCacheFetchJSONGzip(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_ = json.NewEncoder(gz).Encode(packageApps{
			"dpk://deb/com.example.gzip": {Name: "Gzip", Category: "utils"},
		})
		_ = gz.Close()
	}))
	defer srv.Close()

	var v packageApps
	err := cacheFetchJSON(&v, srv.URL, cachePath, expireDelay)
	require.NoError(t, err)
	require.Contains(t, v, "dpk://deb/com.example.gzip")
	assert.Equal(t, "Gzip", v["dpk://deb/com.example.gzip"].Name)
}

func TestCacheFetchJSONDecodeError(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()

	var v packageApps
	err := cacheFetchJSON(&v, srv.URL, cachePath, expireDelay)
	assert.Error(t, err)
}

func TestCacheFetchJSONHTTPError(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")

	var v packageApps
	err := cacheFetchJSON(&v, "http://127.0.0.1:1", cachePath, expireDelay)
	assert.Error(t, err)
}

func TestCacheFetchJSONGzipError(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "packages.cache.json")

	// Content-Encoding: gzip with a non-gzip body makes gzip.NewReader fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write([]byte("not gzip data"))
	}))
	defer srv.Close()

	var v packageApps
	err := cacheFetchJSON(&v, srv.URL, cachePath, expireDelay)
	assert.Error(t, err)
}

func TestCacheFetchJSONOpenFileError(t *testing.T) {
	// A cache path in a non-existent directory makes os.OpenFile fail after a
	// successful fetch; the function logs and returns nil.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(packageApps{
			"dpk://deb/com.example.fetched": {Name: "Fetched", Category: "utils"},
		})
	}))
	defer srv.Close()

	var v packageApps
	err := cacheFetchJSON(&v, srv.URL, filepath.Join(t.TempDir(), "no-such-dir", "pkg.cache.json"), expireDelay)
	assert.NoError(t, err)
	assert.Contains(t, v, "dpk://deb/com.example.fetched")
}

func TestGetPackageApplicationFetchError(t *testing.T) {
	// A metadata server pointing at an unreachable host makes cacheFetchJSON
	// fail, exercising the GetPackageApplication error return.
	f := ini.Empty()
	sec, err := f.NewSection("General")
	require.NoError(t, err)
	_, err = sec.NewKey("Server", "http://127.0.0.1:1")
	require.NoError(t, err)

	s := &Store{sysCfg: f}
	v, err := s.GetPackageApplication(filepath.Join(t.TempDir(), "packages"))
	assert.Error(t, err)
	assert.Nil(t, v)
}
