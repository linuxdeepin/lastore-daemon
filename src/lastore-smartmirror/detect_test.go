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
			"http://packages.linuxdeepin.com/favicon.ico",
			"http://notavaliddoamian.com/abc",
			true,
		},
		{
			"http://packages.linuxdeepin.com/favicon.ico",
			"http://cdn.packages.linuxdeepin.com/packages-debian/dists/unstable/Release",
			false,
		},
		{
			"http://notexit.com/abc",
			"http://cdn.packages.linuxdeepin.com/packages-debian/dists/unstable/Release",
			false,
		},
		{
			"http://localhost/abc",
			"http://localhost/xxxx",
			true,
		},
	}

	for _, item := range data {
		r := MakeChecker(item.Official, item.Mirror).Result()

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
