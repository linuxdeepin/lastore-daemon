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
	log "github.com/cihub/seelog"
	"internal/system"
	"pkg.deepin.io/lib/dbus"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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
	queues map[string]*JobQueue

	system system.System

	dispatchLock sync.Mutex

	notify  func()
	changed bool
}

func NewJobManager(api system.System, notifyFn func()) *JobManager {
	if api == nil {
		panic("NewJobManager with api=nil")
	}
	m := &JobManager{
		queues: make(map[string]*JobQueue),
		notify: notifyFn,
		system: api,
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
func (jm *JobManager) CreateJob(jobName string, jobType string, packages []string) (*Job, error) {
	if job := jm.findJobByType(jobType, packages); job != nil {
		switch job.Status {
		case system.FailedStatus, system.PausedStatus:
			return job, jm.MarkStart(job.Id)
		default:
			return job, nil
		}
	}

	var job *Job
	switch jobType {
	case system.DownloadJobType:
		job = NewJob(genJobId(jobType), jobName, packages, jobType, DownloadQueue)
	case system.InstallJobType:
		job = NewJob(genJobId(jobType), jobName, packages, system.DownloadJobType, DownloadQueue)
		job._InitProgressRange(0, 0.5)

		next := NewJob(genJobId(jobType), jobName, packages, jobType, SystemChangeQueue)
		next._InitProgressRange(0.5, 1)

		job.Id = next.Id
		job.next = next
	case system.RemoveJobType:
		job = NewJob(genJobId(jobType), jobName, packages, jobType, SystemChangeQueue)
	case system.UpdateSourceJobType:
		job = NewJob(genJobId(jobType), jobName, nil, jobType, LockQueue)
	case system.DistUpgradeJobType:
		job = NewJob(genJobId(jobType), jobName, packages, jobType, LockQueue)
	case system.PrepareDistUpgradeJobType:
		job = NewJob(genJobId(jobType), jobName, packages, system.DownloadJobType, DownloadQueue)
	case system.UpdateJobType:
		job = NewJob(genJobId(jobType), jobName, packages, jobType, SystemChangeQueue)

	case system.CleanJobType:
		job = NewJob(genJobId(jobType), jobName, packages, jobType, LockQueue)
	default:
		return nil, system.NotSupportError
	}

	log.Infof("CreateJob with %q %q %q\n", jobName, jobType, packages)
	jm.addJob(job)
	return job, jm.MarkStart(job.Id)
}

// MarkStart transition the Job status to ReadyStatus
// and move the it to the head of queue.
func (jm *JobManager) MarkStart(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError("MarkStart " + jobId)
	}

	if job.Status != system.ReadyStatus {
		err := TransitionJobState(job, system.ReadyStatus)
		if err != nil {
			return err
		}
	}

	queue, ok := jm.queues[job.queueName]
	if !ok {
		return system.NotFoundError("MarkStart in queues" + job.queueName)
	}
	return queue.Raise(jobId)
}

// CleanJob transition the Job status to EndStatus,
// so the job will be auto clean in next dispatch run.
func (jm *JobManager) CleanJob(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError("CleanJob " + jobId)
	}

	if job.Cancelable && job.Status == system.RunningStatus {
		err := jm.PauseJob(jobId)
		if err != nil {
			return err
		}
	}

	if ValidTransitionJobState(job.Status, system.EndStatus) {
		job.next = nil
	}
	return TransitionJobState(job, system.EndStatus)
}

// PauseJob try aborting the job and transition the status to PauseStatus
func (jm *JobManager) PauseJob(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError("PauseJob jobId")
	}
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

// Dispatch transition Job status in Job Queues
// 1. Clean Jobs whose status is system.EndStatus
// 2. Run all Pending Jobs.
func (jm *JobManager) dispatch() {
	jm.dispatchLock.Lock()
	defer jm.dispatchLock.Unlock()

	var pendingDeleteJobs []*Job
	for _, queue := range jm.queues {
		// 1. Clean Jobs with EndStatus
		pendingDeleteJobs = append(pendingDeleteJobs, queue.DoneJobs()...)
	}
	for _, job := range pendingDeleteJobs {
		jm.changed = true
		jm.removeJob(job.Id, job.queueName)
		if job.next != nil {
			log.Debugf("Job(%q).next is %v\n", job.Id, job.next)
			job = job.next

			jm.addJob(job)

			jm.MarkStart(job.Id)

			job.notifyAll()
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

	if jm.changed && jm.notify != nil {
		jm.changed = false
		jm.notify()
	}
}

func (jm *JobManager) startJobsInQueue(queue *JobQueue) {
	jobs := queue.PendingJobs()
	for _, job := range jobs {
		jm.changed = true
		if job.Status == system.FailedStatus {
			jm.MarkStart(job.Id)
			log.Infof("Retry failed Job %v\n", job)
		}
		err := StartSystemJob(jm.system, job)
		if err != nil {
			TransitionJobState(job, system.FailedStatus)
			log.Errorf("StartSystemJob failed %v :%v\n", job, err)
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
	err = dbus.InstallOnSystem(j)
	if err != nil {
		return err
	}

	jm.changed = true
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
	jm.changed = true
	return nil
}

func (jm *JobManager) handleJobProgressInfo(info system.JobProgressInfo) {
	j := jm.findJobById(info.JobId)
	if j == nil {
		log.Warnf("Can't find Job %q when update info %v\n", info.JobId, info)
		return
	}

	if j._UpdateInfo(info) {
		jm.changed = true
	}
	jm.dispatch()
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
