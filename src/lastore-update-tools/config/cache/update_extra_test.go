// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderCacheInfoWithUpdateMetaInfo(t *testing.T) {
	cc := &CacheConfig{}
	err := cc.LoaderCacheInfoWithUpdateMetaInfo("/tmp/test", "uuid-123", CacheInfo{})
	assert.NoError(t, err)
}

func TestCacheConfigLoaderInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("a: b\n\tc: [unclosed"), 0644))

	var cc CacheConfig
	err := cc.Loader(cfgPath)
	assert.Error(t, err)
}
