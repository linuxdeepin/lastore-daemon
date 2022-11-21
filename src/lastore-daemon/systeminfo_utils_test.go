// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemInfoUtil(t *testing.T) {
	useDbus := NotUseDBus
	NotUseDBus = true
	defer func() {
		NotUseDBus = useDbus
	}()
	sys := getSystemInfo()
	assert.NotEmpty(t, sys)
}
