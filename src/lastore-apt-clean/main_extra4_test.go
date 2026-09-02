// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersionsGtFast_ValidVersions(t *testing.T) {
	gt, err := compareVersionsGtFast("2.0", "1.0")
	assert.NoError(t, err)
	assert.True(t, gt)
}

func TestCompareVersionsGtFast_EqualVersions(t *testing.T) {
	gt, err := compareVersionsGtFast("1.0", "1.0")
	assert.NoError(t, err)
	assert.False(t, gt)
}

func TestCompareVersionsGtFast_LessThan(t *testing.T) {
	gt, err := compareVersionsGtFast("1.0", "2.0")
	assert.NoError(t, err)
	assert.False(t, gt)
}

func TestCompareVersionsGtFast_InvalidVersion1(t *testing.T) {
	_, err := compareVersionsGtFast("invalid-version-!!!", "1.0")
	assert.Error(t, err)
}

func TestCompareVersionsGtFast_InvalidVersion2(t *testing.T) {
	_, err := compareVersionsGtFast("1.0", "invalid-version-!!!")
	assert.Error(t, err)
}

func TestCompareVersionsGt_WithFast(t *testing.T) {
	assert.True(t, compareVersionsGt("2.0", "1.0"))
	assert.False(t, compareVersionsGt("1.0", "2.0"))
	assert.False(t, compareVersionsGt("1.0", "1.0"))
}

func TestCompareVersionsGt_ComplexVersions(t *testing.T) {
	assert.True(t, compareVersionsGt("1.0.1-1", "1.0.0-1"))
	assert.True(t, compareVersionsGt("1:1.0", "1.0"))
	assert.False(t, compareVersionsGt("1.0-1", "1.0-2"))
}

func TestCompareVersionsGt_FallbackToDpkg(t *testing.T) {
	// When debVersion.Parse fails, compareVersionsGt falls back to dpkg --compare-versions
	// Just verify it doesn't panic and returns a bool
	assert.NotPanics(t, func() {
		_ = compareVersionsGt("\x00invalid", "1.0")
	})
}

func TestCompareVersionsGtFast_WithEpoch(t *testing.T) {
	gt, err := compareVersionsGtFast("2:1.0", "1:1.0")
	assert.NoError(t, err)
	assert.True(t, gt)
}

func TestCompareVersionsGtFast_WithRevision(t *testing.T) {
	gt, err := compareVersionsGtFast("1.0-2", "1.0-1")
	assert.NoError(t, err)
	assert.True(t, gt)
}
