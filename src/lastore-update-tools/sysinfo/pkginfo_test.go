// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"fmt"
	"testing"
)

// ToDo:(DingHao)替换成袁老师的hash函数
func TestGetSysPkgStateAndVersion(t *testing.T) {
	t.Run("valid-DPKG", func(t *testing.T) {
		if _, _, err := GetSysPkgStateAndVersion("dpkg"); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		state, version, _ := GetSysPkgStateAndVersion("dpkg")
		fmt.Printf("%s %s", state, version)
	})

	t.Run("valid-APT", func(t *testing.T) {
		if _, _, err := GetSysPkgStateAndVersion("apt"); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		state, version, _ := GetSysPkgStateAndVersion("apt")
		fmt.Printf("%s %s", state, version)
	})
}
