package utils

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestReadFileContent(t *testing.T) {
	convey.Convey("Test ReadFileContent", t, func() {
		var value = "hello"
		data, err := ReadFileContent("testdata/file.hello")
		convey.So(data, convey.ShouldEqual, value)
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestSHA256Sum(t *testing.T) {
	convey.Convey("Test SHA256Sum", t, func() {
		convey.So(SHA256Sum([]byte("hello,world")), convey.ShouldEqual,
			"77df263f49123356d28a4a8715d25bf5b980beeeb503cab46ea61ac9f3320eda")
	})
}

func TestScanDir(t *testing.T) {
	convey.Convey("Test ScanDir", t, func() {
		var values = []string{"dir1", "dir2"}
		names, err := ScanDir("testdata/scan-dirs", func(string) bool {
			return false
		})
		convey.So(names, convey.ShouldResemble, values)
		convey.So(err, convey.ShouldBeNil)
		names, _ = ScanDir("testdata/scan-dirs", func(string) bool {
			return true
		})
		convey.So(len(names), convey.ShouldEqual, 0)
	})
}

func TestProcGetByKey(t *testing.T) {
	convey.Convey("Test ProcGetByKey", t, func() {
		var set = map[string]string{
			"model name": "",
			"cpu cores":  "",
		}
		err := ProcGetByKey("testdata/proc.file", ":", set, false)
		convey.So(err, convey.ShouldBeNil)
		convey.So(set["model name"], convey.ShouldEqual, "Intel I7-9750H")
		convey.So(set["cpu cores"], convey.ShouldEqual, "0")

		ProcGetByKey("testdata/proc.file", ":", set, true)
		convey.So(set["model name"], convey.ShouldEqual, "Intel I7-9750H")
		convey.So(set["cpu cores"], convey.ShouldEqual, "1")

		set = make(map[string]string)
		set["id"] = ""
		err = ProcGetByKey("testdata/proc.file", ":", set, true)
		convey.So(err, convey.ShouldNotBeNil)
	})
}
