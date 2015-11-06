package main

import (
	"fmt"
	"internal/system"
	"log"
	"sort"
	"sync"
	"time"
)

const (
	DownloadQueue        = "download"
	DownloadQueueCap     = 3
	SystemChangeQueue    = "system change"
	SystemChangeQueueCap = 1
)

// JobManager
// 1. maintain DownloadQueue and SystemchangeQueue
// 2. Create, Delete and Pause Jobs and schedule they.
type JobManager struct {
	queues map[string]*JobList

	system system.System

	dispatchLock sync.Mutex

	notify  func()
	changed bool
}

// CreateJob create the job and try starting it
func (m *JobManager) CreateJob(jobType string, packageId string) (*Job, error) {
	for _, job := range m.List() {
		if job.PackageId == packageId {
			if job.Type == jobType || (job.next != nil && job.next.Type == jobType) {
				return nil, system.ResourceExitError
			}
		}
	}

	var job *Job
	switch jobType {
	case system.DownloadJobType:
		job = NewDownloadJob(packageId)
	case system.InstallJobType:
		job = NewInstallJob(packageId)
	case system.RemoveJobType:
		job = NewRemoveJob(packageId)
	case system.DistUpgradeJobType:
		job = NewDistUpgradeJob()
	case system.UpdateJobType:
		job = NewUpdateJob(packageId)
	}
	m.addJob(job)
	return job, m.MarkStart(job.Id)
}

// MarkStart transition the Job status to ReadyStatus
// and move the it to the head of queue.
func (m *JobManager) MarkStart(jobId string) error {
	job := m.find(jobId)
	if job == nil {
		return system.NotFoundError
	}
	if !TransitionJobState(job, system.ReadyStatus) {
		return fmt.Errorf("Can't transition job %q's status from %q to %q\n", job.Id, job.Status, system.ReadyStatus)
	}

	var err error
	for _, queue := range m.queues {
		err = queue.Raise(jobId)
		if err == nil {
			return nil
		}
	}
	return err
}

// CleanJob transition the Job status to EndStatus,
// so the job will be auto clean in next dispatch run.
func (m *JobManager) CleanJob(jobId string) error {
	job := m.find(jobId)
	if job == nil {
		return system.NotFoundError
	}

	if ValidTransitionJobState(job.Status, system.EndStatus) {
		job.next = nil
	}
	if !TransitionJobState(job, system.EndStatus) {
		return fmt.Errorf("Can't transition the status of Job %q from %q to %q", jobId, job.Status, system.EndStatus)
	}
	return nil
}

// PauseJob try aborting the job and transition the status to PauseStatus
func (m *JobManager) PauseJob(jobId string) error {
	job := m.find(jobId)
	if job == nil {
		return system.NotFoundError
	}

	err := m.system.Abort(job.Id)
	if err != nil {
		return err
	}

	if !TransitionJobState(job, system.PausedStatus) {
		return fmt.Errorf("Can't transition the status of Job %q from %q to %q", jobId, job.Status, system.EndStatus)
	}

	return nil
}

func (m *JobManager) find(jobId string) *Job {
	for _, queue := range m.queues {
		job := queue.Find(jobId)
		if job != nil {
			return job
		}
	}
	return nil
}

func NewJobManager(api system.System, notifyFn func()) *JobManager {
	if api == nil || notifyFn == nil {
		panic("NewJobManager with api=nil, notifyFn=nil")
	}
	m := &JobManager{
		queues: make(map[string]*JobList),
		notify: notifyFn,
		system: api,
	}
	m.createJobList(DownloadQueue, DownloadQueueCap)
	m.createJobList(SystemChangeQueue, SystemChangeQueueCap)
	return m
}

func (m *JobManager) List() []*Job {
	var r []*Job
	for _, queue := range m.queues {
		for _, job := range queue.Jobs {
			r = append(r, job)
		}
	}
	return r
}

// Dispatch transition Job status in Job Queues
// 1. Clean Jobs whose status is system.EndStatus
// 2. Run all Pending Jobs.
func (m *JobManager) dispatch() {
	m.dispatchLock.Lock()
	defer m.dispatchLock.Unlock()

	var pendingDeleteJobs []*Job
	for _, queue := range m.queues {
		// 1. Clean Jobs with EndStatus
		for _, job := range queue.Jobs {
			if job.Status == system.EndStatus {
				pendingDeleteJobs = append(pendingDeleteJobs, job)
			}
		}
	}
	for _, job := range pendingDeleteJobs {
		m.removeJob(job.Id, job.queueName)
		if job.next != nil {
			job = job.next
			m.addJob(job)
			m.MarkStart(job.Id)
		}
	}
	for _, queue := range m.queues {
		// 2. Try starting jobs with ReadyStatus
		jobs := queue.PendingJobs()
		for _, job := range jobs {
			StartSystemJob(m.system, job)
		}
	}

	if m.changed && m.notify != nil {
		m.changed = false
		m.notify()
	}
}

func (m *JobManager) Dispatch() {
	for {
		<-time.After(time.Millisecond * 500)
		m.dispatch()
	}
}

func (m *JobManager) createJobList(name string, cap int) {
	list := NewJobList(name, cap)
	m.queues[name] = list
}

func (m *JobManager) addJob(j *Job) error {
	queueName := j.queueName
	queue, ok := m.queues[queueName]
	if !ok {
		return system.NotFoundError
	}

	err := queue.Add(j)
	if err != nil {
		return err
	}
	m.changed = true
	return nil
}
func (m *JobManager) removeJob(jobId string, queueName string) error {
	queue, ok := m.queues[queueName]
	if !ok {
		return system.NotFoundError
	}

	err := queue.Remove(jobId)
	if err != nil {
		return err
	}
	m.changed = true
	return nil
}

type JobList struct {
	Name string
	Jobs []*Job
	Cap  int
}

func NewJobList(name string, cap int) *JobList {
	return &JobList{
		Name: name,
		Cap:  cap,
	}
}

// PendingJob get the workable ready Jobs
func (l *JobList) PendingJobs() []*Job {
	var numRunning int
	var readyJobs []*Job
	for _, job := range l.Jobs {
		switch job.Status {
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
		log.Println("These jobs are waiting for running...", readyJobs[n+1:])
	}
	return readyJobs[:n]
}

func (l JobList) Len() int {
	return len(l.Jobs)
}
func (l JobList) Less(i, j int) bool {
	return l.Jobs[i].CreateTime < l.Jobs[j].CreateTime
}
func (l *JobList) Swap(i, j int) {
	l.Jobs[i], l.Jobs[j] = l.Jobs[j], l.Jobs[i]
}

func (l *JobList) Add(j *Job) error {
	for _, job := range l.Jobs {
		if job.PackageId == j.PackageId && job.Type == j.Type {
			return fmt.Errorf("exists job %q:%q", job.Type, job.PackageId)
		}
	}
	l.Jobs = append(l.Jobs, j)
	sort.Sort(l)
	return nil
}

func (l *JobList) Remove(id string) error {
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
	sort.Sort(l)
	return nil
}

// Raise raise the specify Job to head of JobList
// return system.NotFoundError if can't find the specify Job
func (l *JobList) Raise(jobId string) error {
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
	l.Swap(0, p)
	return nil
}

func (l *JobList) Find(id string) *Job {
	for _, job := range l.Jobs {
		if job.Id == id {
			return job
		}
	}
	return nil
}

func (m *JobManager) handleJobProgressInfo(info system.JobProgressInfo) {
	j := m.find(info.JobId)
	if j == nil {
		log.Printf("Can't find Job %q when update info %v\n", info.JobId, info)
		return
	}

	j.updateInfo(info)
}
