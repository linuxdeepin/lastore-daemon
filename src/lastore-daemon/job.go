package main

import (
	"internal/system"
	"log"
	"pkg.deepin.io/lib/dbus"
	"strconv"
	"time"
)

var genJobId = func() func() string {
	var __count = 0
	return func() string {
		__count++
		return strconv.Itoa(__count)
	}
}()

type Job struct {
	next   *Job
	option map[string]string

	Id         string
	PackageId  string
	CreateTime int64

	Type string

	Status system.Status

	Progress    float64
	Description string
}

func NewJob(packageId string, jobType string) *Job {
	j := &Job{
		Id:         genJobId() + jobType,
		CreateTime: time.Now().UnixNano(),
		Type:       jobType,
		PackageId:  packageId,
		Status:     system.StartStatus,
		Progress:   .0,
		option:     make(map[string]string),
	}
	return j
}

func NewDistUpgradeJob() *Job {
	return NewJob("", system.DistUpgradeJobType)
}
func NewUpdateJob(packageId string) *Job {
	return NewJob(packageId, system.UpdateJobType)
}
func NewRemoveJob(packageId string) *Job {
	return NewJob(packageId, system.RemoveJobType)
}
func NewDownloadJob(packageId string) *Job {
	return NewJob(packageId, system.DownloadJobType)
}
func NewInstallJob(packageId string) *Job {
	installJob := NewJob(packageId, system.InstallJobType)

	downloadJob := NewDownloadJob(packageId)
	downloadJob.Id = installJob.Id
	downloadJob.next = installJob
	return downloadJob
}

func (j *Job) changeType(jobType string) {
	j.Type = jobType
}

func (j *Job) updateInfo(info system.JobProgressInfo) {
	if info.Description != j.Description {
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if !TransitionJobState(j, info.Status) {
		log.Printf("Can't transition job %q status from %q to %q\n", j.Id, j.Status, info.Status)
		return
	}

	if info.Progress != j.Progress && info.Progress != -1 {
		j.Progress = info.Progress
		dbus.NotifyChange(j, "Progress")
	}
	log.Printf("JobId: %q(%q)  ----> progress:%f ----> msg:%q, status:%q\n", j.Id, j.PackageId, j.Progress, j.Description, j.Status)

	if j.Status == system.SucceedStatus {
		TransitionJobState(j, system.EndStatus)
	}
}
