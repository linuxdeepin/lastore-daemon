package main

import (
	"pkg.deepin.io/lib/dbus"
)

func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "com.deepin.lastore",
		ObjectPath: "/com/deepin/lastore",
		Interface:  "com.deepin.lastore.Manager",
	}
}

func (m *Manager) updateJobList() {
	list := m.jobManager.List()
	jobChanged := len(list) != len(m.JobList)
	systemOnChanging := false

	for i, j2 := range list {
		if !j2.Cancelable {
			systemOnChanging = true
		}

		if jobChanged || (i < len(m.JobList) && j2 == m.JobList[i]) {
			continue
		}
		jobChanged = true

		if jobChanged && systemOnChanging {
			break
		}
	}

	if jobChanged {
		m.JobList = list
		dbus.NotifyChange(m, "JobList")

		m.updatableApps()
	}

	if systemOnChanging != m.SystemOnChanging {
		m.SystemOnChanging = systemOnChanging
		dbus.NotifyChange(m, "SystemOnChanging")
	}
}

func (m *Manager) updatableApps() {
	apps := UpdatableNames(m.b.UpgradeInfo())
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
		dbus.NotifyChange(m, "UpgradableApps")
	}
}

func DestroyJob(j *Job) {
	dbus.UnInstallObject(j)
}

func (j *Job) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "com.deepin.lastore",
		ObjectPath: "/com/deepin/lastore/Job" + j.Id,
		Interface:  "com.deepin.lastore.Job",
	}
}

func (u Updater) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "com.deepin.lastore",
		ObjectPath: "/com/deepin/lastore",
		Interface:  "com.deepin.lastore.Updater",
	}
}
