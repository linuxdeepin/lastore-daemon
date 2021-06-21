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

package apt

import "testing"
import C "gopkg.in/check.v1"
import "internal/system"

type testWrap struct{}

func TestSystemAptAll(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestParseInfo(c *C.C) {
	line := "dummy:" + system.RunningStatus + ":1:" + "running"
	info, err := ParseProgressInfo("jobid", string(line))
	c.Check(err, C.Equals, nil)
	c.Check(info.Status, C.Equals, system.RunningStatus)
	c.Check(info.JobId, C.Equals, "jobid")
}
