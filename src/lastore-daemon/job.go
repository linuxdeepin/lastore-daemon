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
		Status:     system.ReadyStatus,
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

func (j *Job) updateInfo(info system.JobProgressInfo) {
	if info.Description != j.Description {
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if !TransitionJobState(j, info.Status) {
		panic("Can't transition job " + j.Id + " status from " + string(j.Status) + " to " + string(info.Status))
	}

	if info.Progress != j.Progress && info.Progress != -1 {
		j.Progress = info.Progress
		dbus.NotifyChange(j, "Progress")
	}
	log.Printf("JobId: %q(%q)  ----> progress:%f ----> msg:%q, status:%q\n", j.Id, j.PackageId, j.Progress, j.Description, j.Status)

	if j.Status == system.SucceedStatus {
		if j.next != nil {
			j.swap(j.next)
			j.next = nil
		} else {
			TransitionJobState(j, system.EndStatus)
		}
	}
}

func (j *Job) swap(j2 *Job) {
	log.Printf("Swaping from %v to %v", j, j2)
	if j2.Id != j.Id {
		panic("Can't swap Job with differnt Id")
	}
	j.Type = j2.Type
	dbus.NotifyChange(j, "Type")
	info := system.JobProgressInfo{
		JobId:       j.Id,
		Progress:    j2.Progress,
		Description: j2.Description,
		Status:      system.Status(j2.Status),
	}
	// force change status
	j.Status = j2.Status

	j.updateInfo(info)
}
