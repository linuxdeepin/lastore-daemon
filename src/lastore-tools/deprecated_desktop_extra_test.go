// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDesktopIndexes(t *testing.T) {
	err := GenerateDesktopIndexes(t.TempDir())
	t.Logf("GenerateDesktopIndexes returned: %v", err)
}

func TestGenerateDesktopIndexesFull(t *testing.T) {
	oldBase := BaseDir
	t.Cleanup(func() { BaseDir = oldBase })

	base := t.TempDir()
	BaseDir = base
	u2dDir := filepath.Join(base, "override", "desktop2uaid")
	require.NoError(t, os.MkdirAll(u2dDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(u2dDir, "u2d.json"), []byte(`{"firefox.desktop":"uaid-firefox"}`), 0644))

	outDir := t.TempDir()
	assert.NoError(t, GenerateDesktopIndexes(outDir))

	// The desktop_package.json index should exist after a successful run.
	data, err := os.ReadFile(filepath.Join(outDir, "desktop_package.json"))
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}
