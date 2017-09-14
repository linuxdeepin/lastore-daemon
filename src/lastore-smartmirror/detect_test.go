/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	C "gopkg.in/check.v1"
	"os"
	"testing"
)

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestDetect(c *C.C) {
	if os.Getenv("NO_TEST_NETWORK") == "1" {
		c.Skip("NO_TEST_NETWORK")
	}
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
			"http://cdn.packages.deepin.com/deepin/dists/unstable/Release",
			false,
		},
		{
			"http://localhost/abc",
			"http://cdn.packages.deepin.com/deepin/dists/unstable/Release",
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
