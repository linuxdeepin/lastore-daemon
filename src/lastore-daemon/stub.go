// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"time"

	"github.com/godbus/dbus/v5"
)

var NotUseDBus = false

const (
	dbusInterfaceManager = "org.deepin.dde.Lastore1.Manager"
	dbusInterfaceJob     = "org.deepin.dde.Lastore1.Job"
	dbusInterfaceUpdater = "org.deepin.dde.Lastore1.Updater"
)

func (*Manager) GetInterfaceName() string {
	return dbusInterfaceManager
}

func (*Job) GetInterfaceName() string {
	return dbusInterfaceJob
}

func (*Updater) GetInterfaceName() string {
	return dbusInterfaceUpdater
}

func (m *Manager) updateJobList() {
	list := m.jobManager.List()
	var caller methodCaller

	m.PropsMu.RLock()
	jobChanged := len(list) != len(m.jobList)
	m.PropsMu.RUnlock()

	systemOnChanging := false

	for i, j2 := range list {

		j2.PropsMu.RLock()
		j2Cancelable := j2.Cancelable
		j2.PropsMu.RUnlock()
		if !j2Cancelable {
			systemOnChanging = true
			caller = j2.caller
		}

		if jobChanged {
			continue
		}

		m.PropsMu.RLock()
		shouldContinue := i < len(m.jobList) && j2 == m.jobList[i]
		m.PropsMu.RUnlock()
		if shouldContinue {
			// j2 （新）和 jobList[i] （旧）是相同的
			continue
		}

		jobChanged = true

		if systemOnChanging {
			break
		}
	}

	if jobChanged {
		m.jobList = list
		var jobPaths []dbus.ObjectPath
		for _, j := range list {
			jobPaths = append(jobPaths, j.getPath())
		}
		m.PropsMu.Lock()
		m.JobList = jobPaths
		_ = m.emitPropChangedJobList(jobPaths)
		m.PropsMu.Unlock()
	}

	if systemOnChanging != m.SystemOnChanging {
		m.SystemOnChanging = systemOnChanging
		m.updateSystemOnChanging(systemOnChanging, caller)
		_ = m.emitPropChangedSystemOnChanging(systemOnChanging)
	}
}

func (m *Manager) updatableApps(apps []string) {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	changed := len(apps) != len(m.UpgradableApps)
	if !changed {
		for i, app := range apps {
			if m.UpgradableApps[i] != app {
				changed = true
				break
			}
		}
	}
	if changed {
		m.UpgradableApps = apps
		err := m.emitPropChangedUpgradableApps(apps)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (u *Updater) setUpdatableApps(ids []string) {
	u.PropsMu.Lock()
	defer u.PropsMu.Unlock()

	changed := len(ids) != len(u.UpdatableApps)
	if !changed {
		for i, id := range ids {
			if u.UpdatableApps[i] != id {
				changed = true
				break
			}
		}
	}
	if changed {
		u.UpdatableApps = ids
		_ = u.emitPropChangedUpdatableApps(ids)
	}
}

func (u *Updater) setUpdatablePackages(ids []string) {
	u.PropsMu.Lock()
	defer u.PropsMu.Unlock()

	changed := len(ids) != len(u.UpdatablePackages)
	if !changed {
		for i, id := range ids {
			if u.UpdatablePackages[i] != id {
				changed = true
				break
			}
		}
	}
	if changed {
		u.UpdatablePackages = ids
		_ = u.emitPropChangedUpdatablePackages(ids)
	}
}

func DestroyJobDBus(j *Job) {
	if NotUseDBus {
		return
	}
	// 有next的job无需广播状态，会导致前端判断异常
	if j.next == nil {
		j.notifyAll()
	}
	<-time.After(time.Millisecond * 100)
	err := j.service.StopExport(j)
	if err != nil {
		logger.Warningf("failed to stop export job %q: %v", j.Id, err)
	}
}

func (j *Job) getPath() dbus.ObjectPath {
	return dbus.ObjectPath("/org/deepin/dde/Lastore1/Job" + j.Id)
}

func (j *Job) notifyAll() {
	_ = j.emitPropChangedType(j.Type)
	_ = j.emitPropChangedStatus(j.Status)
	_ = j.emitPropChangedProgress(j.Progress)
	_ = j.emitPropChangedSpeed(j.Speed)
	_ = j.emitPropChangedCancelable(j.Cancelable)
}
