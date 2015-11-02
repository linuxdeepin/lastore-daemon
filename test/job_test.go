package main

import lastore "./stub"
import "testing"
import C "gopkg.in/check.v1"
import "pkg.deepin.io/lib/dbus"

type testWrap struct {
	m *lastore.Manager
	u *lastore.Updater
}

func (wrap *testWrap) SetUpSuite(c *C.C) {
	var err error
	wrap.m, err = lastore.NewManager("org.deepin.lastore", "/org/deepin/lastore")
	c.Check(err, C.Equals, nil)
	wrap.u, err = lastore.NewUpdater("org.deepin.lastore", "/org/deepin/lastore")
	c.Check(err, C.Equals, nil)
}

func Test(t *testing.T) { C.TestingT(t) }

func init() {
	C.Suite(&testWrap{})
}

func (wrap *testWrap) TestInit(c *C.C) {
	c.Check(wrap.m, C.Not(C.Equals), nil)
	c.Check(wrap.u, C.Not(C.Equals), nil)
}

func GetJob(o dbus.ObjectPath, err error) *lastore.Job {
	if err != nil {
		panic(err)
	}
	job, err := lastore.NewJob("org.deepin.lastore", o)
	if err != nil {
		panic(err)
	}
	return job
}

func (wrap *testWrap) TestInstall(c *C.C) {
	job := GetJob(wrap.m.InstallPackage("deepin-movie"))
	c.Check(job, C.Not(C.Equals), nil)
	c.Check(job.PackageId.Get(), C.Equals, "deepin-movie")
	c.Check(job.Status.Get(), C.Equals, "ready")
	c.Check(job.Type.Get(), C.Equals, "download")
	c.Check(job.Progress.Get(), C.Equals, 0.0)

	c.Check(wrap.m.StartJob(job.Id.Get()), C.Equals, nil)
	c.Check(wrap.m.CleanJob(job.Id.Get()), C.Not(C.Equals), nil)
}
