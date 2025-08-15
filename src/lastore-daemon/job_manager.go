// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
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

var JobExistError = errors.New("job is exist")

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

	jobDetailFn func(msg string)
}

func NewJobManager(service *dbusutil.Service, api system.System, notifyFn func(), jobDetailFn func(msg string)) *JobManager {
	if api == nil {
		panic("NewJobManager with api=nil")
	}
	m := &JobManager{
		service:     service,
		queues:      make(map[string]*JobQueue),
		notify:      notifyFn,
		system:      api,
		jobDetailFn: jobDetailFn,
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
func (jm *JobManager) CreateJob(jobName, jobType string, packages []string, environ map[string]string, jobArgc map[string]interface{}) (bool, *Job, error) {
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
	case system.PrepareSystemUpgradeJobType,
		system.PrepareAppStoreUpgradeJobType,
		system.PrepareSecurityUpgradeJobType,
		system.PrepareUnknownUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.PrepareDistUpgradeJobType, DownloadQueue, environ)
	case system.PrepareDistUpgradeJobType:
		var jobList []*Job
		mode, ok := jobArgc["UpdateMode"].(system.UpdateType)
		if !ok {
			return false, nil, fmt.Errorf("invalid arg %+v", jobArgc)
		}
		sizeMap, ok := jobArgc["DownloadSize"].(map[string]float64)
		if !ok {
			return false, nil, fmt.Errorf("invalid arg %+v", jobArgc)
		}
		packageMap, ok := jobArgc["PackageMap"].(map[string][]string)
		if !ok {
			return false, nil, fmt.Errorf("invalid arg %+v", jobArgc)
		}
		var allDownloadSize float64
		var holderSize float64
		for _, typ := range system.AllInstallUpdateType() {
			if typ&mode != 0 {
				allDownloadSize += sizeMap[typ.JobType()]
			}
		}
		for _, typ := range system.AllInstallUpdateType() {
			if typ&mode != 0 {
				// 使用dist-upgrade解决"有正在安装job时，依赖环境发生改变而导致检查依赖错误的问题"
				partJob := NewJob(jm.service, genJobId(jobType), jobName, packageMap[typ.JobType()], system.PrepareDistUpgradeJobType, DownloadQueue, environ)
				if utils.IsDir(system.GetCategorySourceMap()[typ]) {
					partJob.option = map[string]string{
						"Dir::Etc::SourceList":  "/dev/null",
						"Dir::Etc::SourceParts": system.GetCategorySourceMap()[typ],
					}
				} else {
					partJob.option = map[string]string{
						"Dir::Etc::SourceList":  system.GetCategorySourceMap()[typ],
						"Dir::Etc::SourceParts": "/dev/null",
					}
				}
				partJob.updateTyp = typ
				if len(jobList) >= 1 {
					jobList[len(jobList)-1].next = partJob
				}
				jobList = append(jobList, partJob)
				partJob._InitProgressRange(holderSize/allDownloadSize, (holderSize+sizeMap[typ.JobType()])/allDownloadSize)
				holderSize += sizeMap[typ.JobType()]
			}
		}
		job = jobList[0]

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
		job._InitProgressRange(0.11, 0.9)
	case system.DistUpgradeJobType:
		mode, ok := jobArgc["UpdateMode"].(system.UpdateType)
		if !ok {
			return false, nil, fmt.Errorf("invalid arg %+v", jobArgc)
		}
		supportIgnore, ok := jobArgc["SupportDpkgScriptIgnore"].(bool)
		if !ok {
			return false, nil, fmt.Errorf("invalid arg %+v", jobArgc)
		}

		var includeUnknown bool
		if mode&system.UnknownUpdate != 0 && mode != system.UnknownUpdate {
			// 不仅仅只存在第三方更新
			includeUnknown = true
		}
		if includeUnknown {
			// 非第三方job
			commonJob := NewJob(jm.service, genJobId(jobType), jobName, packages, system.DistUpgradeJobType, LockQueue, environ)
			commonJob._InitProgressRange(0, 0.70)
			commonJob.updateTyp = mode
			commonJob.retry = 0
			// 第三方job
			thirdJob := NewJob(jm.service, genJobId(jobType), jobName, packages, system.DistUpgradeJobType, LockQueue, environ)
			thirdJob._InitProgressRange(0.71, 0.99)
			thirdPath := system.GetCategorySourceMap()[system.UnknownUpdate]
			if utils.IsDir(thirdPath) {
				thirdJob.option = map[string]string{
					"Dir::Etc::SourceList":  "/dev/null",
					"Dir::Etc::SourceParts": thirdPath,
				}
			} else {
				thirdJob.option = map[string]string{
					"Dir::Etc::SourceList":  thirdPath,
					"Dir::Etc::SourceParts": "/dev/null",
				}
			}
			if supportIgnore {
				thirdJob.option["DPkg::Options::"] = "--script-ignore-error"
			}
			thirdJob.updateTyp = mode
			thirdJob.retry = 0
			commonJob.next = thirdJob
			job = commonJob
		} else {
			job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.DistUpgradeJobType, LockQueue, environ)
			job.updateTyp = mode
			job._InitProgressRange(0, 0.99)
		}

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
		job = NewJob(jm.service, jobId, jobName, packages, jobType, LockQueue, environ)
	// 分类更新接口触发
	case system.SystemUpgradeJobType,
		system.SecurityUpgradeJobType,
		system.UnknownUpgradeJobType,
		system.AppStoreUpgradeJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.InstallJobType, LockQueue, environ)
		job._InitProgressRange(0, 0.99)
	case system.CheckSystemJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, nil, system.CheckSystemJobType, SystemChangeQueue, environ)
	case system.BackupJobType:
		job = NewJob(jm.service, genJobId(jobType), jobName, packages, system.BackupJobType, LockQueue, environ)
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

// 在无需将job的执行顺序提前,但是需要标记job状态时,使用markReady
func (jm *JobManager) markReady(job *Job) error {
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
	return nil
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
		logger.Warningf("Try pausing a paused Job %v\n", job)
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

// ForceAbortAndRetry 终止该job，并将退出状态设置为failed
func (jm *JobManager) ForceAbortAndRetry(job *Job) error {
	job.PropsMu.Lock()
	defer job.PropsMu.Unlock()
	if job.Status == system.RunningStatus {
		if job.retry < 1 {
			job.retry = 1
		}
		err := jm.system.AbortWithFailed(job.Id)
		if err != nil {
			return err
		}
	}
	if job.Status == system.FailedStatus && job.retry < 1 {
		job.retry = 1
	}

	return nil
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
			// 部分属性需要继承
			if job.next.option == nil {
				job.next.option = make(map[string]string)
			}

			if job.option != nil {
				// 上一个job无论是否存在该配置，都需要传递给下一个job
				v, ok := job.option[aptLimitKey]
				if ok {
					job.next.option[aptLimitKey] = v
				} else {
					delete(job.next.option, aptLimitKey)
				}
			}
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
	jm.sendNotify()
	jm.startJobsInQueue(lockQueue)
	jm.startJobsInQueue(jm.queues[DelayLockQueue])
	jm.startJobsInQueue(jm.queues[DownloadQueue])
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
		job.PropsMu.RUnlock()

		if jobStatus == system.FailedStatus { // 将失败的job标记为ready
			job.subRetryCount(false)
			_ = jm.markStart(job)
			logger.Infof("Retry failed Job %v\n", job)
		}

		err := StartSystemJob(jm.system, job)
		if err != nil {
			logger.Errorf("StartSystemJob failed %v :%v\n", job, err)
			var jobErr *system.JobError
			ok := errors.As(err, &jobErr)
			if ok {
				// do not retry job
				job.subRetryCount(true) // retry 设置为 0
				job.PropsMu.Lock()
				job.setError(jobErr)
				job.errLogPath = jobErr.ErrorLog
				job.PropsMu.Unlock()
			} else if job.retry == 0 {
				job.PropsMu.Lock()
				job.setError(&system.JobError{
					ErrType:   system.ErrorUnknown,
					ErrDetail: "failed to start system job: " + err.Error(),
				})
				job.PropsMu.Unlock()
			}
			// failed状态迁移放到 setError 后面,需要failed hook 上报错误信息
			job.PropsMu.Lock()
			_ = TransitionJobState(job, system.FailedStatus)
			job.PropsMu.Unlock()
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
	if (j.Id == genJobId(system.UpdateSourceJobType)) && (len(jm.queues[DownloadQueue].RunningJobs()) != 0 || len(jm.queues[DelayLockQueue].RunningJobs()) != 0 || len(jm.queues[LockQueue].RunningJobs()) != 0) {
		return errors.New("download or install running, not need check update")
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
	logger.Infof("Add job with %q %q %q %+v %+v\n", j.Name, j.Type, j.Packages, j.option, j.environ)
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
	if jm.jobDetailFn != nil && len(info.OriginalLog) > 0 {
		jm.jobDetailFn(fmt.Sprintf("[%s] %s", time.Now().Format(time.DateTime), info.OriginalLog))
	}
	if info.OnlyLog {
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
		case system.PrepareDistUpgradeJobType, system.DistUpgradeJobType, system.BackupJobType,
			system.UpdateSourceJobType, system.CleanJobType, system.PrepareSystemUpgradeJobType,
			system.PrepareAppStoreUpgradeJobType, system.PrepareSecurityUpgradeJobType, system.PrepareUnknownUpgradeJobType,
			system.SystemUpgradeJobType, system.AppStoreUpgradeJobType, system.SecurityUpgradeJobType, system.UnknownUpgradeJobType, system.CheckSystemJobType:
			return jobType
		default:
			__count++
			return strconv.Itoa(__count) + jobType
		}
	}
}()
