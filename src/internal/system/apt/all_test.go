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
