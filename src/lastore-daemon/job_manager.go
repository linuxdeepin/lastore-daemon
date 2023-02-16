// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"internal/system"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	DownloadQueue        = "download"
	DownloadQueueCap     = 4
	SystemChangeQueue    = "system change"
	SystemChangeQueueCap = 1

	// LockQueue is special. All other queue must wait for LockQueue be emptied.
	LockQueue = "lock"

	// DelayLockQueue 特殊的队列,用于存放lock队列完成后执行的job
	DelayLockQueue = "delayLock"
)

// JobManager
// 1. maintain DownloadQueue and SystemchangeQueue
// 2. Create, Delete and Pause Jobs and schedule they.
type JobManager struct {
	service *dbusutil.Service
	queues  map[string]*JobQueue

	system system.System

	mux     sync.RWMutex
	changed bool

	dispatchMux sync.Mutex
	notify      func()
}

func NewJobManager(service *dbusutil.Service, api system.System, notifyFn func()) *JobManager {
	if api == nil {
		panic("NewJobManager with api=nil")
	}
	m := &JobManager{
		service: service,
		queues:  make(map[string]*JobQueue),
		notify:  notifyFn,
		system:  api,
	}
	m.createJobList(DownloadQueue, DownloadQueueCap)
	m.createJobList(SystemChangeQueue, SystemChangeQueueCap)
	m.createJobList(LockQueue, 1)
	m.createJobList(DelayLockQueue, 1)

	api.AttachIndicator(m.handleJobProgressInfo)
	return m
}

func (jm *JobManager) List() JobList {
	var r JobList
	for _, queue := range jm.queues {
		r = append(r, queue.AllJobs()...)
	}
	sort.Sort(r)
	return r
}

// CreateJob create the job and try starting it
func (jm *JobManager) CreateJob(jobName, jobType string, packages []string, environ map[string]string) (bool, *Job, error) {
	if job := jm.findJobByType(jobType, packages); job != nil {
		switch job.Status {
		case system.FailedStatus:
			return true, job, jm.markStart(job)
		default:
			return true, job, nil
		}
	}
	var job *Job
	switch jobType {
	case system.DownloadJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.DownloadJobType, DownloadQueue, environ)
	case system.PrepareDistUpgradeJobType,
		system.PrepareSystemUpgradeJobType,
		system.PrepareAppStoreUpgradeJobType,
		system.PrepareSecurityUpgradeJobType,
		system.PrepareUnknownUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.PrepareDistUpgradeJobType, DownloadQueue, environ)
	case system.InstallJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.DownloadJobType,
			DownloadQueue, environ)
		job._InitProgressRange(0, 0.5)

		next := NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, SystemChangeQueue, environ)
		next._InitProgressRange(0.5, 1)

		job.Id = next.Id
		job.next = next
	case system.OnlyInstallJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.InstallJobType, DelayLockQueue, environ)
	case system.RemoveJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, SystemChangeQueue, environ)
	case system.UpdateSourceJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, nil, jobType, LockQueue, environ)
	case system.DistUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, LockQueue, environ)
		job._InitProgressRange(0, 0.99)
	case system.UpdateJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, SystemChangeQueue, environ)
	case system.CleanJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, LockQueue, environ)
	case system.FixErrorJobType:
		var errType string
		if len(packages) >= 1 {
			errType = packages[0]
		}
		jobId := jobType + "_" + errType
		job = NewJob(jm.service, jobId, jobName, packages, jobType,
			LockQueue, environ)
	case system.SystemUpgradeJobType,
		system.SecurityUpgradeJobType,
		system.UnknownUpgradeJobType,
		system.AppStoreUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.DistUpgradeJobType, LockQueue, environ)
		job._InitProgressRange(0, 0.99)
	default:
		return false, nil, system.NotSupportError
	}

	logger.Infof("CreateJob with %q %q %q %+v\n", jobName, jobType, packages, environ)
	return false, job, jm.markReady(job)
}

func (jm *JobManager) markStart(job *Job) error {
	jm.markDirty()

	job.PropsMu.Lock()
	if job.Status != system.ReadyStatus {
		err := TransitionJobState(job, system.ReadyStatus, false)
		if err != nil {
			job.PropsMu.Unlock()
			return err
		}
	}
	job.PropsMu.Unlock()

	queue, ok := jm.queues[job.queueName]
	if !ok {
		return system.NotFoundError("MarkStart in queues" + job.queueName)
	}
	return queue.Raise(job.Id)
}

// 在无需将job的执行顺序提前,但是需要标记job状态时,使用markReady
func (jm *JobManager) markReady(job *Job) error {
	jm.markDirty()

	job.PropsMu.Lock()
	if job.Status != system.ReadyStatus {
		err := TransitionJobState(job, system.ReadyStatus, false)
		if err != nil {
			job.PropsMu.Unlock()
			return err
		}
	}
	job.PropsMu.Unlock()
	return nil
}

// markReload 当job的参数需要变化时,标记为reload,重新执行reload的hook完成修改
func (jm *JobManager) markReload(job *Job) error {
	jm.markDirty()
	job.PropsMu.Lock()
	if job.Status != system.ReloadStatus {
		err := TransitionJobState(job, system.ReloadStatus, true)
		if err != nil {
			job.PropsMu.Unlock()
			return err
		}
	}
	job.PropsMu.Unlock()
	queue, ok := jm.queues[job.queueName]
	if !ok {
		return system.NotFoundError("MarkStart in queues" + job.queueName)
	}
	return queue.Raise(job.Id)
}

// MarkStart transition the Job status to ReadyStatus
// and move the it to the head of queue.
func (jm *JobManager) MarkStart(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError("MarkStart " + jobId)
	}

	return jm.markStart(job)
}

// CleanJob transition the Job status to EndStatus,
// so the job will be auto clean in next dispatch run.
func (jm *JobManager) CleanJob(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError("CleanJob " + jobId)
	}

	job.PropsMu.Lock()
	defer job.PropsMu.Unlock()

	if job.Cancelable && job.Status == system.RunningStatus {
		err := jm.pauseJob(job)
		if err != nil {
			return err
		}
	}

	if ValidTransitionJobState(job.Status, system.EndStatus) {
		job.next = nil
	}
	return TransitionJobState(job, system.EndStatus, false)
}

func (jm *JobManager) pauseJob(job *Job) error {
	switch job.Status {
	case system.PausedStatus:
		logger.Warningf("Try pausing a paused Job %v\n", job)
		return nil
	case system.RunningStatus:
		err := jm.system.Abort(job.Id)
		if err != nil {
			return err
		}
	}

	return TransitionJobState(job, system.PausedStatus, job.needReload)
}

// PauseJob try aborting the job and transition the status to PauseStatus
func (jm *JobManager) PauseJob(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError("PauseJob jobId")
	}
	job.PropsMu.Lock()
	err := jm.pauseJob(job)
	job.PropsMu.Unlock()
	return err
}

// Dispatch transition Job status in Job Queues
// 1. Clean Jobs whose status is system.EndStatus
// 2. Run all Pending Jobs.
func (jm *JobManager) dispatch() {
	jm.dispatchMux.Lock()
	defer jm.dispatchMux.Unlock()
	var pendingDeleteJobs []*Job
	for _, queue := range jm.queues {
		// 1. Clean Jobs with EndStatus
		pendingDeleteJobs = append(pendingDeleteJobs, queue.DoneJobs()...)
	}

	for _, job := range pendingDeleteJobs {
		_ = jm.removeJob(job.Id, job.queueName)
		if job.next != nil {
			logger.Infof("Job(%q).next is %v\n", job.Id, job.next)
			job = job.next

			jm.dispatchMux.Unlock()
			_ = jm.addJob(job)
			jm.dispatchMux.Lock()

			_ = jm.markStart(job)
			job.PropsMu.RLock()
			job.notifyAll()
			job.PropsMu.RUnlock()
		}
	}

	// 2. Try starting jobs with ReadyStatus
	lockQueue := jm.queues[LockQueue]
	jm.startJobsInQueue(lockQueue)
	jm.startJobsInQueue(jm.queues[DownloadQueue]) // 由于下载任务不会影响到安装和更新,可以在更新时继续下载
	jm.startJobsInQueue(jm.queues[DelayLockQueue])
	// wait for LockQueue be idled
	if len(lockQueue.RunningJobs()) == 0 {
		jm.startJobsInQueue(jm.queues[SystemChangeQueue])
	}

	jm.sendNotify()
}

func (jm *JobManager) markDirty() {
	jm.mux.Lock()
	jm.changed = true
	jm.mux.Unlock()
}
func (jm *JobManager) sendNotify() {
	if jm.notify == nil {
		return
	}

	jm.mux.Lock()
	changed := jm.changed
	if changed {
		jm.changed = false
	}
	jm.mux.Unlock()

	if changed {
		jm.notify()
	}
}

func (jm *JobManager) startJobsInQueue(queue *JobQueue) {
	if NotUseDBus {
		return
	}
	jobs := queue.PendingJobs()
	for _, job := range jobs {
		job.PropsMu.RLock()
		jobStatus := job.Status
		jobLastStatus := job.lastStatus
		job.PropsMu.RUnlock()

		if jobStatus == system.FailedStatus {
			job.retry--
			_ = jm.markStart(job)
			logger.Infof("Retry failed Job %v\n", job)
		}

		if job.needReload {
			logger.Infof("need reload  %v job status:%v", job.Id, jobStatus)
			switch jobStatus {
			case system.RunningStatus:
				err := jm.pauseJob(job)
				if err != nil {
					logger.Warning(err)
					continue
				}
				job.PropsMu.Lock()
				job.lastStatus = system.RunningStatus
				job.PropsMu.Unlock()
				continue
			case system.FailedStatus, system.PausedStatus, system.ReadyStatus:
				job.PropsMu.Lock()
				job.lastStatus = jobStatus
				job.PropsMu.Unlock()
				err := jm.markReload(job)
				if err != nil {
					logger.Warning(err)
					continue
				}
				job.needReload = false
				continue
			}
		}
		if jobStatus == system.ReloadStatus { // 对reload状态的job进行处理，还原至reload前的状态
			switch jobLastStatus {
			case system.FailedStatus:
				logger.Info("recover failed")
				job.PropsMu.Lock()
				err := TransitionJobState(job, system.FailedStatus, true)
				if err != nil {
					logger.Warning(err)
				}
				job.PropsMu.Unlock()
				continue
			case system.ReadyStatus:
				logger.Info("recover ready")
				err := jm.markReady(job)
				if err != nil {
					logger.Warning(err)
					continue
				}
			case system.RunningStatus:
				logger.Info("recover running(ready)")
				err := jm.markStart(job)
				if err != nil {
					logger.Warning(err)
					continue
				}
			case system.PausedStatus:
				logger.Info("recover paused")
				err := jm.pauseJob(job)
				if err != nil {
					logger.Warning(err)
				}
				continue
			}
		}
		err := StartSystemJob(jm.system, job)
		if err != nil {
			job.PropsMu.Lock()
			_ = TransitionJobState(job, system.FailedStatus, false)
			job.PropsMu.Unlock()
			logger.Errorf("StartSystemJob failed %v :%v\n", job, err)

			pkgSysErr, ok := err.(*system.PkgSystemError)
			if ok {
				// do not retry job
				job.retry = 0
				hookFn := job.getHook(string(system.FailedStatus))
				if hookFn != nil {
					hookFn()
				}
				job.PropsMu.Lock()
				job.setError(pkgSysErr)
				_ = job.emitPropChangedStatus(job.Status)
				job.PropsMu.Unlock()
			} else if job.retry == 0 {
				job.setError(&system.JobError{
					Type:   "unknown",
					Detail: "failed to start system job: " + err.Error(),
				})
			}
		}
	}
}

func (jm *JobManager) Dispatch() {
	for {
		<-time.After(time.Millisecond * 500)
		jm.dispatch()
	}
}

func (jm *JobManager) createJobList(name string, cap int) {
	list := NewJobQueue(name, cap)
	jm.queues[name] = list
}

func (jm *JobManager) addJob(j *Job) error {
	jm.dispatchMux.Lock()
	defer jm.dispatchMux.Unlock()
	if j == nil {
		return system.NotFoundError("addJob with nil")
	}
	queueName := j.queueName
	queue, ok := jm.queues[queueName]
	if !ok {
		return system.NotFoundError("addJob with queue " + queueName)
	}

	err := queue.Add(j)
	if err != nil {
		if strings.Contains(err.Error(), "exists job") {
			logger.Warning(err)
			return nil
		}
		return err
	}
	if !NotUseDBus {
		// use dbus
		err = jm.service.Export(j.getPath(), j)
		if err != nil {
			logger.Warning(err)
			return err
		}
	}
	jm.markDirty()
	return nil
}
func (jm *JobManager) removeJob(jobId string, queueName string) error {
	queue, ok := jm.queues[queueName]
	if !ok {
		return system.NotFoundError("removeJob queue " + queueName)
	}

	job, err := queue.Remove(jobId)
	if err != nil {
		return err
	}
	DestroyJobDBus(job)
	jm.markDirty()
	return nil
}

func (jm *JobManager) handleJobProgressInfo(info system.JobProgressInfo) {
	j := jm.findJobById(info.JobId)
	if j == nil {
		logger.Warningf("Can't find Job %q when update info %v\n", info.JobId, info)
		return
	}

	if j.updateInfo(info) {
		jm.markDirty()
	}
}

func (jm *JobManager) findJobById(jobId string) *Job {
	for _, queue := range jm.queues {
		job := queue.Find(jobId)
		if job != nil {
			return job
		}
	}
	return nil
}

func (jm *JobManager) findJobByType(jobType string, pkgs []string) *Job {
	pList := strings.Join(pkgs, "")
	for _, job := range jm.List() {
		job.PropsMu.RLock()
		if job.Id == jobType {
			job.PropsMu.RUnlock()
			return job
		}
		if job.Type == jobType && strings.Join(job.Packages, "") == pList {
			job.PropsMu.RUnlock()
			return job
		}
		if job.next == nil {
			job.PropsMu.RUnlock()
			continue
		}
		if job.next.Type == jobType && strings.Join(job.next.Packages, "") == pList {
			// Don't return the job.next.
			// It's not a workable Job before the Job finished.
			job.PropsMu.RUnlock()
			return job
		}
		job.PropsMu.RUnlock()
	}
	return nil
}

var genJobId = func() func(string) string {
	var __count = 0
	return func(jobType string) string {
		switch jobType {
		case system.PrepareDistUpgradeJobType, system.DistUpgradeJobType,
			system.UpdateSourceJobType, system.CleanJobType, system.PrepareSystemUpgradeJobType,
			system.PrepareAppStoreUpgradeJobType, system.PrepareSecurityUpgradeJobType, system.PrepareUnknownUpgradeJobType,
			system.SystemUpgradeJobType, system.AppStoreUpgradeJobType, system.SecurityUpgradeJobType, system.UnknownUpgradeJobType:
			return jobType
		default:
			__count++
			return strconv.Itoa(__count) + jobType
		}
	}
}()

// 分类更新任务需要创建Job的内容
type upgradeJobInfo struct {
	PrepareJobId   string // 下载Job的Id
	PrepareJobType string // 下载Job的类型
	UpgradeJobId   string // 更新Job的Id
	UpgradeJobType string // 更新Job的类型
}

// GetUpgradeInfoMap 更新种类和具体job类型的映射
func GetUpgradeInfoMap() map[system.UpdateType]upgradeJobInfo {
	return map[system.UpdateType]upgradeJobInfo{
		system.SystemUpdate: {
			PrepareJobId:   genJobId(system.PrepareSystemUpgradeJobType),
			PrepareJobType: system.PrepareSystemUpgradeJobType,
			UpgradeJobId:   genJobId(system.SystemUpgradeJobType),
			UpgradeJobType: system.DistUpgradeJobType,
		},
		system.SecurityUpdate: {
			PrepareJobId:   genJobId(system.PrepareSecurityUpgradeJobType),
			PrepareJobType: system.PrepareSecurityUpgradeJobType,
			UpgradeJobId:   genJobId(system.SecurityUpgradeJobType),
			UpgradeJobType: system.DistUpgradeJobType,
		},
		system.AppStoreUpdate: {
			PrepareJobId:   genJobId(system.PrepareAppStoreUpgradeJobType),
			PrepareJobType: system.PrepareAppStoreUpgradeJobType,
			UpgradeJobId:   genJobId(system.AppStoreUpgradeJobType),
			UpgradeJobType: system.InstallJobType,
		},
		system.UnknownUpdate: {
			PrepareJobId:   genJobId(system.PrepareUnknownUpgradeJobType),
			PrepareJobType: system.PrepareUnknownUpgradeJobType,
			UpgradeJobId:   genJobId(system.UnknownUpgradeJobType),
			UpgradeJobType: system.DistUpgradeJobType,
		},
		system.OnlySecurityUpdate: {
			PrepareJobId:   genJobId(system.PrepareSecurityUpgradeJobType),
			PrepareJobType: system.PrepareSecurityUpgradeJobType,
			UpgradeJobId:   genJobId(system.SecurityUpgradeJobType),
			UpgradeJobType: system.DistUpgradeJobType,
		},
	}
}
