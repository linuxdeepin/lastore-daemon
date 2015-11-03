package main

import (
	"fmt"
	"internal/system"
	"log"
	"pkg.deepin.io/lib/dbus"
	"sort"
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

type JobList []*Job

func (l JobList) Len() int {
	return len(l)
}
func (l JobList) Less(i, j int) bool {
	return l[i].CreateTime < l[j].CreateTime
}

func (l JobList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l JobList) Add(j *Job) (JobList, error) {
	for _, item := range l {
		if item.PackageId == j.PackageId && item.Type == j.Type {
			return l, fmt.Errorf("exists job %q:%q", item.Type, item.PackageId)
		}
	}
	r := append(l, j)
	sort.Sort(r)
	return r, nil
}

func (l JobList) Remove(id string) (JobList, error) {
	index := -1
	for i, item := range l {
		if item.Id == id {
			index = i
			break
		}
	}
	if index == -1 {
		return l, system.NotFoundError
	}

	r := append(l[0:index], l[index+1:]...)
	sort.Sort(r)
	return r, nil
}

func (l JobList) Find(id string) (*Job, error) {
	for _, item := range l {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, system.NotFoundError
}

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
	ElapsedTime int32

	Notify func(status int32)
}

func (j *Job) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "org.deepin.lastore",
		ObjectPath: "/org/deepin/lastore/Job" + j.Id,
		Interface:  "org.deepin.lastore.Job",
	}
}

func NewJob(packageId string, jobType string) *Job {
	j := &Job{
		Id:          genJobId() + jobType,
		CreateTime:  time.Now().UnixNano(),
		Type:        jobType,
		PackageId:   packageId,
		Status:      system.ReadyStatus,
		Progress:    .0,
		ElapsedTime: 0,
		option:      make(map[string]string),
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

func (j *Job) updateInfo(info system.ProgressInfo) {
	if info.Description != j.Description {
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if !TransitionJobState(j, info.Status) {
		panic("Can't transition job status from " + string(j.Status) + " to " + string(info.Status))
	}

	if info.Progress != j.Progress && info.Progress != -1 {
		j.Progress = info.Progress
		dbus.NotifyChange(j, "Progress")
	}
	log.Printf("JobId: %q(%q)  ----> progress:%f ----> msg:%q, status:%q\n", j.Id, j.PackageId, j.Progress, j.Description, j.Status)
}

func (j *Job) swap(j2 *Job) {
	log.Printf("Swaping from %v to %v", j, j2)
	if j2.Id != j.Id {
		panic("Can't swap Job with differnt Id")
	}
	j.Type = j2.Type
	dbus.NotifyChange(j, "Type")
	info := system.ProgressInfo{
		JobId:       j.Id,
		Progress:    j2.Progress,
		Description: j2.Description,
		Status:      system.Status(j2.Status),
	}
	// force change status
	j.Status = j2.Status

	j.updateInfo(info)
}
