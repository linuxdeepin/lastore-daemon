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

	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbus1"
)

var NotUseDBus = false

func (*Manager) GetInterfaceName() string {
	return "com.deepin.lastore.Manager"
}

func (m *Manager) updateJobList() {
	list := m.jobManager.List()
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	jobChanged := len(list) != len(m.jobList)
	systemOnChanging := false

	for i, j2 := range list {

		j2.PropsMu.RLock()
		j2Cancelable := j2.Cancelable
		j2.PropsMu.RUnlock()
		if !j2Cancelable {
			systemOnChanging = true
		}

		if jobChanged || (i < len(m.jobList) && j2 == m.jobList[i]) {
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
		m.JobList = jobPaths
		m.emitPropChangedJobList(jobPaths)
	}

	if systemOnChanging != m.SystemOnChanging {
		m.SystemOnChanging = systemOnChanging
		m.updateSystemOnChaning(systemOnChanging)
		m.emitPropChangedSystemOnChanging(systemOnChanging)
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
		m.emitPropChangedUpgradableApps(apps)
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
		u.emitPropChangedUpdatableApps(ids)
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
		u.emitPropChangedUpdatablePackages(ids)
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
		log.Warnf("failed to stop export job %q: %v", j.Id, err)
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
	j.emitPropChangedType(j.Type)
	j.emitPropChangedStatus(j.Status)
	j.emitPropChangedProgress(j.Progress)
	j.emitPropChangedSpeed(j.Speed)
	j.emitPropChangedCancelable(j.Cancelable)
}
