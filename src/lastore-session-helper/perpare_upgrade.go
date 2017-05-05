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
	"pkg.deepin.io/lib/dbus"
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

func (l *Lastore) CheckPrepareDistUpgradeJob() (complete bool, objpath dbus.ObjectPath, err error) {
	job := l.findPrepareDistUpgradeJob()
	if job != nilObjPath {
		// in progress
		return false, job, nil
	}

	packages := l.updatablePackages
	if len(packages) == 0 {
		return true, nilObjPath, nil
	}

	size, err := l.core.PackagesDownloadSize(packages)
	log.Debugf("packages %v download size: %v, err: %v", packages, size, err)
	if err != nil {
		return false, nilObjPath, err
	}
	return size == 0, nilObjPath, nil
}
