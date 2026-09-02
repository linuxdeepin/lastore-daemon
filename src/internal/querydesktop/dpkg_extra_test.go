// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package querydesktop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPkgsFilesEmpty(t *testing.T) {
	result := ListPkgsFiles(nil)
	assert.Nil(t, result, "ListPkgsFiles with empty pkgs should return nil")
}

func TestListPkgsFilesBash(t *testing.T) {
	result := ListPkgsFiles([]string{"bash"})
	// bash is installed, should return some files
	assert.NotNil(t, result, "ListPkgsFiles should return non-nil for installed package bash")
	assert.NotEmpty(t, result, "ListPkgsFiles should return non-empty list for bash")
}

func TestListPkgsFilesNonExistent(t *testing.T) {
	result := ListPkgsFiles([]string{"nonexistent-pkg-xyz123"})
	assert.Nil(t, result, "ListPkgsFiles should return nil for non-existent package")
}

func TestQuerySameSourcePkgsUnknown(t *testing.T) {
	// Before InitDB, maps are nil
	result := QuerySameSourcePkgs("unknown-pkg")
	assert.Nil(t, result, "QuerySameSourcePkgs should return nil for unknown package")
}

func TestQuerySameSourcePkgsAfterInitDB(t *testing.T) {
	InitDB()
	// After InitDB, try with bash which should be in the map
	// The result depends on dpkg-query output, just verify it doesn't panic
	result := QuerySameSourcePkgs("bash")
	_ = result
}

func TestListDesktopFilesEmptyPkg(t *testing.T) {
	result := ListDesktopFiles("nonexistent-pkg-xyz123")
	assert.Nil(t, result, "ListDesktopFiles should return nil for non-existent package")
}

func TestGroupBySource(t *testing.T) {
	s2b, b2s := groupBySource()
	// The function runs dpkg-query; on a system with packages installed it should return non-empty maps
	// Just verify it doesn't panic
	_ = s2b
	_ = b2s
}
