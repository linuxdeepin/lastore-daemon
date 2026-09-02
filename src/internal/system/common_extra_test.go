// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEditionName(t *testing.T) {
	// /etc/os-version should exist and be readable in the test environment
	edition, err := getEditionName()
	assert.NoError(t, err)
	assert.NotEmpty(t, edition)
}
