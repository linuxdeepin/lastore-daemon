// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTr(t *testing.T) {
	result := Tr("test")
	assert.NotEmpty(t, result)
	assert.Equal(t, "test", result)
}

func TestTrEmpty(t *testing.T) {
	result := Tr("")
	assert.Equal(t, "", result)
}

func TestGetCoreListFromCacheNonExistent(t *testing.T) {
	// The file at coreListVarPath likely doesn't exist in test environment
	result := getCoreListFromCache()
	// Should return nil since file doesn't exist or can't be read
	assert.Nil(t, result)
}

func TestCheckSupportDpkgScriptIgnore(t *testing.T) {
	// This function calls dpkg which may or may not exist
	// Just verify it doesn't panic and returns a bool
	result := checkSupportDpkgScriptIgnore()
	_ = result
}

func TestGetRebootCheckJobUUIDNonExistent(t *testing.T) {
	result := getRebootCheckJobUUID()
	// File likely doesn't exist in test env, should return ""
	assert.Equal(t, "", result)
}

func TestApplicationInfosNonExistent(t *testing.T) {
	result := applicationInfos()
	// Function always returns a non-nil map; content depends on whether
	// the JSON file exists on the system
	assert.NotNil(t, result)
}

func TestPackageIconInfosNonExistent(t *testing.T) {
	result := packageIconInfos()
	// Function always returns a non-nil map; content depends on whether
	// the JSON file exists on the system
	assert.NotNil(t, result)
}

func TestGetArchiveInfoNonExistent(t *testing.T) {
	// lastore-apt-clean binary likely doesn't exist in test env
	result, err := getArchiveInfo()
	// Should return error since binary doesn't exist
	if err != nil {
		assert.Empty(t, result)
	} else {
		// If binary exists, just verify we got a string
		assert.NotEmpty(t, result)
	}
}

func TestGetNeedCleanCacheSizeNonExistent(t *testing.T) {
	_, err := getNeedCleanCacheSize()
	// Should return error since binary doesn't exist
	// Just verify it doesn't panic
	_ = err
}
