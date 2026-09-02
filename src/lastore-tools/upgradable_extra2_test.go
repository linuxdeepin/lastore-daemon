// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSystemArchitectures_ReturnsNonEmpty(t *testing.T) {
	archs := getSystemArchitectures()
	// dpkg should be available in the test environment
	assert.NotEmpty(t, archs)
	// The first arch should be the primary architecture (e.g., amd64)
	assert.NotEmpty(t, archs[0])
}
