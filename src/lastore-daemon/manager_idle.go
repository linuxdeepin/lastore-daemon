// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/godbus/dbus/v5"
)

const lastoreJobCacheJson = "/tmp/lastoreJobCache.json"

func (m *Manager) canAutoQuit() bool {
	m.PropsMu.RLock()
	jobList := m.jobList
	m.PropsMu.RUnlock()
	haveActiveJob := len(jobList) > 0
	// for _, job := range jobList {
	// 	if (job.Status != system.FailedStatus || job.retry > 0) && job.Status != system.PausedStatus {
	// 		logger.Info(job.Id)
	// 		haveActiveJob = true
	// 	}
	// }
	m.autoQuitCountMu.Lock()
	inhibitAutoQuitCount := m.inhibitAutoQuitCount
	m.autoQuitCountMu.Unlock()
	logger.Info("haveActiveJob", haveActiveJob)
	logger.Info("inhibitAutoQuitCount", inhibitAutoQuitCount)
	logger.Info("upgrade status:", m.config.UpgradeStatus.Status)
	return !haveActiveJob && inhibitAutoQuitCount == 0 && m.config.UpgradeStatus.Status != system.UpgradeRunning
}

type JobContent struct {
	Id   string
	Name string

	Packages     []string
	CreateTime   int64
	DownloadSize int64

	Type string

	Status system.Status

	Progress    float64
	Description string
	Environ     map[string]string
	// completed bytes per second
	QueueName string
	HaveNext  bool
}

// 读取上一次退出时失败和暂停的job,并导出
func (m *Manager) loadCacheJob() {
	var jobList []*JobContent
	jobContent, err := os.ReadFile(lastoreJobCacheJson)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = json.Unmarshal(jobContent, &jobList)
	if err != nil {
		logger.Warning(err)
		return
	}
	for _, j := range jobList {
		switch j.Status {
		case system.FailedStatus:
			failedJob := NewJob(m.service, j.Id, j.Name, j.Packages, j.Type, j.QueueName, j.Environ)
			failedJob.Description = j.Description
			failedJob.CreateTime = j.CreateTime
			failedJob.DownloadSize = j.DownloadSize
			failedJob.Status = j.Status
			failedJob.retry = 0
			err = m.jobManager.addJob(failedJob)
			if err != nil {
				logger.Warning(err)
				continue
			}
		case system.PausedStatus:
			_, err := m.prepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]), m.CheckUpdateMode, initiatorAuto)
			if err != nil {
				logger.Warning(err)
				return
			}
			pausedJob := m.jobManager.findJobById(j.Id)
			if pausedJob != nil {
				pausedJob.PropsMu.Lock()
				err := m.jobManager.pauseJob(pausedJob)
				if err != nil {
					logger.Warning(err)
				}
				pausedJob.Progress = j.Progress
				pausedJob.PropsMu.Unlock()
			}

		default:
			continue
		}
	}
}

// 保存失败和暂停的job内容
func (m *Manager) saveCacheJob() {
	m.PropsMu.RLock()
	jobList := m.jobList
	m.PropsMu.RUnlock()

	var needSaveJobs []*JobContent
	for _, job := range jobList {
		if (job.Status == system.FailedStatus && job.retry == 0) || job.Status == system.PausedStatus {
			haveNext := false
			if job.next != nil {
				haveNext = true
			}
			needSaveJob := &JobContent{
				job.Id,
				job.Name,
				job.Packages,
				job.CreateTime,
				job.DownloadSize,
				job.Type,
				job.Status,
				job.Progress,
				job.Description,
				job.environ,
				job.queueName,
				haveNext,
			}
			needSaveJobs = append(needSaveJobs, needSaveJob)
		}
	}
	b, err := json.Marshal(needSaveJobs)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = os.WriteFile(lastoreJobCacheJson, b, 0600)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) inhibitAutoQuitCountSub() {
	m.autoQuitCountMu.Lock()
	m.inhibitAutoQuitCount -= 1
	m.autoQuitCountMu.Unlock()
}

func (m *Manager) inhibitAutoQuitCountAdd() {
	m.autoQuitCountMu.Lock()
	m.inhibitAutoQuitCount += 1
	m.autoQuitCountMu.Unlock()
}

func (m *Manager) loadLastoreCache() {
	m.loadUpdateSourceOnce()
}

func (m *Manager) saveLastoreCache() {
	m.saveUpdateSourceOnce()
	m.userAgents.saveRecordContent(userAgentRecordPath)
}

func (m *Manager) handleOSSignal() {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGSEGV)

	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGSEGV:
			logger.Info("received signal:", sig)
			m.service.Quit()
		}
	}
}
