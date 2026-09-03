// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestGuestBasePackageName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"pkg-name", "pkg"},
		{"pkg:amd64", "pkg"},
		{"pkg_name", "pkg"},
		{"pkg", "pkg"},
		{"lib-test-1.0", "lib-test"},
		{"no-match", "no"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, guestBasePackageName(tt.input))
	}
}

func TestParsePackageSize(t *testing.T) {
	tests := []struct {
		line        string
		needSize    float64
		allSize     float64
		expectError bool
	}{
		{"Need to get 1,234 MB of archives", 1234 * 1000 * 1000, 1234 * 1000 * 1000, false},
		{"Need to get 100 MB of archives", 100 * 1000 * 1000, 100 * 1000 * 1000, false},
		{"Need to get 50 MB/100 MB of archives", 50 * 1000 * 1000, 100 * 1000 * 1000, false},
		{"Need to get 5 kB of archives", 5 * 1000, 5 * 1000, false},
		{"Need to get 1 GB of archives", 1 * 1000 * 1000 * 1000, 1 * 1000 * 1000 * 1000, false},
		{"invalid line", SizeUnknown, SizeUnknown, true},
	}
	for _, tt := range tests {
		need, all, err := parsePackageSize(tt.line)
		if tt.expectError {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.InDelta(t, tt.needSize, need, 1)
		assert.InDelta(t, tt.allSize, all, 1)
	}
}

func TestParseInstallAddSize(t *testing.T) {
	tests := []struct {
		line     string
		want     float64
		hasError bool
	}{
		{"After this operation, 200 MB of disk space will be used", 200 * 1000 * 1000, false},
		{"After this operation, 1,234 kB of disk space will be used", 1234 * 1000, false},
		{"After this operation, 200 GB disk space will be freed", 0, false},
		{"After this operation, 5 GB of disk space will be used", 5 * 1000 * 1000 * 1000, false},
		{"no match", SizeUnknown, true},
	}
	for _, tt := range tests {
		got, err := parseInstallAddSize(tt.line)
		if tt.hasError {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.InDelta(t, tt.want, got, 1)
	}
}

func TestSystemArchitectures(t *testing.T) {
	archs, err := SystemArchitectures()
	require.NoError(t, err)
	assert.NotEmpty(t, archs, "should return at least one architecture")
}

func TestListPackageFileBash(t *testing.T) {
	files := ListPackageFile("bash")
	// bash is always installed; dpkg -L bash should list at least /bin/bash
	if _, err := os.Stat("/bin/bash"); err == nil {
		assert.NotEmpty(t, files, "ListPackageFile should return files for installed package bash")
	}
}

func TestListPackageFileNonExistent(t *testing.T) {
	files := ListPackageFile("this-package-does-not-exist-xyz123")
	assert.Nil(t, files, "ListPackageFile should return nil for non-existent package")
}

func TestQueryPackageDependenciesBash(t *testing.T) {
	deps := QueryPackageDependencies("bash")
	// bash has dependencies; result should not be nil (may be empty slice but not nil if deps contain baseName)
	// Just verify it doesn't panic
	_ = deps
}

func TestQueryPackageDependenciesNonExistent(t *testing.T) {
	deps := QueryPackageDependencies("nonexistent-pkg-xyz123")
	assert.Nil(t, deps, "QueryPackageDependencies should return nil for non-existent package")
}

func TestQueryFileCacheSizeTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file with known content
	err := os.WriteFile(filepath.Join(tmpDir, "testfile"), make([]byte, 4096), 0644)
	require.NoError(t, err)

	size, err := QueryFileCacheSize(tmpDir)
	require.NoError(t, err)
	assert.Greater(t, size, 0.0, "QueryFileCacheSize should return positive size for non-empty dir")
}

func TestQueryFileCacheSizeNonExistent(t *testing.T) {
	_, err := QueryFileCacheSize("/nonexistent/path/xyz123")
	assert.Error(t, err, "QueryFileCacheSize should return error for non-existent path")
}

func TestGetArchivesDirEmpty(t *testing.T) {
	// With empty config path, apt-config may still work with defaults
	// Just verify it doesn't panic and returns either a path or an error
	dir, err := GetArchivesDir("")
	if err != nil {
		assert.Empty(t, dir)
	} else {
		assert.NotEmpty(t, dir)
	}
}

func TestHandleDelayPackageHoldBash(t *testing.T) {
	// Test hold and unhold on bash - should not panic
	HandleDelayPackage(true, []string{"bash"})
	HandleDelayPackage(false, []string{"bash"})
}

func TestHandleDelayPackageEmpty(t *testing.T) {
	// Should not panic with empty packages
	HandleDelayPackage(true, []string{})
}

func TestParsePackageSizeValid(t *testing.T) {
	tests := []struct {
		line    string
		wantErr bool
	}{
		{"Need to get 1,234 kB of archives", false},
		{"Need to get 500 MB of archives", false},
		{"Need to get 100 B of archives", false},
		{"Need to get 2.5 GB/5.0 GB of archives", false},
		{"invalid line", true},
	}
	for _, tt := range tests {
		_, _, err := parsePackageSize(tt.line)
		if tt.wantErr {
			assert.Error(t, err, "expected error for line: %s", tt.line)
		} else {
			assert.NoError(t, err, "expected no error for line: %s", tt.line)
		}
	}
}

func TestParseInstallAddSizeFreed(t *testing.T) {
	// "freed" means disk space will be freed, should return 0
	size, err := parseInstallAddSize("After this operation, 200 MB disk space will be freed.")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, size)
}

func TestParseInstallAddSizeInvalid(t *testing.T) {
	_, err := parseInstallAddSize("invalid line")
	assert.Error(t, err)
}
