// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/base64"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/codegangsta/cli"
	config "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostUpgrade(t *testing.T) {
	// Empty data: no cache file, so the loop is skipped and the cache is removed.
	assert.NoError(t, postUpgrade(""))
}

func TestMainPostUpgrade(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	c := cli.NewContext(&cli.App{}, set, nil)
	assert.NoError(t, MainPostUpgrade(c))
}

func TestMainPostUpgradeWithArg(t *testing.T) {
	oldCache := postUpgradeCacheFile
	t.Cleanup(func() { postUpgradeCacheFile = oldCache })
	// Point the cache at a path whose parent is a regular file so the cache
	// write fails deterministically, independent of whether the test runs as
	// root.
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0600))
	postUpgradeCacheFile = filepath.Join(blocker, "postupgrade.cache")

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"not-base64"}))
	c := cli.NewContext(&cli.App{}, set, nil)

	// The invalid base64 payload fails to decode, so postUpgrade rewrites the
	// cache; the unwritable cache path surfaces as an error.
	assert.Error(t, MainPostUpgrade(c))
}

func TestPostUpgradeWithData(t *testing.T) {
	oldCache := postUpgradeCacheFile
	t.Cleanup(func() { postUpgradeCacheFile = oldCache })
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0600))
	postUpgradeCacheFile = filepath.Join(blocker, "postupgrade.cache")

	// Same decode-failure path as MainPostUpgrade, exercised directly.
	assert.Error(t, postUpgrade("not-base64"))
}

func TestPostUpgradeValidData(t *testing.T) {
	old := postUpgradeNewConfig
	t.Cleanup(func() { postUpgradeNewConfig = old })
	// Empty PlatformUrl makes the HTTP POST fail fast ("unsupported protocol
	// scheme"), avoiding any real network; the root-owned cache write also fails.
	postUpgradeNewConfig = func(string) *config.Config { return &config.Config{} }

	payload := base64.StdEncoding.EncodeToString([]byte(`{"serialNumber":"sn123"}`))
	assert.Error(t, postUpgrade(payload))
}

func TestPostUpgradeReadsCache(t *testing.T) {
	oldCfg := postUpgradeNewConfig
	oldCache := postUpgradeCacheFile
	t.Cleanup(func() {
		postUpgradeNewConfig = oldCfg
		postUpgradeCacheFile = oldCache
	})
	postUpgradeNewConfig = func(string) *config.Config { return &config.Config{} }
	postUpgradeCacheFile = filepath.Join(t.TempDir(), "postupgrade.cache")

	payload := base64.StdEncoding.EncodeToString([]byte(`{"serialNumber":"cached"}`))
	require.NoError(t, os.WriteFile(postUpgradeCacheFile, []byte("upmsg v1.0\n"+payload+"\n"), 0600))

	// Empty data: the cache supplies the payload; the empty platform URL makes
	// the POST fail (unsupported protocol scheme), which surfaces as the
	// returned error even though the temp cache is rewritten successfully.
	assert.Error(t, postUpgrade(""))
}

func TestPostUpgradeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/update/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	oldCfg := postUpgradeNewConfig
	oldCache := postUpgradeCacheFile
	t.Cleanup(func() {
		postUpgradeNewConfig = oldCfg
		postUpgradeCacheFile = oldCache
	})
	postUpgradeNewConfig = func(string) *config.Config { return &config.Config{PlatformUrl: server.URL} }
	postUpgradeCacheFile = filepath.Join(t.TempDir(), "postupgrade.cache")

	payload := base64.StdEncoding.EncodeToString([]byte(`{"serialNumber":"sn123"}`))
	assert.NoError(t, postUpgrade(payload))
}
