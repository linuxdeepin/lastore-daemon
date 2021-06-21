package main

import (
	"internal/system"
	"internal/system/apt"

	C "gopkg.in/check.v1"
)

type jobManagerSuite struct{}

func init() {
	C.Suite(&jobManagerSuite{})
	NotUseDBus = true
}

func (*jobManagerSuite) TestSuiteJobManager(c *C.C) {
	jm := NewJobManager(nil, apt.New(), nil)

	// 空包只走流程
	_, err := jm.CreateJob(system.DistUpgradeJobType, system.InstallJobType, nil, nil, 0)
	c.Check(err, C.Equals, nil)
	c.Check(jm.findJobByType(system.DistUpgradeJobType, nil), C.Equals, (*Job)(nil))

	jobDistUpgrade2, err := jm.CreateJob("", system.DistUpgradeJobType, nil, nil, 0)
	c.Check(err, C.Equals, nil)
	c.Check(jm.findJobByType(system.DistUpgradeJobType, nil), C.Equals, jobDistUpgrade2)

	jobDownload, err := jm.CreateJob(system.DownloadJobType, system.DownloadJobType, nil, nil, 0)
	c.Check(err, C.Equals, nil)
	c.Check(jm.findJobByType(system.DownloadJobType, nil), C.Equals, jobDownload)

	jm.MarkStart(jobDistUpgrade2.Id)
	c.Check(jm.List().Len(), C.Equals, 2)

	jobDistUpgrade2.Status = system.RunningStatus
	jm.CleanJob(jobDistUpgrade2.Id)
	c.Check(jobDistUpgrade2.Status, C.Equals, system.RunningStatus)
	jm.removeJob(jobDownload.Id, DownloadQueue)
	c.Check(jm.List().Len(), C.Equals, 1)
}
