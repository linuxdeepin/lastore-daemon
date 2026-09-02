// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPackageName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pkg.list", "pkg"},
		{"pkg:amd64.list", "pkg"},
		{"short", "short"},
		{"a.list", "a"},
		{"abc.list", "abc"},
	}
	for _, tt := range tests {
		got := getPackageName(tt.input)
		assert.Equal(t, tt.want, got, "input=%s", tt.input)
	}
}

func TestMergeDesktopIndex(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "index.json")

	infos := map[string]string{
		"app1.desktop": "pkg1",
		"app2.desktop": "pkg2",
	}
	result := mergeDesktopIndex(infos, fpath)
	assert.Equal(t, "pkg1", result["app1.desktop"])
	assert.Equal(t, "pkg2", result["app2.desktop"])

	// Read back and merge again
	infos2 := map[string]string{
		"app3.desktop": "pkg3",
	}
	result2 := mergeDesktopIndex(infos2, fpath)
	assert.Equal(t, "pkg1", result2["app1.desktop"])
	assert.Equal(t, "pkg3", result2["app3.desktop"])
}

func TestMergeDesktopIndexNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "nonexistent.json")

	infos := map[string]string{
		"app1.desktop": "pkg1",
	}
	result := mergeDesktopIndex(infos, fpath)
	assert.Equal(t, "pkg1", result["app1.desktop"])

	_, err := os.Stat(fpath)
	assert.NoError(t, err)
}

func TestMergeDesktopIndexEmptyValues(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "index.json")

	infos := map[string]string{
		"":      "pkg1",
		"app1":  "",
		"app2":  "pkg2",
	}
	result := mergeDesktopIndex(infos, fpath)
	_, hasEmpty := result[""]
	assert.False(t, hasEmpty)
	_, hasEmptyVal := result["app1"]
	assert.False(t, hasEmptyVal)
	assert.Equal(t, "pkg2", result["app2"])
}
