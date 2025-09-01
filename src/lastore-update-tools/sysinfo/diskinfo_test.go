// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"fmt"
	"testing"
)

func TestGetRootDiskFreeSpace(t *testing.T) {
	t.Run("validRootDisk", func(t *testing.T) {
		if _, err := GetRootDiskFreeSpace(); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		rootFree, _ := GetRootDiskFreeSpace()
		fmt.Printf("%d", rootFree)
	})
}

func TestGetDataDiskFreeSpace(t *testing.T) {
	t.Run("validDataDisk", func(t *testing.T) {
		if m, err := GetDataDiskFreeSpace(); err != nil {
			t.Errorf("expected nil error, got %v %d", err, m)
		}
		dataFree, _ := GetDataDiskFreeSpace()
		fmt.Printf("%d", dataFree)
	})
}
