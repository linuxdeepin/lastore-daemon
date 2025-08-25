// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

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

	Status system.Status
	caller methodCaller

	Progress    float64
	Description string

	// completed bytes per second
	Speed      int64
	speedMeter SpeedMeter

	Cancelable bool

	queueName         string
	retry             int
	subRetryHookFn    func(*Job) // hook执行规则是在retry--之前执行hook
	realRunningHookFn func()

	// adjust the progress range, used by some download job type
	progressRangeBegin float64
	progressRangeEnd   float64

	environ map[string]string

	preChangeStatusHooks   map[string]func() error
	preChangeStatusHooksMu sync.Mutex

	afterChangedHooks   map[string]func() error
	afterChangedHooksMu sync.Mutex

	updateTyp system.UpdateType

	errLogPath []string

	initiator Initiator // source of trigger
}

// Initiator is the source of trigger
type Initiator uint

const (
	// initiatorUser User manually triggered
	initiatorUser Initiator = iota
	// initiatorAuto Automatically triggered
	initiatorAuto
)

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
		retry:     1,

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
	s, _, err := system.QueryPackageDownloadSize(system.AllInstallUpdate, j.Packages...)
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
		j.errLogPath = info.Error.ErrorLog
	}

	if info.Cancelable != j.Cancelable {
		changed = true
		j.Cancelable = info.Cancelable
		_ = j.emitPropChangedCancelable(info.Cancelable)
	}
	logger.Debugf("updateInfo %v <- %v\n", j, info)

	// TODO 下载时重复触发
	if info.Status == system.RunningStatus && j.realRunningHookFn != nil {
		j.realRunningHookFn()
	}

	var newProgress float64
	var shouldUpdateProgress bool

	if info.ResetProgress {
		newProgress = 0
		shouldUpdateProgress = true
	} else {
		newProgress = buildProgress(info.Progress, j.progressRangeBegin, j.progressRangeEnd)
		// Only update when new progress is greater than current progress
		shouldUpdateProgress = newProgress > j.Progress
	}

	if shouldUpdateProgress {
		// Update progress
		changed = true
		j.Progress = newProgress
		_ = j.emitPropChangedProgress(newProgress)
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
		err := TransitionJobState(j, info.Status)
		if err != nil {
			logger.Warningf("_UpdateInfo: %v\n", err)
			// 当success的hook报错时，需要将error内容传递给job，running的hook错误在StartSystemJob中处理（其他四类状态应该不会有hook报错的情况）
			if info.Status == system.SucceedStatus {
				var jobErr *system.JobError
				ok := errors.As(err, &jobErr)
				if ok {
					j.setError(jobErr)
					j.errLogPath = jobErr.ErrorLog
					_ = TransitionJobState(j, system.FailedStatus)
					// 当需要迁移到success时，Cancelable为false，当hook报错时，需要将Cancelable设置为true
					j.Cancelable = true
					_ = j.emitPropChangedCancelable(info.Cancelable)
					return true
				}
				// failed状态迁移放到 setError 后面,需要failed hook 上报错误信息
				_ = TransitionJobState(j, system.FailedStatus)
			}
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

func (j *Job) setError(e *system.JobError) {
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		logger.Warning(err)
		return
	}
	j.setPropDescription(string(jsonBytes))
}

func (j *Job) getPreHook(name string) func() error {
	j.preChangeStatusHooksMu.Lock()
	fn := j.preChangeStatusHooks[name]
	j.preChangeStatusHooksMu.Unlock()
	return fn
}

// 当success的hook报错时,需要在 updateInfo 处理error;running的hook错误在StartSystemJob中处理;其他四类状态应该不会有hook报错的情况.
func (j *Job) setPreHooks(hooks map[string]func() error) {
	j.preChangeStatusHooksMu.Lock()
	if j.preChangeStatusHooks == nil {
		j.preChangeStatusHooks = make(map[string]func() error)
	}
	if hooks != nil {
		for k, v := range hooks {
			j.preChangeStatusHooks[k] = v
		}
	}
	j.preChangeStatusHooksMu.Unlock()
}

func (j *Job) wrapPreHooks(appendHooks map[string]func() error) {
	j.preChangeStatusHooksMu.Lock()
	defer j.preChangeStatusHooksMu.Unlock()
	if j.preChangeStatusHooks == nil {
		j.preChangeStatusHooks = appendHooks
		return
	}
	for key, fn := range appendHooks {
		appendFn := fn
		f, ok := j.preChangeStatusHooks[key]
		if ok {
			j.preChangeStatusHooks[key] = func() error {
				err := f()
				if err != nil {
					return err
				}
				return appendFn()
			}
		} else {
			j.preChangeStatusHooks[key] = fn
		}
	}
}

func (j *Job) getAfterHook(name string) func() error {
	j.afterChangedHooksMu.Lock()
	fn := j.afterChangedHooks[name]
	j.afterChangedHooksMu.Unlock()
	return fn
}

// after hook中 success状态的hook不要返回error
func (j *Job) setAfterHooks(hooks map[string]func() error) {
	j.afterChangedHooksMu.Lock()
	if j.afterChangedHooks == nil {
		j.afterChangedHooks = make(map[string]func() error)
	}
	if hooks != nil {
		for k, v := range hooks {
			j.afterChangedHooks[k] = v
		}
	}
	j.afterChangedHooksMu.Unlock()
}

func (j *Job) wrapAfterHooks(appendHooks map[string]func() error) {
	j.afterChangedHooksMu.Lock()
	defer j.afterChangedHooksMu.Unlock()
	if j.afterChangedHooks == nil {
		j.afterChangedHooks = appendHooks
		return
	}
	for key, fn := range appendHooks {
		appendFn := fn
		f, ok := j.afterChangedHooks[key]
		if ok {
			j.afterChangedHooks[key] = func() error {
				err := f()
				if err != nil {
					return err
				}
				return appendFn()
			}
		} else {
			j.afterChangedHooks[key] = fn
		}
	}
}

func (j *Job) subRetryCount(toZero bool) {
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	if toZero {
		j.retry = 0
		return
	}
	if j.subRetryHookFn != nil {
		j.subRetryHookFn(j)
	}
	j.retry--
}

// HasStatus check if the job has the given status
func (j *Job) HasStatus(status system.Status) bool {
	j.PropsMu.RLock()
	defer j.PropsMu.RUnlock()
	return j.Status == status
}
