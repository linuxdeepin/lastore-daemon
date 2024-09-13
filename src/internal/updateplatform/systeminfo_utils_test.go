// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemInfoUtil(t *testing.T) {
	sys := getSystemInfo(true)
	assert.NotEmpty(t, sys)
}
