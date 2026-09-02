// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoaderCacheInfoWithUpdateMetaInfo(t *testing.T) {
	cc := &CacheConfig{}
	err := cc.LoaderCacheInfoWithUpdateMetaInfo("/tmp/test", "uuid-123", CacheInfo{})
	assert.NoError(t, err)
}
