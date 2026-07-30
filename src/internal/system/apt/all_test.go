// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	C "gopkg.in/check.v1"
)

type testWrap struct{}

func TestSystemAptAll(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestParseInfo(c *C.C) {
	line := "dummy:" + system.RunningStatus + ":1:" + "running"
	info, err := parseProgressInfo("jobid", string(line))
	c.Check(err, C.Equals, nil)
	c.Check(info.Status, C.Equals, system.RunningStatus)
	c.Check(info.JobId, C.Equals, "jobid")
}

func (*testWrap) TestValidatePackageNames(c *C.C) {
	// valid package names per dpkg pkg_name_is_illegal() strict rules
	validCases := []string{
		"vim",
		"g++",
		"libstdc++6",
		"deepin-desktop-environment-core",
		"qterminal-data",
		"0ad",
		"libfoo.bar.baz",
		"libc6:amd64",
		"libc6:AMD64",
		"vim=2:8.1.2269-1+deb10u1",
		"libc6=2.31-13+deb11u4~deb10u1",
		"linux-image-amd64:amd64=5.10.0-1~bpo10+1",
	}
	for _, pkg := range validCases {
		err := validatePackageNames([]string{pkg})
		c.Check(err, C.IsNil, C.Commentf("expected %q to be valid", pkg))
	}

	// invalid package names that could be interpreted as apt-get options
	// or violate dpkg naming policy
	invalidCases := []string{
		"--allow-unauthenticated",
		"-y",
		"--yes",
		"-o",
		"--option",
		"--allow-downgrades",
		"",
		" ",
		"-",
		"--",
		"pkg name", // contains space
		"pkg$HOME", // contains shell metachar
		"pkg;rm",   // contains shell metachar
		"pkg\nmal", // contains newline
		"PKG",      // uppercase not allowed by dpkg strict rules
		"pkg_underscore",
		".leadingdot",
	}
	for _, pkg := range invalidCases {
		err := validatePackageNames([]string{pkg})
		c.Check(err, C.NotNil, C.Commentf("expected %q to be invalid", pkg))
	}

	// empty slice is valid
	c.Check(validatePackageNames(nil), C.IsNil)
	c.Check(validatePackageNames([]string{}), C.IsNil)

	// mixed valid and invalid
	err := validatePackageNames([]string{"vim", "--allow-unauthenticated"})
	c.Check(err, C.NotNil)

	// multiple valid packages
	c.Check(validatePackageNames([]string{"vim", "git", "curl"}), C.IsNil)
}
