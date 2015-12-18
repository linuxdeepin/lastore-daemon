package apt

import "testing"
import C "gopkg.in/check.v1"
import "internal/system"

type testWrap struct{}

func Test(t *testing.T) { C.TestingT(t) }
func init() {
	C.Suite(&testWrap{})
}

func (*testWrap) TestParseInfo(c *C.C) {
	line := "dstatus:" + system.RunningStatus + ":" + "running"
	info, err := ParseProgressInfo("jobid", line)
	c.Check(err, C.Equals, nil)
	c.Check(info.Status, C.Equals, system.RunningStatus)
	c.Check(info.JobId, C.Equals, "jobid")
}
