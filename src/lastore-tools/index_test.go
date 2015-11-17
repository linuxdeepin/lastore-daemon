package main

import "testing"
import C "gopkg.in/check.v1"

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestNormalizePackageName(c *C.C) {
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
