package main

import "testing"
import C "gopkg.in/check.v1"
import "internal/system/apt"

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestParseSize(c *C.C) {
	data := []struct {
		Line string
		Size float64
	}{
		{`Need to get 0 B of archives.`, 0},
		{`Need to get 0 B/3,792 kB of archives.`, 0},
		{`Need to get 1,33 MB/3,792 kB of archives.`, 133 * 1000 * 1000},
		{`Need to get 9,401 kB of archives.`, 9401 * 1000},
		{`Need to get 13.7 MB of archives.`, 13.7 * 1000 * 1000},
	}
	for _, d := range data {
		c.Check(parsePackageSize(d.Line), C.Equals, d.Size)
	}
}

func (*testWrap) TestLoadSourceMirrors(c *C.C) {
	c.Check(LoadMirrorSources("http://api.lastore.deepin.org"), C.Not(C.Equals), 0)
}

func (*testWrap) TestPackageDownloadSize(c *C.C) {
	s := apt.New()
	var packages = []string{"abiword", "0ad", "acl2"}
	for _, p := range packages {
		if s.CheckInstalled(p) {
			c.Check(GuestPackageDownloadSize(p), C.Equals, 0)
		} else {
			c.Check(GuestPackageDownloadSize(p), C.Not(C.Equals), 0)
		}
	}
}
