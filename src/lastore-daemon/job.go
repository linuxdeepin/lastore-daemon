package main

import (
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
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

	Cancelable bool

	queueName string
}

func NewJob(packageId string, jobType string, queueName string) *Job {
	j := &Job{
		Id:         genJobId() + jobType,
		CreateTime: time.Now().UnixNano(),
		Type:       jobType,
		PackageId:  packageId,
		Status:     system.ReadyStatus,
		Progress:   .0,
		Cancelable: true,
		option:     make(map[string]string),
		queueName:  queueName,
	}
	return j
}

func (j *Job) changeType(jobType string) {
	j.Type = jobType
}

func (j Job) String() string {
	return fmt.Sprintf("Job{Id:%q:%q,Type:%q(%v), %q(%v)}@%q", j.Id, j.PackageId, j.Type, j.Cancelable, j.Description, j.Progress, j.queueName)
}

// _UpdateInfo update Job information from info and return
// whether the information changed.
func (j *Job) _UpdateInfo(info system.JobProgressInfo) bool {
	if err := TransitionJobState(j, info.Status); err != nil {
		log.Warnf("_UpdateInfo: %v\n", err)
		return false
	}

	var changed = false

	if info.Description != j.Description {
		changed = true
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	if info.Progress != j.Progress && info.Progress != -1 {
		changed = true
		j.Progress = info.Progress
		dbus.NotifyChange(j, "Progress")
	}

	if info.Cancelable != j.Cancelable {
		changed = true
		j.Cancelable = info.Cancelable
		dbus.NotifyChange(j, "Cancelable")
	}

	log.Infof("updateInfo %v <- %v\n", j, info)

	if j.Status == system.SucceedStatus {
		TransitionJobState(j, system.EndStatus)
		changed = true
	}
	return changed
}
