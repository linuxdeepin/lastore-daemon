// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

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
