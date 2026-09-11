// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFakeImmutableCtl(t *testing.T, output string) string {
	t.Helper()
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(output) + "\n"
	path := filepath.Join(t.TempDir(), "deepin-immutable-ctl")
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path
}

func shellQuote(s string) string {
	return "'" + s + "'"
}

func TestIsIncrementalUpdateCachedZeroCount(t *testing.T) {
	old := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = newFakeImmutableCtl(t, "Need download count: 0\n")
	t.Cleanup(func() { DeepinImmutableCtlPath = old })

	assert.True(t, IsIncrementalUpdateCached(""))
}

func TestIsIncrementalUpdateCachedNonZeroCount(t *testing.T) {
	old := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = newFakeImmutableCtl(t, "Need download count: 5\n")
	t.Cleanup(func() { DeepinImmutableCtlPath = old })

	assert.False(t, IsIncrementalUpdateCached(""))
}

func TestIsIncrementalUpdateCachedNoMatch(t *testing.T) {
	old := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = newFakeImmutableCtl(t, "all cached\n")
	t.Cleanup(func() { DeepinImmutableCtlPath = old })

	assert.False(t, IsIncrementalUpdateCached(""))
}

func TestIsIncrementalUpdateCachedSourceArgsEnv(t *testing.T) {
	old := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = newFakeImmutableCtl(t, "Need download count: 0\n")
	t.Cleanup(func() { DeepinImmutableCtlPath = old })

	assert.True(t, IsIncrementalUpdateCached("-o foo=bar"))
}
