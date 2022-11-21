// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	C "gopkg.in/check.v1"
)

type testWrap struct{}

func TestIndex(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) Test_getPackageName(c *C.C) {
	var data = []struct {
		FileName    string
		PackageName string
	}{
		{"libfortran3:amd64.list", "libfortran3"},
	}
	for _, item := range data {
		c.Check(getPackageName(item.FileName), C.Equals, item.PackageName)
	}
}
