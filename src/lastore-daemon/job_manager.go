package main

import (
	"fmt"
	"internal/system"
	"log"
	"sort"
	"time"
)

const (
	DownloadQueue        = "download"
	DownloadQueueCap     = 3
	SystemChangeQueue    = "system change"
	SystemChangeQueueCap = 1
)

// JobManager
// 1. 维护downloads 和 systemChanges 队列, 并隐藏其细节
// 2. 负责Job的创建以及其状态变更 (但不做实际的任务处理)
type JobManager struct {
	queues map[string]*JobList
	notify func()
	system system.System
}

// CreateJob create the job and try starting it
func (m *JobManager) CreateJob(jobType string, packageId string) (*Job, error) {
	var job *Job
	switch jobType {
	case system.DownloadJobType:
		job = NewDownloadJob(packageId)
		m.addJob(job, DownloadQueue)
	case system.InstallJobType:
		job = NewInstallJob(packageId)
		m.addJob(job, SystemChangeQueue)
	case system.RemoveJobType:
		job = NewRemoveJob(packageId)
		m.addJob(job, SystemChangeQueue)
	case system.DistUpgradeJobType:
		job = NewDistUpgradeJob()
		m.addJob(job, SystemChangeQueue)
	case system.UpdateJobType:
		job = NewUpdateJob(packageId)
		m.addJob(job, SystemChangeQueue)
	}
	return job, m.StartJob(job.Id)
}

// StartJob 将对应Job调整到队列最最前端，并调用dispatch
func (m *JobManager) StartJob(jobId string) error {
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

// CleanJob 将对应Job的状态更改为EndStatus, 并调用dispatch
func (m *JobManager) CleanJob(jobId string) error {
	job := m.find(jobId)
	if job == nil {
		return system.NotFoundError
	}
	if !TransitionJobState(job, system.EndStatus) {
		return fmt.Errorf("Can't transition the status of Job %q from %q to %q", jobId, job.Status, system.EndStatus)
	}
	m.dispatch()
	return nil
}

// PauseJob abort对应Job的执行,将状态更改为PauseStatus, 并调用dispatch
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

func (m JobManager) find(jobId string) *Job {
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

// Dispatch 根据当前队列情况, 进行Job的状态自动变更
// 1. 清理所有状态为system.EndStatus的Job
// 2. 取出所有Pending的Job并开始执行
func (m *JobManager) dispatch() {
	for _, queue := range m.queues {
		// 1. Clean Jobs with EndStatus
		for _, job := range queue.Jobs {
			if job.Status == system.EndStatus {
				m.removeJob(job.Id)
				log.Printf("Clean Job %q with Status %q\n", job.Id, system.EndStatus)
			}
		}

		// 2. Try starting jobs with ReadyStatus
		jobs := queue.PendingJobs()
		for _, job := range jobs {
			StartSystemJob(m.system, job)
		}
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

func (m *JobManager) addJob(j *Job, queueName string) error {
	queue, ok := m.queues[queueName]
	if !ok {
		return system.NotFoundError
	}

	err := queue.Add(j)
	if err != nil {
		return err
	}
	m.notify()
	m.dispatch()
	return nil
}
func (m *JobManager) removeJob(jobId string) error {
	var err error
	for _, queue := range m.queues {
		err = queue.Remove(jobId)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	m.notify()
	m.dispatch()
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

func (l JobList) Find(id string) *Job {
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
