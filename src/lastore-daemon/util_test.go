package main

import "testing"
import C "gopkg.in/check.v1"

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
	}
	for _, d := range data {
		c.Check(parsePackageSize(d.Line), C.Equals, d.Size)
	}
}
