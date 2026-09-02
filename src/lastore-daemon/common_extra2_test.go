// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainsPathTraversalClean(t *testing.T) {
	result := containsPathTraversal("Unpacking foo (1.0) over (2.0) ...\nSetting up foo (1.0) ...\n")
	assert.False(t, result)
}

func TestContainsPathTraversalEmpty(t *testing.T) {
	result := containsPathTraversal("")
	assert.False(t, result)
}

func TestContainsPathTraversalAbsolute(t *testing.T) {
	output := "drwxr-xr-x root/root         0 2024-01-01 12:00 /usr/bin/foo"
	result := containsPathTraversal(output)
	assert.True(t, result)
}

func TestContainsPathTraversalTraversal(t *testing.T) {
	output := "some line with enough fields and ../../../etc/passwd"
	result := containsPathTraversal(output)
	assert.True(t, result)
}

func TestFormatSize(t *testing.T) {
	assert.Equal(t, "0B", formatSize(0))
	assert.Equal(t, "1023B", formatSize(1023))
	assert.Equal(t, "1.00KB", formatSize(1024))
	assert.Equal(t, "1.00MB", formatSize(1024*1024))
	assert.Equal(t, "1.00GB", formatSize(1024*1024*1024))
}

func TestFormatSizeTB(t *testing.T) {
	assert.Equal(t, "1.00TB", formatSize(1024*1024*1024*1024))
}

func TestGetContentSha256(t *testing.T) {
	result := getContentSha256("hello")
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", result)
}

func TestGetContentSha256Empty(t *testing.T) {
	result := getContentSha256("")
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", result)
}

func TestGetFileSha256Valid(t *testing.T) {
	tmpDir := t.TempDir()
	fp := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(fp, []byte("hello"), 0644))
	result, err := getFileSha256(fp)
	assert.NoError(t, err)
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", result)
}

func TestGetFileSha256EmptyPath(t *testing.T) {
	_, err := getFileSha256("")
	assert.Error(t, err)
}

func TestGetFileSha256NonExistent(t *testing.T) {
	_, err := getFileSha256("/nonexistent/path/file.txt")
	assert.Error(t, err)
}

func TestNewTimeRangeNormal(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	assert.Equal(t, start, tr.Start)
	assert.Equal(t, end, tr.End)
}

func TestNewTimeRangeSwapped(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	assert.Equal(t, end, tr.Start)
	assert.Equal(t, start, tr.End)
}

func TestTimeRangeContainsInside(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	mid := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	assert.True(t, tr.Contains(mid))
}

func TestTimeRangeContainsBoundary(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	assert.True(t, tr.Contains(start))
	assert.True(t, tr.Contains(end))
}

func TestTimeRangeContainsOutside(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	before := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	after := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)
	assert.False(t, tr.Contains(before))
	assert.False(t, tr.Contains(after))
}

func TestTimeRangeString(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	s := tr.String()
	assert.Contains(t, s, "~")
	assert.Contains(t, s, "2024-01-01T10:00:00Z")
	assert.Contains(t, s, "2024-01-01T12:00:00Z")
}
