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
		Official string
		Mirror   string
		Result   Hit
	}{
		{"http://packages.linuxdeepin.com/favicon.ico", "http://notavaliddoamian.com/abc", OfficialHit},
		{"http://packages.linuxdeepin.com/favicon.ico", "http://cdn.packages.linuxdeepin.com/packages-debian/dists/unstable/Release", MirrorHit},
		{"http://notexit.com/abc", "http://cdn.packages.linuxdeepin.com/packages-debian/dists/unstable/Release", MirrorHit},
		{"http://localhost/abc", "http://localhost/xxxx", NotFoundHit},
	}

	for _, item := range data {
		if !c.Check(SmartMirrorDetector(item.Official, item.Mirror), C.Equals, item.Result) {
			c.Fatalf("Failed when %q,%q\n", item.Official, item.Mirror)
		}
	}
}
