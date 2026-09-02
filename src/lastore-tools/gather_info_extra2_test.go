// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDiskSize_ReturnsResult(t *testing.T) {
	infos, err := getDiskSize()
	// lsblk should be available in the test environment
	if err != nil {
		t.Skipf("lsblk not available or failed: %v", err)
	}
	// Should return at least one disk
	assert.NotEmpty(t, infos)
	for _, info := range infos {
		assert.NotEmpty(t, info.DiskNo)
		assert.Greater(t, info.TotalCapacity, int64(0))
		assert.GreaterOrEqual(t, info.FreeCapacity, int64(0))
	}
}

func TestGetDiskSize_DiskInfoStruct(t *testing.T) {
	infos, err := getDiskSize()
	if err != nil {
		t.Skipf("lsblk not available or failed: %v", err)
	}
	for _, info := range infos {
		// FreeCapacity should not exceed TotalCapacity
		assert.LessOrEqual(t, info.FreeCapacity, info.TotalCapacity)
	}
}
