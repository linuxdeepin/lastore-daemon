// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ecode

import (
	"fmt"
	"testing"
)

func TestExtCodeMapping(t *testing.T) {
	t.Run("check retcode whether overflows", func(t *testing.T) {
		// var m int
		fmt.Printf("UPDATE_PKG_INSTALL_FAILED:%d\n", UPDATE_PKG_INSTALL_FAILED)
		fmt.Printf("UPDATE_RULES_CHECK_FAILED:%d\n", UPDATE_RULES_CHECK_FAILED)
		fmt.Printf("TEST_LEFT62:%d\n", 1<<62+1)
	})
}
