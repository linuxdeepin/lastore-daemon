// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils/fixme/pkg_recommend"
	"strings"
	"testing"

	C "gopkg.in/check.v1"
)

type testWrap struct{}

func TestUtil(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
	NotUseDBus = true
}

func (*testWrap) TestTransition(c *C.C) {
	var data = []struct {
		from  system.Status
		to    system.Status
		valid bool
	}{
		{system.ReadyStatus, system.ReadyStatus, false},
		{system.ReadyStatus, system.RunningStatus, true},
		{system.ReadyStatus, system.FailedStatus, true},
		{system.ReadyStatus, system.SucceedStatus, false},
		{system.ReadyStatus, system.PausedStatus, true},
		{system.ReadyStatus, system.EndStatus, true},

		{system.RunningStatus, system.ReadyStatus, false},
		{system.RunningStatus, system.RunningStatus, false},
		{system.RunningStatus, system.FailedStatus, true},
		{system.RunningStatus, system.SucceedStatus, true},
		{system.RunningStatus, system.PausedStatus, true},
		{system.RunningStatus, system.EndStatus, false},

		{system.FailedStatus, system.ReadyStatus, true},
		{system.FailedStatus, system.RunningStatus, false},
		{system.FailedStatus, system.FailedStatus, false},
		{system.FailedStatus, system.SucceedStatus, false},
		{system.FailedStatus, system.PausedStatus, false},
		{system.FailedStatus, system.EndStatus, true},

		{system.SucceedStatus, system.ReadyStatus, false},
		{system.SucceedStatus, system.RunningStatus, false},
		{system.SucceedStatus, system.FailedStatus, false},
		{system.SucceedStatus, system.SucceedStatus, false},
		{system.SucceedStatus, system.PausedStatus, false},
		{system.SucceedStatus, system.EndStatus, true},

		{system.PausedStatus, system.ReadyStatus, true},
		{system.PausedStatus, system.RunningStatus, false},
		{system.PausedStatus, system.FailedStatus, false},
		{system.PausedStatus, system.SucceedStatus, false},
		{system.PausedStatus, system.PausedStatus, false},
		{system.PausedStatus, system.EndStatus, true},

		{system.EndStatus, system.ReadyStatus, false},
		{system.EndStatus, system.RunningStatus, false},
		{system.EndStatus, system.FailedStatus, false},
		{system.EndStatus, system.SucceedStatus, false},
		{system.EndStatus, system.PausedStatus, false},
		{system.EndStatus, system.EndStatus, false},
	}

	for _, d := range data {
		if !c.Check(ValidTransitionJobState(d.from, d.to), C.Equals, d.valid) {
			c.Logf("Transition %s to %s failed (%v)\n", d.from, d.to, d.valid)
		}
	}
}

func (*testWrap) TestGetEnhancedLocalePackages(c *C.C) {
	if !system.QueryPackageInstalled("deepin-desktop-base") {
		c.Skip("deepin-desktop-base not installed")
		return
	}
	lang := "zh_CN.UTF-8"

	positive := []string{"firefox-dde", "libreoffice", "thunderbird", "gimp", "chromium-browser"}
	negative := []string{"vim", "lastore-daemon"}

	for _, p := range positive {
		d := pkg_recommend.GetEnhancedLocalePackages(lang, p)
		c.Check(len(d), C.Not(C.Equals), 0)
	}
	for _, p := range negative {
		d := pkg_recommend.GetEnhancedLocalePackages(lang, p)
		c.Check(len(d), C.Equals, 0)
	}
}

func (*testWrap) TestNormalizePackageNames(c *C.C) {
	s, err := NormalizePackageNames("ab bc cd")
	c.Check(err, C.Equals, nil)
	c.Check(strings.Join(s, "_"), C.Equals, "ab_bc_cd")

	s, err = NormalizePackageNames("")
	c.Check(err, C.Not(C.Equals), nil)
	c.Check(len(s), C.Equals, 0)
}
