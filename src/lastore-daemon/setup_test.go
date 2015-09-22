package main

/*
import (
	proxy "./dbusproxy"
	. "github.com/smartystreets/goconvey/convey"
	//	"pkg.deepin.io/lib/dbus"
	"fmt"
	"testing"
)

func TestReleaseFDs(t *testing.T) {
	m, _ := proxy.NewManager("org.deepin.lastore", "/org/deepin/lastore")
	jobp, _ := m.InstallPackages([]string{"deepin-movie"})
	job, _ := proxy.NewJob("org.deepin.lastore", jobp)
	m.StartJob(job.Id.Get())
}

func TestSetup(t *testing.T) {
	return
	var job *proxy.Job
	var m *proxy.Manager
	var err error
	var done = make(chan bool)
	ps := []string{"deepin-movie"}
	Convey("Test dbus service features, please setup lastore-dameon before test this", t, func() {
		m, err = proxy.NewManager("org.deepin.lastore", "/org/deepin/lastore")
		So(err, ShouldBeNil)

		Convey(fmt.Sprintf("Try removing the package of %v", ps), func() {
			Convey("Call Manager.RemovePackages ", func() {
				jobp, err := m.RemovePackages(ps)
				So(err, ShouldBeNil)
				job, err = proxy.NewJob("org.deepin.lastore", jobp)
				So(err, ShouldBeNil)

				Convey("Get the Job object from "+string(job.Path)+" and start it", func() {
					r, err := m.StartJob(job.Id.Get())
					So(err, ShouldBeNil)
					So(r, ShouldEqual, true)

					Convey("Wait the package removed", func(c C) {

						job.Progress.ConnectChanged(func() {
							c.Printf("\nAction:%q Name:%q Progress:%f Status:%q\n",
								job.Type.Get(), job.PackageId.Get(), job.Progress.Get(), job.Status.Get())
							if job.Progress.Get() == 1 {
								done <- true
							}
						})
						So(job.Status.Get(), ShouldEqual, "ready")
						<-done
						So(job.Status.Get(), ShouldEqual, "success")

						Convey("Whether this job is still live", func(c C) {
							So(job.Status.Get(), ShouldEqual, "success")
							//So(m.JobList.Get(), ShouldContain, jobp)
						})

						Convey("Clean this Job", func() {
							//So(m.JobList.Get(), ShouldContain, jobp)
						})
						//So(job.Status.Get(), ShouldEqual, "success")
						So(err, ShouldBeNil)
					})
				})

			})

			Convey("Call Manager.InstallPackages ", func() {
				jobp, err := m.InstallPackages(ps)
				So(err, ShouldBeNil)
				job, err = proxy.NewJob("org.deepin.lastore", jobp)
				So(err, ShouldBeNil)

				Convey("Get the Job object from "+string(job.Path)+" and start it", func() {
					So(job.Status.Get(), ShouldEqual, "ready")

					r, err := m.StartJob(job.Id.Get())
					So(err, ShouldBeNil)
					So(r, ShouldEqual, true)

					Convey("Wait the package removed", func(c C) {
						job.Progress.ConnectChanged(func() {
							c.Printf("\nAction:%q Name:%q Progress:%f Status:%q\n",
								job.Type.Get(), job.PackageId.Get(), job.Progress.Get(), job.Status.Get())
							if job.Progress.Get() == 1 {
								done <- true
							}

						})
					})
					<-done

					So(job.Status.Get(), ShouldEqual, "success")
				})

			})
		})
	})
}
*/
