/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/json"
	"fmt"
	"internal/system"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbusutil"
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

	Status system.Status

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
	s, err := system.QueryPackageDownloadSize(j.Packages...)
	if err != nil {
		log.Warnf("initDownloadSize failed: %v", err)
		return
	}
	size := int64(s)
	j.PropsMu.Lock()
	if j.DownloadSize == 0 {
		j.DownloadSize = size
		j.emitPropChangedDownloadSize(size)
	}
	j.speedMeter.SetDownloadSize(size)
	j.PropsMu.Unlock()
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

// _UpdateInfo update Job information from info and return
// whether the information changed.
func (j *Job) _UpdateInfo(info system.JobProgressInfo) bool {
	var changed = false

	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()

	if info.Error == nil {
		if info.Description != j.Description {
			changed = true
			j.Description = info.Description
			j.emitPropChangedDescription(info.Description)
		}
	} else {
		changed = true
		j.setError(info.Error)
	}

	if info.Cancelable != j.Cancelable {
		changed = true
		j.Cancelable = info.Cancelable
		j.emitPropChangedCancelable(info.Cancelable)
	}
	log.Tracef("updateInfo %v <- %v\n", j, info)

	cProgress := buildProgress(info.Progress, j.progressRangeBegin, j.progressRangeEnd)
	if cProgress > j.Progress {
		changed = true
		j.Progress = cProgress
		j.emitPropChangedProgress(cProgress)
	}

	// see the apt.go, we scale download progress value range in [0,0.5
	speed := j.speedMeter.Speed(info.Progress)

	if speed != j.Speed {
		changed = true
		j.Speed = speed
		j.emitPropChangedSpeed(speed)
	}

	if info.FatalError {
		j.retry = 0
	}

	if info.Status != j.Status {
		err := TransitionJobState(j, info.Status)
		if err != nil {
			log.Warnf("_UpdateInfo: %v\n", err)
			return false
		}
		changed = true
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
		log.Warn(err)
		return
	}
	j.setPropDescription(string(jsonBytes))
}
