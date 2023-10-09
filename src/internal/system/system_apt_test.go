// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	C "gopkg.in/check.v1"
)

type testWrap struct{}

func TestSystemApt(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestPackageDownloadSize(c *C.C) {
	// TODO: using debootstrap to build test environment
	c.Skip("TODO: using debootstrap to build test environment")

	var packages = []string{"abiword", "0ad", "acl2"}
	for _, p := range packages {
		if QueryPackageInstalled(p) {
			s, _, err := QueryPackageDownloadSize(AllCheckUpdate, p)
			c.Check(err, C.Equals, nil)
			c.Check(s, C.Equals, float64(0))
		} else {
			s, _, err := QueryPackageDownloadSize(AllCheckUpdate, p)
			c.Check(err, C.Equals, nil)
			c.Check(s >= 0, C.Equals, true)
		}
	}
}

func (*testWrap) TestQueryPackageDepend(c *C.C) {
	p := "firefox-dde"
	if !QueryPackageInstalled(p) {
		return
	}
	ds := QueryPackageDependencies(p)

	c.Check(len(ds) > 0, C.Equals, true)

	found := false
	for _, i := range ds {
		if i == "firefox" {
			found = true
			break
		}
	}
	c.Check(found, C.Equals, true)
}

func (*testWrap) TestParseSize(c *C.C) {
	data := []struct {
		Line string
		Size float64
	}{
		{`Need to get 0 B of archives.`, 0},
		{`Need to get 0 B/3,792 kB of archives.`, 0},
		{`Need to get 1,33 MB/3,792 kB of archives.`, 133 * 1000 * 1000},
		{`Need to get 3,985 kB/26.2 MB of archives.`, 3985 * 1000},
		{`Need to get 9,401 kB of archives.`, 9401 * 1000},
		{`Need to get 13.7 MB of archives.`, 13.7 * 1000 * 1000},
	}
	for _, d := range data {
		s, _, err := parsePackageSize(d.Line)
		c.Check(err, C.Equals, nil)
		c.Check(s, C.Equals, d.Size)
	}
}
