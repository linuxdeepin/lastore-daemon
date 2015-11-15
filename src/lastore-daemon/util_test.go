package main

import "testing"
import C "gopkg.in/check.v1"
import "internal/system/apt"
import "internal/system"

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
			c.Check(GuestPackageDownloadSize(p), C.Equals, float64(0))
		} else {
			c.Check(GuestPackageDownloadSize(p), C.Not(C.Equals), float64(0))
		}
	}
}

func (*testWrap) TestTranisition(c *C.C) {
	var data = []struct {
		from  system.Status
		to    system.Status
		valid bool
	}{
		{system.ReadyStatus, system.ReadyStatus, false},
		{system.ReadyStatus, system.RunningStatus, true},
		{system.ReadyStatus, system.FailedStatus, false},
		{system.ReadyStatus, system.SucceedStatus, false},
		{system.ReadyStatus, system.PausedStatus, true},
		{system.ReadyStatus, system.EndStatus, false},

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
		{system.PausedStatus, system.EndStatus, false},

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
