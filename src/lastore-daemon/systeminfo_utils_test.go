package main

import (
	"os"

	C "gopkg.in/check.v1"
)

type systeminfoSuite struct{}

func init() {
	C.Suite(&systeminfoSuite{})
	NotUseDBus = true
}

func (*systeminfoSuite) TestSystemInfoUtils(c *C.C) {
	sys, err := getSystemInfo()
	if err != nil {
		if os.IsNotExist(err) {
			c.Log("TestSystemInfoUtils error:", err)
			return
		} else {
			c.Assert(err, C.Equals, nil)
		}
	}
	c.Check(sys.SystemName, C.Not(C.Equals), "")
	c.Check(sys.ProductType, C.Not(C.Equals), "")
	c.Check(sys.EditionName, C.Not(C.Equals), "")
	c.Check(sys.Version, C.Not(C.Equals), "")
	c.Check(sys.HardwareId, C.Not(C.Equals), "")
	c.Check(sys.Processor, C.Not(C.Equals), "")
}
