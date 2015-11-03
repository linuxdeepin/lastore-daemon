package main

import (
	"pkg.deepin.io/lib/dbus"
)

func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "org.deepin.lastore",
		ObjectPath: "/org/deepin/lastore",
		Interface:  "org.deepin.lastore.Manager",
	}
}

func (m *Manager) updateJobList() {
	list := m.jobManager.List()
	changed := len(m.JobList) != len(list)
	if !changed {
		for i, job := range list {
			if m.JobList[i] != job {
				changed = true
				break
			}
		}
	}
	if changed {
		m.JobList = list
		dbus.NotifyChange(m, "UpgradableApps")

		m.updatableApps()
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
		Dest:       "org.deepin.lastore",
		ObjectPath: "/org/deepin/lastore/Job" + j.Id,
		Interface:  "org.deepin.lastore.Job",
	}
}
