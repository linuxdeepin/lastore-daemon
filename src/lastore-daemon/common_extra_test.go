// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplicationInfosExtra(t *testing.T) {
	// applications.json typically doesn't exist at /var/lib/lastore/applications.json
	// Should return empty map without panic
	result := applicationInfos()
	assert.NotNil(t, result, "applicationInfos should return non-nil map")
}

func TestPackageIconInfos(t *testing.T) {
	// package_icon.json typically doesn't exist
	// Should return empty map without panic
	result := packageIconInfos()
	assert.NotNil(t, result, "packageIconInfos should return non-nil map")
}

func TestGetArchiveInfoNonExistentBinary(t *testing.T) {
	// /usr/bin/lastore-apt-clean may not be installed
	_, err := getArchiveInfo()
	// Just verify it doesn't panic
	_ = err
}

func TestGetNeedCleanCacheSize(t *testing.T) {
	_, err := getNeedCleanCacheSize()
	// Just verify it doesn't panic
	_ = err
}

func TestListPackageDesktopFilesBash(t *testing.T) {
	// bash is installed; listPackageDesktopFiles should return desktop files if any
	result := listPackageDesktopFiles("bash")
	// bash may or may not have desktop files; just verify it doesn't panic
	_ = result
}

func TestListPackageDesktopFilesNonExistent(t *testing.T) {
	result := listPackageDesktopFiles("nonexistent-pkg-xyz123")
	assert.Nil(t, result, "listPackageDesktopFiles should return nil for non-existent package")
}
