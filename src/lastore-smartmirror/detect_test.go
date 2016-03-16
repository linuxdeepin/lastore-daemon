/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

import "testing"
import C "gopkg.in/check.v1"

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestDetect(c *C.C) {
	data := []struct {
		Official   string
		Mirror     string
		IsOfficial bool
	}{
		{
			"http://packages.deepin.com/favicon.ico",
			"http://localhost/abc",
			true,
		},
		{
			"http://packages.deepin.com/favicon.ico",
			"http://cdn.packages.deepin.com/packages-debian/dists/unstable/Release",
			false,
		},
		{
			"http://localhost/abc",
			"http://cdn.packages.deepin.com/packages-debian/dists/unstable/Release",
			false,
		},
		{
			"http://localhost/abc",
			"http://localhost/xxxx",
			true,
		},
	}

	for _, item := range data {
		r := MakeChoice(item.Official, item.Mirror)

		var expect string
		if item.IsOfficial {
			expect = item.Official
		} else {
			expect = item.Mirror
		}

		if !c.Check(r, C.Equals, expect) {
			c.Fatalf("Failed when %q,%q\n", item.Official, item.Mirror)
		}
	}
}
