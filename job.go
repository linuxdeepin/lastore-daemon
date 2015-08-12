package main

import (
	"./system"
	"pkg.deepin.io/lib/dbus"
)

type Job struct {
	Id        string
	Type      string
	PackageId string
	Status    string

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

func NewDownloadJob(pid string, dest string) (*Job, error) {
	j := &Job{
		Type:        DownloadJobType,
		PackageId:   pid,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}
	return j, nil
}

func NewInstallJob(pid string, dest string) (*Job, error) {
	j := &Job{
		Type:        InstallJobType,
		PackageId:   pid,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}
	return j, nil
}

func NewRemoveJob(pid string) (*Job, error) {
	j := &Job{
		Type:        RemoveJobType,
		PackageId:   pid,
		Status:      string(system.ReadyStatus),
		Progress:    .0,
		ElapsedTime: 0,
	}
	return j, nil
}
