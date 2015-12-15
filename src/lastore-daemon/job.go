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
	Name       string
	Packages   []string
	CreateTime int64

	Type string

	Status system.Status

	Progress    float64
	Description string

	// completed bytes per second
	Speed int64
	//  effect bytes
	effectSizes float64
	// updateInfo timestamp
	updateProgressTime time.Time

	Cancelable bool

	queueName string
	retry     int
}

func NewJob(jobName string, packages []string, jobType string, queueName string) *Job {
	j := &Job{
		Id:         genJobId() + jobType,
		Name:       jobName,
		CreateTime: time.Now().UnixNano(),
		Type:       jobType,
		Packages:   packages,
		Status:     system.ReadyStatus,
		Progress:   .0,
		Cancelable: false,
		option:     make(map[string]string),
		queueName:  queueName,
		retry:      3,
	}
	return j
}

func (j *Job) setEffectSizes() bool {
	if j.effectSizes > 0 {
		return true
	}

	switch j.Type {
	case system.DownloadJobType:
		j.effectSizes = QueryPackageDownloadSize(j.Packages...)
	}
	return j.effectSizes > 0
}

func (j *Job) changeType(jobType string) {
	j.Type = jobType
}

func (j Job) String() string {
	return fmt.Sprintf("Job{Id:%q:%q,Type:%q(%v,%v), %q(%.2f)}@%q",
		j.Id, j.Packages,
		j.Type, j.Cancelable, j.Status,
		j.Description, j.Progress, j.queueName,
	)
}

func SmoothCalc(oldSpeed, newSpeed int64, interval time.Duration) int64 {
	ratio := float64(time.Second-interval) * 1.0 / float64(time.Second)
	if ratio < 0 {
		return int64(newSpeed)
	}
	return int64(float64(oldSpeed)*(1-ratio) + float64(newSpeed)*ratio)
}

// _UpdateInfo update Job information from info and return
// whether the information changed.
func (j *Job) _UpdateInfo(info system.JobProgressInfo) bool {
	var changed = false

	if info.Status != j.Status {
		err := TransitionJobState(j, info.Status)
		if err != nil {
			log.Warnf("_UpdateInfo: %v\n", err)
			return false
		}
		changed = true
	}

	if info.Description != j.Description {
		changed = true
		j.Description = info.Description
		dbus.NotifyChange(j, "Description")
	}

	// The Progress may not changed when we calculate speed.
	if j.setEffectSizes() {
		completed := (info.Progress - j.Progress) * j.effectSizes
		now := time.Now()

		if s := now.Sub(j.updateProgressTime).Seconds(); s > 0 && completed >= 0 {
			j.Speed = SmoothCalc(j.Speed, int64(completed/s), now.Sub(j.updateProgressTime))
			dbus.NotifyChange(j, "Speed")
		}
		j.updateProgressTime = now
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
		err := TransitionJobState(j, system.EndStatus)
		if err != nil {
			log.Warnf("_UpdateInfo: %v\n", err)
		}
		changed = true
	}
	return changed
}
