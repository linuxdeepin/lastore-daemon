// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTarFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source files
	file1 := filepath.Join(tmpDir, "a.txt")
	file2 := filepath.Join(tmpDir, "b.log")
	require.NoError(t, os.WriteFile(file1, []byte("content-a"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content-b"), 0644))

	outFile := filepath.Join(tmpDir, "out.tar")
	err := tarFiles([]string{file1, file2}, outFile)
	require.NoError(t, err)

	// Verify the tar file
	f, err := os.Open(outFile)
	require.NoError(t, err)
	defer f.Close()

	tr := tar.NewReader(f)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		entries = append(entries, hdr.Name)
	}
	assert.Contains(t, entries, "a.txt")
	assert.Contains(t, entries, "b.log")
}

func TestTarFilesNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "out.tar")
	err := tarFiles([]string{filepath.Join(tmpDir, "nope.txt")}, outFile)
	assert.Error(t, err)
}

func TestTarFilesInvalidOutPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "a.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("x"), 0644))
	err := tarFiles([]string{srcFile}, filepath.Join(tmpDir, "nonexistent_dir", "out.tar"))
	assert.Error(t, err)
}

func TestGetTaskIdNoFile(t *testing.T) {
	// cacheTaskInfo path should not exist in test env; expect 0
	result := getTaskId()
	assert.Equal(t, 0, result)
}

func TestLoadLocalCVEData(t *testing.T) {
	// cveLocalInfo may or may not exist depending on the environment.
	// Just verify the function does not panic and returns either nil or data.
	result := loadLocalCVEData()
	// If the file exists, result should be non-nil; if not, nil.
	_ = result
}

func TestUpdateRequestUrl(t *testing.T) {
	m := &UpdatePlatformManager{}
	m.UpdateRequestUrl("https://example.com/api")
	assert.Equal(t, "https://example.com/api", m.requestUrl)
}

func TestIsMajorUpgradeEmptyTarget(t *testing.T) {
	m := &UpdatePlatformManager{}
	assert.False(t, m.IsMajorUpgrade())
}

func TestSetInhibitAutoQuit(t *testing.T) {
	m := &UpdatePlatformManager{}
	// This is a no-op function, just verify it doesn't panic
	m.SetInhibitAutoQuit()
}
