// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package meta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMetaCfg(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "meta.json")
	ui := cache.UpdateInfo{UUID: "test-uuid", ApiVersion: "1.0"}
	data, err := json.Marshal(ui)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0644))

	var meta cache.CacheInfo
	err = LoadMetaCfg(cfgPath, &meta)
	require.NoError(t, err)
	assert.Equal(t, "test-uuid", meta.UpdateMetaInfo.UUID)
	assert.Equal(t, "1.0", meta.UpdateMetaInfo.ApiVersion)
}

func TestLoadMetaCfgNonExist(t *testing.T) {
	var meta cache.CacheInfo
	err := LoadMetaCfg("/nonexistent/meta.json", &meta)
	assert.Error(t, err)
}

func TestLoadMetaCfgInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte("invalid json"), 0644))

	var meta cache.CacheInfo
	err := LoadMetaCfg(cfgPath, &meta)
	assert.Error(t, err)
}
