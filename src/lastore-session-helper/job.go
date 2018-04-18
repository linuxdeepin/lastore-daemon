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
	"fmt"
	"strings"
	"time"

	"dbus/com/deepin/lastore"
	"dbus/com/deepin/system/power"
	"dbus/org/freedesktop/login1"

	"internal/system"

	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/gettext"

	log "github.com/cihub/seelog"
)

type CacheJobInfo struct {
	Id       string
	Status   system.Status
	Name     string
	Progress float64
	Type     string
}

type Lastore struct {
	jobStatus map[dbus.ObjectPath]CacheJobInfo
	lang      string
	onLine    bool
	inhibitFd dbus.UnixFD

	power   *power.Power
	core    *lastore.Manager
	updater *lastore.Updater

	notifiedBattery   bool
	updatablePackages []string
	laterUpgradeTimer *time.Timer

	// prop:
	SourceCheckEnabled bool
}

func NewLastore() (*Lastore, error) {
	l := &Lastore{
		jobStatus: make(map[dbus.ObjectPath]CacheJobInfo),
		inhibitFd: -1,
		lang:      QueryLang(),
	}

	if !FileExist(disableSourceCheckFile) {
		l.SourceCheckEnabled = true
	}

	log.Debugf("CurrentLang: %q\n", l.lang)
	power, err := power.NewPower("com.deepin.system.Power", "/com/deepin/system/Power")
	if err != nil {
		panic(fmt.Errorf("power.NewPower err: %v", err))
	}
	l.power = power

	core, err := lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		panic(fmt.Errorf("lastore.NewManager err: %v", err))
	}

	l.core = core

	updater, err := lastore.NewUpdater("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		panic(fmt.Errorf("lastore.NewUpdater err: %v", err))
	}
	l.updater = updater

	l.updateJobList(core.JobList.Get())
	l.updateUpdatableApps()
	l.online()
	l.monitorBatteryPersent()

	err = dbus.InstallOnSession(l)
	if err != nil {
		log.Warn("install on session failed:", err)
		return nil, err
	}

	go l.monitorSignal()
	return l, nil
}

func (l *Lastore) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "com.deepin.LastoreSessionHelper",
		ObjectPath: "/com/deepin/LastoreSessionHelper",
		Interface:  "com.deepin.LastoreSessionHelper",
	}
}

func (l *Lastore) monitorSignal() {
	con, err := dbus.SystemBus()
	if err != nil {
		log.Errorf("Can't get system bus: %v\n", err)
		return
	}
	ch := con.Signal()

	con.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.DBus.Properties',sender='com.deepin.lastore',member='PropertiesChanged'")
	err = con.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, "interface='org.freedesktop.DBus'").Store()

	for v := range ch {
		switch v.Name {
		case "org.freedesktop.DBus.Properties.PropertiesChanged":
			if len(v.Body) != 3 {
				continue
			}
			props, _ := v.Body[1].(map[string]dbus.Variant)
			switch ifc, _ := v.Body[0].(string); ifc {
			case "com.deepin.lastore.Job":
				l.updateCacheJobInfo(v.Path, props)
			case "com.deepin.lastore.Manager":
				if jobList, ok := props["JobList"]; ok {
					list, _ := jobList.Value().([]dbus.ObjectPath)
					l.updateJobList(list)
				}
			case "com.deepin.lastore.Updater":
				_, ok := props["UpdatableApps"]
				_, ok2 := props["UpdatablePackages"]
				if ok || ok2 {
					l.updateUpdatableApps()
				}
			}
		case "org.freedesktop.DBus.NameOwnerChanged":
			switch name, _ := v.Body[0].(string); name {
			case "com.deepin.lastore":
				newOnwer, _ := v.Body[2].(string)
				if newOnwer == "" {
					l.offline()
				} else {
					l.online()
				}
			}
		default:
			continue
		}

	}
}

// updateUpdatableApps compare apps with record values
// 1. if find new app in apps notify it.
// 2. update record values
func (l *Lastore) updateUpdatableApps() {
	log.Info("on updatable apps and packages change")
	apps := l.updater.UpdatableApps.Get()
	packages := l.updater.UpdatablePackages.Get()

	log.Info("apps:", apps)
	log.Info("packages:", packages)

	if strSliceSetEqual(packages, l.updatablePackages) {
		// no change
		return
	}

	// change
	l.updatablePackages = packages
}

// updateJobList clean invalid cached Job status
// The list is the newest JobList.
func (l *Lastore) updateJobList(list []dbus.ObjectPath) {
	var invalids []dbus.ObjectPath
	for jobPath := range l.jobStatus {
		safe := false
		for _, p := range list {
			if p == jobPath {
				safe = true
				break
			}
		}
		if !safe {
			invalids = append(invalids, jobPath)
		}
	}
	for _, jobPath := range invalids {
		delete(l.jobStatus, jobPath)
	}
	log.Infof("UpdateJobList: %v - %v\n", list, invalids)
}

func TryFetchProperty(getter func() (interface{}, error), propName string, props map[string]dbus.Variant) (interface{}, bool) {
	if v, ok := props[propName]; ok {
		return v.Value(), true
	}
	if getter == nil {
		return nil, false
	}
	v, err := getter()
	if err != nil {
		return nil, false
	}
	return v, true
}

func (l *Lastore) updateCacheJobInfo(path dbus.ObjectPath, props map[string]dbus.Variant) CacheJobInfo {
	info := l.jobStatus[path]
	oldStatus := info.Status

	job, _ := lastore.NewJob("com.deepin.lastore", path)

	if v, ok := TryFetchProperty(job.Id.GetValue, "Id", props); ok {
		if rv, _ := v.(string); rv != "" {
			info.Id = rv
		}
	}

	if v, ok := TryFetchProperty(job.Status.GetValue, "Status", props); ok {
		if rv, _ := v.(string); rv != "" {
			info.Status = system.Status(rv)
		}
	}
	if v, ok := TryFetchProperty(job.Name.GetValue, "Name", props); ok {
		name, _ := v.(string)
		if name == "" {
			if pv, ok := TryFetchProperty(job.Packages.GetValue, "Packages", props); ok {
				pkgs, _ := pv.([]string)
				if len(pkgs) == 0 {
					name = "unknown"
				} else {
					name = PackageName(pkgs[0], l.lang)
				}
			}
		}
		if name != "" {
			info.Name = name
		}
	}
	if v, ok := TryFetchProperty(job.Progress.GetValue, "Progress", props); ok {
		rv, _ := v.(float64)
		info.Progress = rv
	}
	if v, ok := TryFetchProperty(job.Type.GetValue, "Type", props); ok {
		if rv, _ := v.(string); rv != "" {
			info.Type = rv
		}
	}

	l.jobStatus[path] = info
	log.Debugf("updateCacheJobInfo: %v\n", l.jobStatus[path])
	if oldStatus != info.Status {
		l.notifyJob(path)
	}
	return l.jobStatus[path]
}

func (l *Lastore) offline() {
	log.Info("Lastore.Daemon Offline\n")
	l.onLine = false
	l.jobStatus = make(map[dbus.ObjectPath]CacheJobInfo)
}

func (l *Lastore) online() {
	log.Info("Lastore.Daemon Online\n")
	l.onLine = true
}

func (l *Lastore) createUpgradeActions() []Action {
	return []Action{
		Action{
			Id:   "reboot",
			Name: gettext.Tr("Reboot Now"),
			Callback: func() {
				m, err := login1.NewManager("org.freedesktop.login1", "/org/freedesktop/login1")
				if err != nil {
					log.Warnf("Can't create login1 proxy: %v\n", err)
					return
				}
				defer login1.DestroyManager(m)
				m.Reboot(true)

			},
		},
		Action{
			Id:   "default",
			Name: gettext.Tr("Reboot Later"),
		},
	}
}

func (l *Lastore) createJobFailedActions(jobId string) []Action {
	ac := []Action{
		Action{
			Id:   "retry",
			Name: gettext.Tr("Retry"),
			Callback: func() {
				err := l.core.StartJob(jobId)
				log.Infof("StartJob %q : %v\n", jobId, err)
			},
		},
		Action{
			Id:   "cancel",
			Name: gettext.Tr("Cancel"),
			Callback: func() {
				err := l.core.CleanJob(jobId)
				log.Infof("CleanJob %q : %v\n", jobId, err)
			},
		},
	}
	return ac
}

func (l *Lastore) notifyJob(path dbus.ObjectPath) {
	l.checkBattery()

	info := l.jobStatus[path]
	status := info.Status
	log.Debugf("notifyJob: %q %q --> %v\n", path, status, info)
	switch guestJobTypeFromPath(path) {
	case system.InstallJobType:
		switch status {
		case system.FailedStatus:
			NotifyInstall(info.Name, false, l.createJobFailedActions(info.Id))
		case system.SucceedStatus:
			if info.Progress == 1 {
				NotifyInstall(info.Name, true, nil)
			}
		}
	case system.RemoveJobType:
		switch status {
		case system.FailedStatus:
			NotifyRemove(info.Name, false, l.createJobFailedActions(info.Id))
		case system.SucceedStatus:
			NotifyRemove(info.Name, true, nil)
		}

	case system.CleanJobType:
		if status == system.SucceedStatus {
			notifyAutoClean()
		}
	}
}

// guestJobTypeFromPath guest the JobType from object path
// We can't get the JobType when the DBusObject destroyed.
func guestJobTypeFromPath(path dbus.ObjectPath) string {
	_path := string(path)
	for _, jobType := range []string{
		// job types:
		system.InstallJobType, system.DownloadJobType, system.RemoveJobType,
		system.PrepareDistUpgradeJobType, system.DistUpgradeJobType, system.CleanJobType,
	} {
		if strings.Contains(_path, jobType) {
			return jobType
		}
	}
	return ""
}

func (*Lastore) IsDiskSpaceSufficient() (bool, error) {
	avail, err := queryVFSAvailable("/")
	if err != nil {
		return false, err
	}
	return avail > 1024*1024*10 /* 10 MB */, nil
}
