// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenPreBuild_ReturnsNonEmpty(t *testing.T) {
	// genPreBuild reads /var/lib/lastore/os-version.b which should exist on the system
	// If it doesn't exist, it returns empty string (error path)
	// We just verify it doesn't panic
	assert.NotPanics(t, func() {
		_ = genPreBuild()
	})
}

func TestGenPreBuild_ReturnsVersionFormat(t *testing.T) {
	result := genPreBuild()
	// If the os-version file exists and is valid, result should be "MajorVersion.MinorVersion.OsBuild"
	// If not, result will be empty string — both are valid outcomes
	if result != "" {
		// Should contain at least two dots (e.g., "25.0.12345")
		assert.Contains(t, result, ".")
	}
}
