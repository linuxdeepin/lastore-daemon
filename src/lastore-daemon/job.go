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

func NewJob(packageId string, jobType string, region string) *Job {
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
	if region != "" {
		j.option["region"] = region
	}
	return j
}

func NewRemoveJob(packageId string) *Job {
	return NewJob(packageId, RemoveJobType, "")
}
func NewDownloadJob(packageId string, region string) *Job {
	return NewJob(packageId, DownloadJobType, region)
}
func NewInstallJob(packageId string, region string) *Job {
	installJob := NewJob(packageId, InstallJobType, region)
	downloadJob := NewDownloadJob(packageId, region)
	downloadJob.Id = installJob.Id
	downloadJob.next = installJob
	return downloadJob
}

func (j *Job) updateInfo(info system.ProgressInfo) {
	if info.Description != j.Description {
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if info.Status != j.Status {
		j.Status = info.Status
		dbus.NotifyChange(j, "Status")
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
	j.updateInfo(info)
}

func (j *Job) start(sys system.System) error {
	switch j.Type {
	case DownloadJobType:
		err := sys.Download(j.Id, j.PackageId, j.option["region"])
		if err != nil {
			return err
		}
		return sys.Start(j.Id)
	case InstallJobType:
		err := sys.Install(j.Id, j.PackageId)
		if err != nil {
			return err
		}
		return sys.Start(j.Id)

	case RemoveJobType:
		err := sys.Remove(j.Id, j.PackageId)
		if err != nil {
			return err
		}
		return sys.Start(j.Id)
	default:
		return system.NotFoundError
	}
}
