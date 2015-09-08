package main

import (
	"./system"
	"log"
	"pkg.deepin.io/lib/dbus"
	"strconv"
)

var jobId = func() func() string {
	var __count = 0
	return func() string {
		__count++
		return strconv.Itoa(__count)
	}
}()

var __jobIdCounter = 1

type Job struct {
	next *Job

	Id        string
	PackageId string

	Type string

	Status string

	Progress    float64
	Description string
	ElapsedTime int32

	Notify func(status int32)
}

func (j *Job) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		"org.deepin.lastore",
		"/org/deepin/lastore/Job" + j.Id,
		"org.deepin.lastore.Job",
	}
}

func NewDownloadJob(packageId string, dest string) (*Job, error) {
	j := &Job{
		Id:          jobId(),
		Type:        DownloadJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}
	return j, nil
}

func NewInstallJob(packageId string) (*Job, error) {
	id := jobId()
	var next = &Job{
		Id:          id,
		Type:        InstallJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}

	j := &Job{
		Id:          id,
		Type:        DownloadJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
		next:        next,
	}

	return j, nil
}

func NewRemoveJob(packageId string) (*Job, error) {
	j := &Job{
		Id:          jobId(),
		Type:        RemoveJobType,
		PackageId:   packageId,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}
	return j, nil
}

func (j *Job) updateInfo(info system.ProgressInfo) {
	if info.Description != j.Description {
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if string(info.Status) != j.Status {
		j.Status = string(info.Status)
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
		err := sys.Download(j.Id, j.PackageId)
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
