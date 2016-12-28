/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	log "github.com/cihub/seelog"
	"internal/system"
	"os/exec"
	"pkg.deepin.io/lib/dbus"
	"time"
)

var nilObjPath = dbus.ObjectPath("/")

func (l *Lastore) findPrepareDistUpgradeJob() dbus.ObjectPath {
	for p, job := range l.jobStatus {
		if job.Status != system.EndStatus &&
			job.Type == system.PrepareDistUpgradeJobType {
			return p
		}
	}
	return nilObjPath
}

func (l *Lastore) checkPrepareDistUpgradeJob(packages []string) (bool, dbus.ObjectPath, error) {
	job := l.findPrepareDistUpgradeJob()
	if job != nilObjPath {
		// in progress
		return false, job, nil
	}

	size, err := l.core.PackagesDownloadSize(packages)
	log.Debugf("packages %v download size: %v, err: %v", packages, size, err)
	if err != nil {
		return false, nilObjPath, err
	}
	if size == 0 {
		return true, nilObjPath, nil
	}
	return false, nilObjPath, nil
}

func LaunchOfflineUpgrader() {
	log.Debug("Launch offline upgrader")
	go exec.Command("/usr/lib/deepin-daemon/dde-offline-upgrader").Run()
}

func (l *Lastore) handleUpdatablePackagesChanged(packages []string, apps []string) {
	libChanged := len(packages) != len(apps)
	complete, job, err := l.checkPrepareDistUpgradeJob(packages)
	log.Debugf("CheckDownloadUpgradablePackagesJob complete: %v job %q err %v", complete, job, err)
	if err != nil {
		log.Warn(err)
		return
	}
	if !complete && job == nilObjPath {
		if l.updater.AutoDownloadUpdates.Get() {
			// updatable packages are downloaded automatically,
			// so there is no need to notify the user.
			return
		}
		l.notifyDownloadUpgradablePackages(len(apps), libChanged)
		return
	}
	if complete && len(packages) > 0 {
		LaunchOfflineUpgrader()
	}
}

func (l *Lastore) CheckPrepareDistUpgradeJob() (bool, dbus.ObjectPath, error) {
	return l.checkPrepareDistUpgradeJob(l.updatablePackages)
}

func (l *Lastore) LaterUpgrade(nsecs uint32) {
	if l.laterUpgradeTimer != nil {
		l.laterUpgradeTimer.Stop()
	}

	l.laterUpgradeTimer = time.AfterFunc(time.Duration(nsecs)*time.Second, func() {
		complete, job, err := l.CheckPrepareDistUpgradeJob()
		log.Debugf("CheckDownloadUpgradablePackagesJob complete: %v job %q err %v", complete, job, err)
		if err != nil {
			log.Warn(err)
			return
		}
		if complete && len(l.updatablePackages) > 0 {
			LaunchOfflineUpgrader()
		}
	})
}
