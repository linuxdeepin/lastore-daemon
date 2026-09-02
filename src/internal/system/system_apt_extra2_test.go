// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemArchitectures(t *testing.T) {
	archs, err := SystemArchitectures()
	require.NoError(t, err)
	assert.NotEmpty(t, archs, "should return at least one architecture")
}
