// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"internal/system"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
)

type Job struct {
	service *dbusutil.Service
	next    *Job
	option  map[string]string
	PropsMu sync.RWMutex

	Id   string
	Name string
	// dbusutil-gen: equal=nil
	Packages     []string
	CreateTime   int64
	DownloadSize int64

	Type string

	Status     system.Status
	lastStatus system.Status // 用于保存reload前的job状态
	needReload bool
	caller     methodCaller

	Progress    float64
	Description string

	// completed bytes per second
	Speed      int64
	speedMeter SpeedMeter

	Cancelable bool

	queueName string
	retry     int

	// adjust the progress range, used by some download job type
	progressRangeBegin float64
	progressRangeEnd   float64

	environ map[string]string

	hooks   map[string]func()
	hooksMu sync.Mutex
}

func NewJob(service *dbusutil.Service, id, jobName string, packages []string, jobType, queueName string, environ map[string]string) *Job {
	j := &Job{
		service:    service,
		Id:         id,
		Name:       jobName,
		CreateTime: time.Now().UnixNano(),
		Type:       jobType,
		Packages:   packages,
		Status:     system.ReadyStatus,
		Progress:   .0,
		Cancelable: true,

		option:    make(map[string]string),
		queueName: queueName,
		retry:     3,

		progressRangeBegin: 0,
		progressRangeEnd:   1,
		environ:            environ,
	}
	if jobType == system.DownloadJobType {
		go j.initDownloadSize()
	}
	return j
}

func (j *Job) initDownloadSize() {
	s, _, err := system.QueryPackageDownloadSize(system.AllUpdate, j.Packages...)
	if err != nil {
		logger.Warningf("initDownloadSize failed: %v", err)
		return
	}
	size := int64(s)
	j.PropsMu.Lock()
	if j.DownloadSize == 0 {
		j.DownloadSize = size
		_ = j.emitPropChangedDownloadSize(size)
	}
	j.speedMeter.SetDownloadSize(size)
	j.PropsMu.Unlock()
}

func (j *Job) String() string {
	return fmt.Sprintf("Job{Id:%q:%q,Type:%q(%v,%v), %q(%.2f)}@%q",
		j.Id, j.Packages,
		j.Type, j.Cancelable, j.Status,
		j.Description, j.Progress, j.queueName,
	)
}

// updateInfo update Job information from info and return
// whether the information changed.
func (j *Job) updateInfo(info system.JobProgressInfo) bool {
	var changed = false

	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()

	if info.Error == nil {
		if info.Description != j.Description {
			changed = true
			j.Description = info.Description
			_ = j.emitPropChangedDescription(info.Description)
		}
	} else {
		changed = true
		j.setError(info.Error)
	}

	if info.Cancelable != j.Cancelable {
		changed = true
		j.Cancelable = info.Cancelable
		_ = j.emitPropChangedCancelable(info.Cancelable)
	}
	logger.Debugf("updateInfo %v <- %v\n", j, info)

	cProgress := buildProgress(info.Progress, j.progressRangeBegin, j.progressRangeEnd)
	if cProgress > j.Progress {
		changed = true
		j.Progress = cProgress
		_ = j.emitPropChangedProgress(cProgress)
	}

	// see the apt.go, we scale download progress value range in [0,0.5
	speed := j.speedMeter.Speed(info.Progress)

	if speed != j.Speed {
		changed = true
		j.Speed = speed
		_ = j.emitPropChangedSpeed(speed)
	}

	if info.FatalError {
		j.retry = 0
	}

	if info.Status != j.Status {
		err := TransitionJobState(j, info.Status, false)
		if err != nil {
			logger.Warningf("_UpdateInfo: %v\n", err)
			return false
		}
		changed = true
	}
	if info.Status == system.PausedStatus && j.needReload {
		err := TransitionJobState(j, system.ReloadStatus, false)
		if err != nil {
			logger.Warningf("_UpdateInfo Reload: %v\n", err)
		} else {
			changed = true
		}
		j.needReload = false
	}

	return changed
}

func (j *Job) _InitProgressRange(begin, end float64) {
	if end <= begin || end-begin > 1 || begin > 1 || end > 1 {
		panic("Invalid Progress range init")
	}
	if j.Progress != 0 {
		panic("InitProgressRange can only invoke once and before job start")
	}
	j.progressRangeBegin = begin
	j.progressRangeEnd = end
	j.Progress = j.progressRangeBegin
}

func buildProgress(p, begin, end float64) float64 {
	return begin + p*(end-begin)
}

type Error interface {
	GetType() string
	GetDetail() string
}

func (j *Job) setError(e Error) {
	errValue := struct {
		ErrType   string
		ErrDetail string
	}{
		e.GetType(), e.GetDetail(),
	}
	jsonBytes, err := json.Marshal(errValue)
	if err != nil {
		logger.Warning(err)
		return
	}
	j.setPropDescription(string(jsonBytes))
}

func (j *Job) getHook(name string) func() {
	j.hooksMu.Lock()
	fn := j.hooks[name]
	j.hooksMu.Unlock()
	return fn
}

func (j *Job) setHooks(hooks map[string]func()) {
	j.hooksMu.Lock()
	j.hooks = hooks
	j.hooksMu.Unlock()
}

func (j *Job) wrapHooks(appendHooks map[string]func()) {
	j.hooksMu.Lock()
	defer j.hooksMu.Unlock()
	if j.hooks == nil {
		j.hooks = appendHooks
		return
	}
	for key, fn := range appendHooks {
		appendFn := fn
		f, ok := j.hooks[key]
		if ok {
			j.hooks[key] = func() {
				f()
				appendFn()
			}
		} else {
			j.hooks[key] = fn
		}
	}
}
