// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

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

	Type      string
	PolicyTyp int

	Status system.Status

	Progress    float64
	Description string

	// completed bytes per second
	Speed                   int64
	DeliverySpeed           int64
	Proto                   string
	hasDeliveryDownloadInfo bool
	speedMeter              SpeedMeter
	deliveryDownloads       map[string]*downloadSession

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

type downloadSession struct {
	URI             string
	UsedMirror      string
	FileName        string
	StartedAt       time.Time
	HeaderAt        time.Time
	FirstByteAt     time.Time
	Bytes           int64
	Size            int64
	ResumePoint     int64
	TransferStartUs int64
	TransferEndUs   int64
}

type completedDownload struct {
	URI             string
	UsedMirror      string
	FileName        string
	StartedAt       time.Time
	FirstByteAt     time.Time
	Bytes           int64
	Size            int64
	ResumePoint     int64
	TransferStartUs int64
	TransferEndUs   int64
	BytesKnown      bool
	Failed          bool
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

		option:            make(map[string]string),
		queueName:         queueName,
		retry:             1,
		DeliverySpeed:     -1,
		deliveryDownloads: make(map[string]*downloadSession),

		progressRangeBegin: 0,
		progressRangeEnd:   1,
		environ:            environ,
	}
	return j
}

func (j *Job) initDownloadSize(s float64) {
	size := int64(s)
	j.PropsMu.Lock()
	if j.DownloadSize == 0 {
		j.DownloadSize = size
		_ = j.emitPropChangedDownloadSize(size)
	}
	j.speedMeter.SetDownloadSize(size)
	j.PropsMu.Unlock()
}

func (j *Job) setUpdatePolicy(policyTyp updateplatform.UpdateTp) {
	j.PropsMu.Lock()
	policy := int(policyTyp)
	if j.PolicyTyp != policy {
		j.PolicyTyp = policy
		_ = j.emitPropChangedUpdatePolicy(policy)
	}
	j.PropsMu.Unlock()
}

func (j *Job) String() string {
	return fmt.Sprintf("Job{Id:%q:%q,Type:%q(%v,%v), %q(%.2f)}@%q",
		j.Id, j.Packages,
		j.Type, j.Cancelable, j.Status,
		j.Description, j.Progress, j.queueName,
	)
}

func (j *Job) updateDeliveryDownloadInfo(info system.JobDeliveryDownloadInfo) *completedDownload {
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()

	if info.Speed == 0 && info.Proto == "" && info.URI == "" && !info.IsFinished {
		return nil
	}

	j.DeliverySpeed = info.Speed
	j.hasDeliveryDownloadInfo = !info.IsFinished

	speed := info.Speed
	if speed < 0 {
		speed = 0
	}
	if speed != j.Speed {
		j.Speed = speed
		if j.service != nil {
			_ = j.emitPropChangedSpeed(speed)
		}
	}

	proto := normalizeDownloadProto(info.Proto)
	if proto != j.Proto {
		logger.Infof("emitPropChangedProto %s -> %s\n", j.Proto, proto)
		j.Proto = proto
		if j.service != nil {
			_ = j.emitPropChangedProto(proto)
		}
	}

	if j.deliveryDownloads == nil {
		j.deliveryDownloads = make(map[string]*downloadSession)
	}
	eventAt := info.EventAt
	if eventAt.IsZero() {
		eventAt = time.Now()
	}
	uri := info.URI
	if uri == "" && info.FileName != "" {
		for candidateURI, candidate := range j.deliveryDownloads {
			if candidate.FileName == info.FileName {
				uri = candidateURI
				break
			}
		}
	}
	// Some apt method versions emit a compact finish status containing only
	// IsFinish/Speed/Proto. When exactly one download is active, it is safe to
	// associate that status with the sole session; with multiple sessions we
	// must avoid attributing the result to the wrong mirror.
	if uri == "" && info.IsFinished && len(j.deliveryDownloads) == 1 {
		for candidateURI := range j.deliveryDownloads {
			uri = candidateURI
		}
	}
	if uri == "" {
		if info.IsFinished {
			logger.Warningf("cannot match finished download status for job %q: uri=%q filename=%q", j.Id, info.URI, info.FileName)
		}
		return nil
	}
	if info.MethodEvent == 103 {
		// Transfer the same acquisition to its redirect URI, without leaving
		// a duplicate session to be counted again when the job finishes.
		if session := j.deliveryDownloads[uri]; session != nil && info.NewURI != "" {
			delete(j.deliveryDownloads, uri)
			session.URI = info.NewURI
			j.deliveryDownloads[info.NewURI] = session
		}
		return nil
	}
	if info.IMS {
		delete(j.deliveryDownloads, uri)
		return nil
	}
	session := j.deliveryDownloads[uri]
	if session == nil {
		session = &downloadSession{URI: uri, StartedAt: eventAt}
		j.deliveryDownloads[uri] = session
	}
	if info.UsedMirror != "" {
		session.UsedMirror = info.UsedMirror
	}
	if info.FileName != "" {
		session.FileName = info.FileName
	}
	if info.DownloadSize > session.Size {
		session.Size = info.DownloadSize
	}
	if info.ResumePoint > session.ResumePoint {
		session.ResumePoint = info.ResumePoint
	}
	if info.DownloadedSize > session.Bytes {
		session.Bytes = info.DownloadedSize
	}
	// HTTP methods emit 200 after accepting response headers, immediately
	// before the body. Never start the clock at 201: small files may have no
	// intermediate progress events at all.
	if info.MethodEvent == 200 && session.HeaderAt.IsZero() {
		session.HeaderAt = eventAt
	}
	if info.MethodEvent == 200 && info.TransferStartUs > 0 {
		session.TransferStartUs = info.TransferStartUs
	}
	if info.TransferEndUs > 0 {
		session.TransferEndUs = info.TransferEndUs
	}
	if session.FirstByteAt.IsZero() && (!info.IsFinished &&
		(info.DownloadedSize > 0 || info.Speed > 0)) {
		session.FirstByteAt = eventAt
	}
	if !info.IsFinished {
		return nil
	}
	finished := &completedDownload{
		URI:             session.URI,
		UsedMirror:      session.UsedMirror,
		FileName:        session.FileName,
		StartedAt:       session.StartedAt,
		FirstByteAt:     session.FirstByteAt,
		Bytes:           session.Bytes,
		Size:            session.Size,
		ResumePoint:     session.ResumePoint,
		TransferStartUs: session.TransferStartUs,
		TransferEndUs:   session.TransferEndUs,
		BytesKnown:      session.Bytes > 0,
		Failed:          info.Failed,
	}
	if finished.FirstByteAt.IsZero() {
		finished.FirstByteAt = session.HeaderAt
	}
	delete(j.deliveryDownloads, uri)
	return finished
}

// finishActiveDownloads reports the sessions that never received a 201/400
// event, so a job that ends early still contributes its observed samples.
func (j *Job) finishActiveDownloads(failed bool) []*completedDownload {
	j.PropsMu.Lock()
	defer j.PropsMu.Unlock()
	finished := make([]*completedDownload, 0, len(j.deliveryDownloads))
	for uri, session := range j.deliveryDownloads {
		firstByte := session.FirstByteAt
		if firstByte.IsZero() {
			firstByte = session.HeaderAt
		}
		finished = append(finished, &completedDownload{
			URI: session.URI, UsedMirror: session.UsedMirror, FileName: session.FileName,
			StartedAt: session.StartedAt, FirstByteAt: firstByte,
			Bytes: session.Bytes, Size: session.Size,
			ResumePoint: session.ResumePoint, TransferStartUs: session.TransferStartUs,
			BytesKnown: session.Bytes > 0, Failed: failed,
		})
		delete(j.deliveryDownloads, uri)
	}
	return finished
}

func fallbackDownloadSize(filename string, expected int64) (int64, bool, string) {
	if filename == "" || !filepath.IsAbs(filename) {
		return 0, false, "filename is missing or not absolute"
	}
	filenames := []string{filename}
	// APT emits the URI Acquire Filename before it moves the completed file
	// from archives/partial/ into archives/. Try that final location when the
	// original path has disappeared by the time the Done event is handled.
	partial := string(filepath.Separator) + "partial" + string(filepath.Separator)
	if index := strings.LastIndex(filename, partial); index >= 0 {
		final := filename[:index] + string(filepath.Separator) + filename[index+len(partial):]
		if final != filename {
			filenames = append(filenames, final)
		}
	}
	var lastErr error
	for _, candidate := range filenames {
		info, err := os.Lstat(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return 0, false, fmt.Sprintf("%q is not a regular file", candidate)
		}
		if info.Size() <= 0 {
			return 0, false, fmt.Sprintf("%q has zero size", candidate)
		}
		if expected > 0 && info.Size() != expected {
			return 0, false, fmt.Sprintf("%q size %d does not match expected %d", candidate, info.Size(), expected)
		}
		return info.Size(), true, ""
	}
	return 0, false, fmt.Sprintf("stat %q: %v", filename, lastErr)
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

	if !j.hasDeliveryDownloadInfo {
		if isDownloadProtocolJob(j.Type) && j.Proto != "http" {
			changed = true
			logger.Infof("change proto: %s -> http\n", j.Proto)
			j.Proto = "http"
			if j.service != nil {
				_ = j.emitPropChangedProto(j.Proto)
			}
		}

		// see the apt.go, we scale download progress value range in [0,0.5
		speed := j.speedMeter.Speed(info.Progress, j.DeliverySpeed)

		if speed != j.Speed {
			changed = true
			j.Speed = speed
			if j.service != nil {
				_ = j.emitPropChangedSpeed(speed)
			}
		}
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
		panic(fmt.Sprintf("Invalid Progress range init: begin: %.2f, end: %.2f", begin, end))
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

func normalizeDownloadProto(proto string) string {
	switch strings.ToLower(proto) {
	case "http", "https":
		return "http"
	case "p2p", "delivery":
		return "delivery"
	default:
		return proto
	}
}

func isDownloadProtocolJob(jobType string) bool {
	switch jobType {
	case system.DownloadJobType, system.PrepareDistUpgradeJobType, system.IncrementalDownloadJobType:
		return true
	default:
		return false
	}
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
