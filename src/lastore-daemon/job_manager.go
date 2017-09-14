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
	"fmt"
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
		for _, job := range queue.Jobs {
			r = append(r, job)
		}
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
		return system.NotFoundError
	}

	if job.Status != system.ReadyStatus {
		err := TransitionJobState(job, system.ReadyStatus)
		if err != nil {
			return err
		}
	}

	queue, ok := jm.queues[job.queueName]
	if !ok {
		return system.NotFoundError
	}
	return queue.Raise(jobId)
}

// CleanJob transition the Job status to EndStatus,
// so the job will be auto clean in next dispatch run.
func (jm *JobManager) CleanJob(jobId string) error {
	job := jm.findJobById(jobId)
	if job == nil {
		return system.NotFoundError
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
		return system.NotFoundError
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
		for _, job := range queue.Jobs {
			switch {
			case job.Status == system.EndStatus:
				pendingDeleteJobs = append(pendingDeleteJobs, job)
			}
		}
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
		log.Trace("adJob with nil")
		return system.NotFoundError
	}
	queueName := j.queueName
	queue, ok := jm.queues[queueName]
	if !ok {
		return system.NotFoundError
	}

	err := queue.Add(j)
	if err != nil {
		return err
	}

	jm.changed = true
	return nil
}
func (jm *JobManager) removeJob(jobId string, queueName string) error {
	queue, ok := jm.queues[queueName]
	if !ok {
		return system.NotFoundError
	}

	err := queue.Remove(jobId)
	if err != nil {
		return err
	}
	jm.changed = true
	return nil
}

type JobList []*Job

func (l JobList) Len() int {
	return len(l)
}
func (l JobList) Less(i, j int) bool {
	if l[i].Type == system.UpdateSourceJobType {
		return true
	}
	return l[i].CreateTime < l[j].CreateTime
}
func (l JobList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type JobQueue struct {
	Name string
	Jobs JobList
	Cap  int
}

func NewJobQueue(name string, cap int) *JobQueue {
	return &JobQueue{
		Name: name,
		Cap:  cap,
	}
}

// PendingJob get the workable ready Jobs and recoverable failed Jobs
func (l *JobQueue) PendingJobs() JobList {
	var numRunning int
	var readyJobs []*Job
	for _, job := range l.Jobs {
		switch job.Status {
		case system.FailedStatus:
			if job.retry > 0 {
				job.retry--
				readyJobs = append(readyJobs, job)
			}
		case system.RunningStatus:
			numRunning = numRunning + 1
		case system.ReadyStatus:
			readyJobs = append(readyJobs, job)
		}
	}
	space := l.Cap - numRunning
	numPending := len(readyJobs)

	var n int
	for space > 0 && numPending > 0 {
		space--
		numPending--
		n++
	}
	if n+1 < numPending {
		log.Trace("These jobs are waiting for running...", readyJobs[n+1:])
	}
	r := JobList(readyJobs[:n])
	sort.Sort(r)
	return r
}

func (l *JobQueue) RunningJobs() JobList {
	var r JobList
	for _, job := range l.Jobs {
		switch job.Status {
		case system.RunningStatus:
			r = append(r, job)
		}
	}
	return r
}

func (l *JobQueue) Add(j *Job) error {
	for _, job := range l.Jobs {
		if job.Type == j.Type && strings.Join(job.Packages, "") == strings.Join(j.Packages, "") {
			return fmt.Errorf("exists job %q:%q", job.Type, job.Packages)
		}
	}
	l.Jobs = append(l.Jobs, j)
	sort.Sort(l.Jobs)
	dbus.InstallOnSystem(j)
	return nil
}

func (l *JobQueue) Remove(id string) error {
	index := -1
	for i, job := range l.Jobs {
		if job.Id == id {
			index = i
			break
		}
	}
	if index == -1 {
		return system.NotFoundError
	}

	job := l.Jobs[index]
	DestroyJob(job)

	l.Jobs = append(l.Jobs[0:index], l.Jobs[index+1:]...)
	sort.Sort(l.Jobs)
	return nil
}

// Raise raise the specify Job to head of JobList
// return system.NotFoundError if can't find the specify Job
func (l *JobQueue) Raise(jobId string) error {
	var p int = -1
	for i, job := range l.Jobs {
		if job.Id == jobId {
			p = i
			break
		}
	}
	if p == -1 {
		return system.NotFoundError
	}
	l.Jobs.Swap(0, p)
	return nil
}

func (l *JobQueue) Find(id string) *Job {
	for _, job := range l.Jobs {
		if job.Id == id {
			return job
		}
	}
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
