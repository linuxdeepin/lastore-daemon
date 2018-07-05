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
	"internal/system"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	DownloadQueue        = "download"
	DownloadQueueCap     = 3
	SystemChangeQueue    = "system change"
	SystemChangeQueueCap = 1

	// LockQueue is special. All other queue must wait for LockQueue be emptied.
	LockQueue = "lock"
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

	notify func()
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
func (jm *JobManager) CreateJob(jobName, jobType string, packages []string, environ map[string]string) (*Job, error) {
	if job := jm.findJobByType(jobType, packages); job != nil {
		switch job.Status {
		case system.FailedStatus, system.PausedStatus:
			return job, jm.markStart(job)
		default:
			return job, nil
		}
	}

	var job *Job
	switch jobType {
	case system.DownloadJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, DownloadQueue, environ)
	case system.InstallJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.DownloadJobType,
			DownloadQueue, environ)
		job._InitProgressRange(0, 0.5)

		next := NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, SystemChangeQueue, environ)
		next._InitProgressRange(0.5, 1)

		job.Id = next.Id
		job.next = next
	case system.RemoveJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, SystemChangeQueue, environ)
	case system.UpdateSourceJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, nil, jobType, LockQueue, environ)
	case system.DistUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, LockQueue, environ)
	case system.PrepareDistUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.DownloadJobType,
			DownloadQueue, environ)
	case system.UpdateJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, SystemChangeQueue, environ)

	case system.CleanJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, jobType, LockQueue, environ)
	case system.FixErrorJobType:
		errType := packages[0]
		jobId := jobType + "_" + errType
		job = NewJob(jm.service, jobId, jobName, packages, jobType,
			LockQueue, environ)
	default:
		return nil, system.NotSupportError
	}

	log.Infof("CreateJob with %q %q %q %+v\n", jobName, jobType, packages, environ)
	if err := jm.addJob(job); err != nil {
		return nil, err
	}
	return job, jm.markStart(job)
}

func (jm *JobManager) markStart(job *Job) error {
	jm.markDirty()

	job.PropsMu.Lock()
	if job.Status != system.ReadyStatus {
		err := TransitionJobState(job, system.ReadyStatus)
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
	return TransitionJobState(job, system.EndStatus)
}

func (jm *JobManager) pauseJob(job *Job) error {
	switch job.Status {
	case system.PausedStatus:
		log.Warnf("Try pausing a pasued Job %v\n", job)
		return nil
	case system.RunningStatus:
		err := jm.system.Abort(job.Id)
		if err != nil {
			return err
		}
	}

	return TransitionJobState(job, system.PausedStatus)
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
	var pendingDeleteJobs []*Job
	for _, queue := range jm.queues {
		// 1. Clean Jobs with EndStatus
		pendingDeleteJobs = append(pendingDeleteJobs, queue.DoneJobs()...)
	}

	for _, job := range pendingDeleteJobs {
		jm.removeJob(job.Id, job.queueName)
		if job.next != nil {
			log.Debugf("Job(%q).next is %v\n", job.Id, job.next)
			job = job.next

			jm.addJob(job)

			jm.markStart(job)
			job.PropsMu.RLock()
			job.notifyAll()
			job.PropsMu.RUnlock()
		}
	}

	// 2. Try starting jobs with ReadyStatus
	lockQueue := jm.queues[LockQueue]
	jm.startJobsInQueue(lockQueue)

	// wait for LockQueue be idled
	if len(lockQueue.RunningJobs()) == 0 {
		jm.startJobsInQueue(jm.queues[DownloadQueue])
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
	jobs := queue.PendingJobs()
	for _, job := range jobs {
		job.PropsMu.RLock()
		jobStatus := job.Status
		job.PropsMu.RUnlock()

		if jobStatus == system.FailedStatus {
			job.retry--
			jm.markStart(job)
			log.Infof("Retry failed Job %v\n", job)
		}

		err := StartSystemJob(jm.system, job)
		if err != nil {
			job.PropsMu.Lock()
			TransitionJobState(job, system.FailedStatus)
			job.PropsMu.Unlock()
			log.Errorf("StartSystemJob failed %v :%v\n", job, err)

			pkgSysErr, ok := err.(*system.PkgSystemError)
			if ok {
				// do not retry job
				job.retry = 0
				job.PropsMu.Lock()
				job.setError(pkgSysErr)
				job.emitPropChangedStatus(job.Status)
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
		return err
	}
	if !NotUseDBus {
		// use dbus
		err = jm.service.Export(j.getPath(), j)
		if err != nil {
			log.Warn(err)
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
		log.Warnf("Can't find Job %q when update info %v\n", info.JobId, info)
		return
	}

	if j._UpdateInfo(info) {
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
		if job.Id == jobType {
			return job
		}
		if job.Type == jobType && strings.Join(job.Packages, "") == pList {
			return job
		}
		if job.next == nil {
			continue
		}
		if job.next.Type == jobType && strings.Join(job.next.Packages, "") == pList {
			// Don't return the job.next.
			// It's not a workable Job before the Job finished.
			return job
		}
	}
	return nil
}

var genJobId = func() func(string) string {
	var __count = 0
	return func(jobType string) string {
		switch jobType {
		case system.PrepareDistUpgradeJobType, system.DistUpgradeJobType,
			system.UpdateSourceJobType, system.CleanJobType:
			return jobType
		default:
			__count++
			return strconv.Itoa(__count) + jobType
		}
	}
}()
