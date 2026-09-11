// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCacheWithLegacyFallbackPrimaryExists(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "primary.json")
	legacy := filepath.Join(dir, "legacy.json")
	primaryContent := []byte(`{"primary":true}`)
	require.NoError(t, os.WriteFile(primary, primaryContent, 0600))
	require.NoError(t, os.WriteFile(legacy, []byte(`{"legacy":true}`), 0600))

	content, err := readCacheWithLegacyFallback(primary, legacy)
	require.NoError(t, err)
	assert.Equal(t, primaryContent, content)
}

func TestReadCacheWithLegacyFallbackEmptyLegacyPath(t *testing.T) {
	dir := t.TempDir()
	content, err := readCacheWithLegacyFallback(filepath.Join(dir, "missing.json"), "")
	require.Error(t, err)
	assert.Nil(t, content)
}

func TestReadCacheWithLegacyFallbackPrimaryNotAFile(t *testing.T) {
	dir := t.TempDir()
	// os.ReadFile on a directory returns an error that is not os.ErrNotExist,
	// so the function must return that error instead of falling back to legacy.
	content, err := readCacheWithLegacyFallback(dir, filepath.Join(dir, "legacy.json"))
	require.Error(t, err)
	assert.Empty(t, content)
}
