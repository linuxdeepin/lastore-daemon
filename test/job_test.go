package main

import lastore "./stub"
import "testing"
import C "gopkg.in/check.v1"
import "pkg.deepin.io/lib/dbus"
import "fmt"
import "time"

type testWrap struct {
	m *lastore.Manager
	u *lastore.Updater
}

func (wrap *testWrap) SetUpSuite(c *C.C) {
	var err error
	wrap.m, err = lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	c.Check(err, C.Equals, nil)
	wrap.u, err = lastore.NewUpdater("com.deepin.lastore", "/com/deepin/lastore")
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
	job, err := lastore.NewJob("com.deepin.lastore", o)
	if err != nil {
		panic(err)
	}
	return job
}

func (wrap *testWrap) TestDownload(c *C.C) {
	return
	job := GetJob(wrap.m.DownloadPackage("deepin-movie"))
	c.Check(job, C.Not(C.Equals), nil)
	c.Check(job.PackageId.Get(), C.Equals, "deepin-movie")
	c.Check(job.Status.Get(), C.Equals, "running")
	c.Check(job.Type.Get(), C.Equals, "download")
	c.Check(job.Progress.Get(), C.Equals, 0.0)
	c.Check(wrap.m.CleanJob(job.Id.Get()), C.Not(C.Equals), nil)
	s := WaitJob(job)
	c.Check(s, C.Equals, "succeed")
}

func (wrap *testWrap) TestQueue(c *C.C) {
	return
	ps := []string{"deepin-movie", "deepin-music", "abiword", "abiword"}
	for _, p := range ps {
		wrap.m.RemovePackage(p)
	}
	for _, p := range ps {
		job := GetJob(wrap.m.DownloadPackage(p))
		c.Check(job.Status.Get(), C.Equals, "ready")
	}

}

func WaitJob(j *lastore.Job) string {
	done := make(chan bool)
	newState := j.Status.Get()

	if newState != RunningStatus {
		return newState
	}

	j.Status.ConnectChanged(func() {
		newState = j.Status.Get()
		if newState != RunningStatus {
			done <- true
		}
	})
	<-done
	return newState
}

func (wrap *testWrap) TestInvalidAction(c *C.C) {
	job, err := wrap.m.RemovePackage("xx")
	c.Check(err, C.Not(C.Equals), nil)
	c.Check(string(job), C.Equals, "")

	job, err = wrap.m.InstallPackage("dde-daemon")
	c.Check(err, C.Not(C.Equals), nil)
	c.Check(string(job), C.Equals, "")

	wrap.m.InstallPackage("vim-doc")
	wrap.m.UpdatePackage("google-chrome-beta")
	wrap.m.UpdatePackage("wesnoth-1.12-data")
	wrap.m.UpdatePackage("0ad-data")
	wrap.m.DistUpgrade()
}

func (wrap *testWrap) TestPauseApp(c *C.C) {
	return
	job := GetJob(wrap.m.DownloadPackage("google-chrome-stable"))
	<-time.After(time.Second * 3)
	err := wrap.m.PauseJob(job.Id.Get())
	c.Check(err, C.Equals, nil)

}
func (wrap *testWrap) TestDistUpgrade(c *C.C) {
	return
	job := GetJob(wrap.m.DistUpgrade())
	<-time.After(time.Second)
	wrap.m.PauseJob("1dist_upgrade")
	s := WaitJob(job)
	c.Check(s, C.Not(C.Equals), "running")
	fmt.Println("DistUpgrade:", job)
}

func (wrap *testWrap) TestUpdate(c *C.C) {
	return
	job := GetJob(wrap.m.UpdatePackage("deepin-movie"))
	id := job.Id.Get()
	c.Check(job, C.Not(C.Equals), nil)
	c.Check(job.PackageId.Get(), C.Equals, "deepin-movie")
	c.Check(job.Status.Get(), C.Equals, "running")
	c.Check(job.Type.Get(), C.Equals, "update")
	c.Check(job.Progress.Get(), C.Equals, 0.0)

	done := make(chan bool)

	oldState := job.Status.Get()
	job.Status.ConnectChanged(func() {
		newState := job.Status.Get()
		fmt.Printf("Change %q from %q to %q ... \n", id, oldState, newState)

		// TODO: go-dbus lost signals cause this assert failed
		// c.Check(ValidTransitionJobState(oldState, newState), C.Equals, true)

		oldState = newState
		if newState == "end" {
			done <- true
		}
	})
	c.Check(job.Status.Get(), C.Equals, "running")
	<-done
	c.Check(job.Status.Get(), C.Equals, "end")
	c.Check(job.Progress.Get(), C.Equals, 1.0)
}
