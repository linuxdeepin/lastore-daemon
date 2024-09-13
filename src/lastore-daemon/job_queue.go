// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"sort"
	"strings"
	"sync"
)

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
	Cap  int

	jobs JobList

	mux sync.RWMutex
}

func NewJobQueue(name string, cap int) *JobQueue {
	return &JobQueue{
		Name: name,
		Cap:  cap,
	}
}

func (l *JobQueue) AllJobs() JobList {
	l.mux.RLock()
	defer l.mux.RUnlock()

	r := make(JobList, len(l.jobs))
	copy(r, l.jobs)
	return r
}

// PendingJobs get the workable ready Jobs and recoverable failed Jobs
func (l *JobQueue) PendingJobs() JobList {
	l.mux.RLock()

	var numRunning int
	var readyJobs []*Job
	for _, job := range l.jobs {
		job.PropsMu.RLock()
		jobStatus := job.Status
		job.PropsMu.RUnlock()

		switch jobStatus {
		case system.FailedStatus:
			if job.retry > 0 {
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
		logger.Debug("These jobs are waiting for running...", readyJobs[n+1:])
	}
	r := JobList(readyJobs[:n])
	sort.Sort(r)

	l.mux.RUnlock()
	return r
}

func (l *JobQueue) DoneJobs() JobList {
	var ret JobList
	l.mux.RLock()
	for _, j := range l.jobs {
		j.PropsMu.RLock()
		jobStatus := j.Status
		j.PropsMu.RUnlock()
		if jobStatus == system.EndStatus {
			ret = append(ret, j)
		}
	}
	l.mux.RUnlock()
	return ret
}

func (l *JobQueue) RunningJobs() JobList {
	l.mux.RLock()
	var r JobList
	for _, job := range l.jobs {
		job.PropsMu.Lock()
		status := job.Status
		job.PropsMu.Unlock()
		if status == system.ReadyStatus || status == system.RunningStatus {
			r = append(r, job)
		}
	}
	l.mux.RUnlock()
	return r
}

func (l *JobQueue) Add(j *Job) error {
	if j == nil {
		return system.NotFoundError("addJob with nil")
	}
	l.mux.Lock()
	defer l.mux.Unlock()
	for _, job := range l.jobs {
		if job.Type == j.Type && strings.Join(job.Packages, "") == strings.Join(j.Packages, "") {
			if l.Name != SystemChangeQueue { // 如果不是应用安装任务,则需要判断job的Id是否一致
				if job.Id == j.Id {
					return fmt.Errorf("exists job %q:%q", job.Type, job.Packages)
				}
			} else {
				return fmt.Errorf("exists job %q:%q", job.Type, job.Packages)
			}
		}
	}
	l.jobs = append(l.jobs, j)
	sort.Sort(l.jobs)
	return nil
}

func (l *JobQueue) Remove(id string) (*Job, error) {
	l.mux.Lock()
	defer l.mux.Unlock()

	index := -1
	for i, job := range l.jobs {
		if job.Id == id {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, system.NotFoundError("JobQueue.Remove " + id)
	}

	job := l.jobs[index]

	l.jobs = append(l.jobs[0:index], l.jobs[index+1:]...)
	sort.Sort(l.jobs)
	return job, nil
}

// Raise raise the specify Job to head of JobList
// return system.NotFoundError if can't find the specify Job
func (l *JobQueue) Raise(jobId string) error {
	l.mux.Lock()
	defer l.mux.Unlock()

	var p int = -1
	for i, job := range l.jobs {
		if job.Id == jobId {
			p = i
			break
		}
	}
	if p == -1 {
		return system.NotFoundError("JobQueue.Raise " + jobId)
	}
	l.jobs.Swap(0, p)
	return nil
}

func (l *JobQueue) Find(id string) *Job {
	l.mux.RLock()
	defer l.mux.RUnlock()

	for _, job := range l.jobs {
		if job.Id == id {
			return job
		}
	}
	return nil
}
