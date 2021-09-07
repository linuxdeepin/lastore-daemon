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
	"time"

	"github.com/godbus/dbus"
)

var NotUseDBus = false

func (*Manager) GetInterfaceName() string {
	return "com.deepin.lastore.Manager"
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

		if jobChanged && systemOnChanging {
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

func (m *Manager) updatableApps(info []system.UpgradeInfo) {
	apps := UpdatableNames(info)

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
	j.notifyAll()
	<-time.After(time.Millisecond * 100)
	err := j.service.StopExport(j)
	if err != nil {
		logger.Warningf("failed to stop export job %q: %v", j.Id, err)
	}
}

func (j *Job) getPath() dbus.ObjectPath {
	return dbus.ObjectPath("/com/deepin/lastore/Job" + j.Id)
}

func (*Job) GetInterfaceName() string {
	return "com.deepin.lastore.Job"
}

func (*Updater) GetInterfaceName() string {
	return "com.deepin.lastore.Updater"
}

func (j *Job) notifyAll() {
	_ = j.emitPropChangedType(j.Type)
	_ = j.emitPropChangedStatus(j.Status)
	_ = j.emitPropChangedProgress(j.Progress)
	_ = j.emitPropChangedSpeed(j.Speed)
	_ = j.emitPropChangedCancelable(j.Cancelable)
}
