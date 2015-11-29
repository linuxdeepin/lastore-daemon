package main

import "dbus/org/freedesktop/login1"
import "dbus/com/deepin/lastore"
import "pkg.deepin.io/lib/dbus"
import "internal/system"
import "pkg.deepin.io/lib/gettext"
import "syscall"
import log "github.com/cihub/seelog"
import "strings"

type Lastore struct {
	JobStatus        map[dbus.ObjectPath]system.Status
	SystemOnChanging bool
	Lang             string
	OnLine           bool
	inhibitFd        dbus.UnixFD
	core             *lastore.Manager
	updater          *lastore.Updater
	notifiedBattery  bool
	updatableApps    []string
}

func NewLastore() *Lastore {
	l := &Lastore{
		JobStatus:        make(map[dbus.ObjectPath]system.Status),
		SystemOnChanging: true,
		inhibitFd:        -1,
		Lang:             QueryLang(),
	}
	core, err := lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		log.Warnf("NewLastore: %v\n", err)
	}
	l.core = core

	updater, err := lastore.NewUpdater("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		log.Warnf("NewLastore: %v\n", err)
	}
	l.updater = updater

	l.updateSystemOnChaning(core.SystemOnChanging.Get())
	l.updateJobList(core.JobList.Get())
	l.updateUpdatableApps(updater.UpdatableApps.Get())
	l.online()

	go l.monitorSignal()
	return l
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
				status, ok := props["Status"]
				if ok {
					svalue, _ := status.Value().(string)
					log.Infof("Job %s status change to %s\n", v.Path, svalue)
					l.updateJob(v.Path, system.Status(svalue))
				}
			case "com.deepin.lastore.Manager":
				if onChaning, ok := props["SystemOnChanging"]; ok {
					chaning, _ := onChaning.Value().(bool)
					l.updateSystemOnChaning(chaning)
				}

				if jobList, ok := props["JobList"]; ok {
					list, _ := jobList.Value().([]dbus.ObjectPath)
					l.updateJobList(list)
				}
			case "com.deepin.lastore.Updater":
				if variant, ok := props["UpdatableApps"]; ok {
					apps, _ := variant.Value().([]string)
					l.updateUpdatableApps(apps)
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
func (l *Lastore) updateUpdatableApps(apps []string) {
	for _, new := range apps {
		foundNew := false
		for _, old := range l.updatableApps {
			if new == old {
				foundNew = true
				break
			}
		}
		if !foundNew {
			NotifyNewUpdates(len(apps))
			break
		}
	}
	l.updatableApps = apps
	return
}

// updateJobList clean invalid cached Job status
// The list is the newest JobList.
func (l *Lastore) updateJobList(list []dbus.ObjectPath) {
	var invalids []dbus.ObjectPath
	for jobPath := range l.JobStatus {
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
		delete(l.JobStatus, jobPath)
	}
	log.Infof("UpdateJobList: %v - %v\n", list, invalids)
}

// updateJob update job status
func (l *Lastore) updateJob(path dbus.ObjectPath, status system.Status) {
	job, _ := lastore.NewJob("com.deepin.lastore", path)
	defer lastore.DestroyJob(job)
	t := job.Type.Get()
	if strings.Contains(string(path), "install") && t == system.DownloadJobType {
		return
	}

	if l.JobStatus[path] == status {
		return
	}
	l.JobStatus[path] = status

	l.notifyJob(path, status)
}

func (l *Lastore) updateSystemOnChaning(onChanging bool) {
	l.SystemOnChanging = onChanging
	log.Infof("SystemOnChaning to %v\n", onChanging)
	if onChanging && l.inhibitFd == -1 {
		fd, err := Inhibitor("shutdown", gettext.Tr("Deepin Store"),
			gettext.Tr("System is updating, please shut down or reboot later."))
		log.Infof("Prevent shutdown...: fd:%v\n", fd)
		if err != nil {
			log.Infof("Prevent shutdown failed: fd:%v, err:%v\n", fd, err)
			return
		}
		l.inhibitFd = fd
	}

	if !onChanging && l.inhibitFd != -1 {
		err := syscall.Close(int(l.inhibitFd))
		log.Infof("Enable shutdown...")
		if err != nil {
			log.Infof("Enable shutdown...: fd:%d, err:%s\n", l.inhibitFd, err)
		}
		l.inhibitFd = -1
	}
}

func (l *Lastore) offline() {
	log.Info("Lastore.Daemon Offline\n")
	l.OnLine = false
	l.JobStatus = make(map[dbus.ObjectPath]system.Status)
	l.updateSystemOnChaning(false)
}

func (l *Lastore) online() {
	log.Info("Lastore.Daemon Online\n")
	l.OnLine = true
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

func (l *Lastore) notifyJob(path dbus.ObjectPath, status system.Status) {
	job, err := lastore.NewJob("com.deepin.lastore", path)
	if err != nil {
		return
	}
	defer lastore.DestroyJob(job)
	pkgName := PackageName(job.PackageId.Get(), l.Lang)

	switch guestJobTypeFromPath(path) {
	case system.DownloadJobType:
		switch status {
		case system.FailedStatus:
			NotifyFailedDownload(pkgName, l.createJobFailedActions(job.Id.Get()))
		case system.SucceedStatus:
		}

	case system.InstallJobType:
		switch status {
		case system.FailedStatus:
			NotifyInstall(pkgName, false, l.createJobFailedActions(job.Id.Get()))
		case system.SucceedStatus:
			NotifyInstall(pkgName, true, nil)
		}
	case system.RemoveJobType:
		switch status {
		case system.FailedStatus:
			NotifyRemove(pkgName, false, l.createJobFailedActions(job.Id.Get()))
		case system.SucceedStatus:
			NotifyRemove(pkgName, true, nil)
		}
	case system.DistUpgradeJobType:
		switch status {
		case system.FailedStatus:
			NotifyUpgrade(false, l.createJobFailedActions(job.Id.Get()))
		case system.SucceedStatus:
			NotifyUpgrade(true, l.createUpgradeActions())
		}
	default:
		return
	}
}

// guestJobTypeFromPath guest the JobType from object path
// We can't get the JobType when the DBusObject destroyed.
func guestJobTypeFromPath(path dbus.ObjectPath) string {
	if strings.Contains(string(path), system.InstallJobType) {
		return system.InstallJobType
	} else if strings.Contains(string(path), system.DownloadJobType) {
		return system.DownloadJobType
	} else if strings.Contains(string(path), system.RemoveJobType) {
		return system.RemoveJobType
	} else if strings.Contains(string(path), system.DistUpgradeJobType) {
		return system.DistUpgradeJobType
	}
	return ""
}
