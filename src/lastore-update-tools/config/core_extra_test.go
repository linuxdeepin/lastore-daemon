// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestLoaderCfgPreservesMissingFileError(t *testing.T) {
	var cfg CoreConfig
	err := cfg.LoaderCfg(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
	var pathErr *os.PathError
	assert.True(t, errors.As(err, &pathErr))
}

func TestCoreConfigLoaderCfg(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "core.yaml")
	cc := &CoreConfig{Base: dir, CacheList: "cache.yaml", ApiVersion: "2.0"}
	data, err := yaml.Marshal(cc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0644))

	var loaded CoreConfig
	err = loaded.LoaderCfg(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "2.0", loaded.ApiVersion)
	assert.Equal(t, dir, loaded.Base)
}

func TestCoreConfigLoaderCfgNonExist(t *testing.T) {
	var cc CoreConfig
	err := cc.LoaderCfg("/nonexistent/core.yaml")
	assert.Error(t, err)
}

func TestCoreConfigUpdateCfg(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "core_out.yaml")
	cc := &CoreConfig{Base: dir, CacheList: "cache.yaml", DebugMode: true}

	err := cc.UpdateCfg(cfgPath)
	require.NoError(t, err)

	var loaded CoreConfig
	require.NoError(t, loaded.LoaderCfg(cfgPath))
	assert.True(t, loaded.DebugMode)
}

func TestCoreConfigLoaderCacheAndUpdateCache(t *testing.T) {
	dir := t.TempDir()
	cacheList := "test_cache.yaml"
	cachePath := filepath.Join(dir, cacheList)

	originalCC := &CoreConfig{Base: dir, CacheList: cacheList}
	originalCache := &cache.CacheConfig{
		Cache: map[string]cache.CacheInfo{
			"uuid-1": {UUID: "uuid-1", Status: "ok"},
		},
	}
	data, err := yaml.Marshal(originalCache)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0644))

	var loaded cache.CacheConfig
	err = originalCC.LoaderCache(&loaded)
	require.NoError(t, err)
	assert.Equal(t, "uuid-1", loaded.Cache["uuid-1"].UUID)

	err = originalCC.UpdateCache(&cache.CacheConfig{
		Cache: map[string]cache.CacheInfo{
			"uuid-2": {UUID: "uuid-2", Status: "run"},
		},
	})
	require.NoError(t, err)

	var loaded2 cache.CacheConfig
	require.NoError(t, originalCC.LoaderCache(&loaded2))
	assert.Equal(t, "uuid-2", loaded2.Cache["uuid-2"].UUID)
}

func TestCoreConfigLoaderCacheNonExist(t *testing.T) {
	dir := t.TempDir()
	cc := &CoreConfig{Base: dir, CacheList: "nonexist.yaml"}
	var cc2 cache.CacheConfig
	err := cc.LoaderCache(&cc2)
	assert.NoError(t, err) // creates file if not exists
}

func TestCoreConfigLoaderCacheReadError(t *testing.T) {
	dir := t.TempDir()
	// cacheFileName resolves to an existing directory, so ReadFile fails.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "cachedir"), 0755))
	cc := &CoreConfig{Base: dir, CacheList: "cachedir"}
	var cacheCfg cache.CacheConfig
	err := cc.LoaderCache(&cacheCfg)
	assert.Error(t, err)
}

func TestCoreConfigLoaderCfgCache(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "core.yaml")
	cacheList := "test_combined.yaml"

	cc := &CoreConfig{Base: dir, CacheList: cacheList, ApiVersion: "1.0"}
	data, err := yaml.Marshal(cc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0644))

	var cc2 CoreConfig
	var cacheCfg cache.CacheConfig
	err = cc2.LoaderCfgCache(cfgPath, &cacheCfg)
	require.NoError(t, err)
	assert.Equal(t, dir, cc2.Base)
}

func TestCoreConfigLoaderCfgInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("a: b\n\tc: [unclosed"), 0644))

	var cc CoreConfig
	err := cc.LoaderCfg(cfgPath)
	assert.Error(t, err)
}

func TestCoreConfigUpdateCfgWriteError(t *testing.T) {
	// writing to a directory path fails
	dir := t.TempDir()
	cc := &CoreConfig{Base: dir, CacheList: "cache.yaml"}
	err := cc.UpdateCfg(dir)
	assert.Error(t, err)
}

func TestCoreConfigLoaderCacheInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("a: b\n\tc: [unclosed"), 0644))
	cc := &CoreConfig{Base: dir, CacheList: "bad.yaml"}
	var cacheCfg cache.CacheConfig
	err := cc.LoaderCache(&cacheCfg)
	assert.Error(t, err)
}

func TestCoreConfigUpdateCacheWriteError(t *testing.T) {
	// Base points to a file, so Base+"/"+CacheList is not a valid writable path
	dir := t.TempDir()
	baseFile := filepath.Join(dir, "basefile")
	require.NoError(t, os.WriteFile(baseFile, []byte("x"), 0644))
	cc := &CoreConfig{Base: baseFile, CacheList: "cache.yaml"}
	err := cc.UpdateCache(&cache.CacheConfig{})
	assert.Error(t, err)
}

func TestCoreConfigLoaderCfgCacheCfgError(t *testing.T) {
	var cc CoreConfig
	var cacheCfg cache.CacheConfig
	err := cc.LoaderCfgCache("/nonexistent/core.yaml", &cacheCfg)
	assert.Error(t, err)
}

func TestCoreConfigLoaderCfgCacheCacheError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "core.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("Base: "+dir+"\nCacheList: bad.yaml\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("a: b\n\tc: [unclosed"), 0644))

	var cc CoreConfig
	var cacheCfg cache.CacheConfig
	err := cc.LoaderCfgCache(cfgPath, &cacheCfg)
	assert.Error(t, err)
}

func TestLoadOrCreateCfgMissing(t *testing.T) {
	for _, name := range []string{"config.yaml", "missing/nested/config.yaml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			cfg := CoreConfig{Base: "/stale", DebugMode: true, ApiVersion: "stale"}
			created, err := cfg.LoadOrCreateCfg(path)
			require.NoError(t, err)
			assert.True(t, created)
			expected := CoreConfig{CacheList: "caches.yaml", Base: "/var/lib/lastore/check/", DynHookTimeout: 1800}
			assert.Equal(t, expected, cfg)
			var loaded CoreConfig
			require.NoError(t, loaded.LoaderCfg(path))
			assert.Equal(t, expected, loaded)
			// Keep runtime defaults aligned with the configuration installed by packaging.
			data, err := os.ReadFile("../../../configs/config.yaml")
			require.NoError(t, err)
			var packaged CoreConfig
			require.NoError(t, yaml.Unmarshal(data, &packaged))
			assert.Equal(t, packaged, cfg)
		})
	}
}

func TestLoadOrCreateCfgPreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("# custom configuration\nBase: /custom\nDynHookTimeout: 42\nDebugMode: true\n")
	require.NoError(t, os.WriteFile(path, data, 0644))
	for i := 0; i < 2; i++ {
		var cfg CoreConfig
		created, err := cfg.LoadOrCreateCfg(path)
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, "/custom", cfg.Base)
		assert.Equal(t, 42, cfg.DynHookTimeout)
		assert.True(t, cfg.DebugMode)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	}
}

func TestLoadOrCreateCfgErrors(t *testing.T) {
	t.Run("invalid yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		data := []byte("Base: [invalid")
		require.NoError(t, os.WriteFile(path, data, 0644))
		var cfg CoreConfig
		created, err := cfg.LoadOrCreateCfg(path)
		require.Error(t, err)
		assert.False(t, created)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	})
	t.Run("directory", func(t *testing.T) {
		var cfg CoreConfig
		_, err := cfg.LoadOrCreateCfg(t.TempDir())
		require.Error(t, err)
	})
	t.Run("parent is file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "parent")
		require.NoError(t, os.WriteFile(parent, []byte("unchanged"), 0644))
		var cfg CoreConfig
		_, err := cfg.LoadOrCreateCfg(filepath.Join(parent, "config.yaml"))
		require.Error(t, err)
	})
	t.Run("permission denied", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
		var cfg CoreConfig
		_, err := cfg.LoadOrCreateCfg(filepath.Join(dir, "config.yaml"))
		require.Error(t, err)
	})
}

func TestLoaderCfgFillsImportantFields(t *testing.T) {
	for _, data := range []string{"{}", "Base: ''\nCacheList: ''\nDynHookTimeout: 0\n", "DynHookTimeout: -1\n"} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(data), 0644))
			cfg := CoreConfig{Base: "/stale", CacheList: "stale", DebugMode: true, ApiVersion: "stale", DynHookTimeout: 60}
			require.NoError(t, cfg.LoaderCfg(path))
			assert.Equal(t, defaultCoreConfig(), cfg)
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, data, string(got))
		})
	}
}

func TestLoaderCfgReloadAndParseFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("Base: /custom\nCacheList: custom.yaml\nDebugMode: true\nApiVersion: custom\nDynHookTimeout: 42\n"), 0644))
	var cfg CoreConfig
	require.NoError(t, cfg.LoaderCfg(path))
	assert.Equal(t, CoreConfig{Base: "/custom", CacheList: "custom.yaml", DebugMode: true, ApiVersion: "custom", DynHookTimeout: 42}, cfg)
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0644))
	require.NoError(t, cfg.LoaderCfg(path))
	assert.Equal(t, defaultCoreConfig(), cfg)
	before := cfg
	require.NoError(t, os.WriteFile(path, []byte("Base: /changed\nDynHookTimeout: invalid\n"), 0644))
	require.Error(t, cfg.LoaderCfg(path))
	assert.Equal(t, before, cfg)
}
