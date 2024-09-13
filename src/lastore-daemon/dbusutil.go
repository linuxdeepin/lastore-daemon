// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

// Code generated by "dbusutil-gen -type Updater,Job,Manager -output dbusutil.go -import internal/system,github.com/godbus/dbus/v5 updater.go job.go manager.go"; DO NOT EDIT.

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/godbus/dbus/v5"
)

func (v *Updater) setPropAutoCheckUpdates(value bool) (changed bool) {
	if v.AutoCheckUpdates != value {
		v.AutoCheckUpdates = value
		v.emitPropChangedAutoCheckUpdates(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedAutoCheckUpdates(value bool) error {
	return v.service.EmitPropertyChanged(v, "AutoCheckUpdates", value)
}

func (v *Updater) setPropAutoDownloadUpdates(value bool) (changed bool) {
	if v.AutoDownloadUpdates != value {
		v.AutoDownloadUpdates = value
		v.emitPropChangedAutoDownloadUpdates(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedAutoDownloadUpdates(value bool) error {
	return v.service.EmitPropertyChanged(v, "AutoDownloadUpdates", value)
}

func (v *Updater) setPropUpdateNotify(value bool) (changed bool) {
	if v.UpdateNotify != value {
		v.UpdateNotify = value
		v.emitPropChangedUpdateNotify(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedUpdateNotify(value bool) error {
	return v.service.EmitPropertyChanged(v, "UpdateNotify", value)
}

func (v *Updater) setPropMirrorSource(value string) (changed bool) {
	if v.MirrorSource != value {
		v.MirrorSource = value
		v.emitPropChangedMirrorSource(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedMirrorSource(value string) error {
	return v.service.EmitPropertyChanged(v, "MirrorSource", value)
}

func (v *Updater) setPropUpdatableApps(value []string) {
	v.UpdatableApps = value
	v.emitPropChangedUpdatableApps(value)
}

func (v *Updater) emitPropChangedUpdatableApps(value []string) error {
	return v.service.EmitPropertyChanged(v, "UpdatableApps", value)
}

func (v *Updater) setPropUpdatablePackages(value []string) {
	v.UpdatablePackages = value
	v.emitPropChangedUpdatablePackages(value)
}

func (v *Updater) emitPropChangedUpdatablePackages(value []string) error {
	return v.service.EmitPropertyChanged(v, "UpdatablePackages", value)
}

func (v *Updater) setPropClassifiedUpdatablePackages(value map[string][]string) {
	v.ClassifiedUpdatablePackages = value
	v.emitPropChangedClassifiedUpdatablePackages(value)
}

func (v *Updater) emitPropChangedClassifiedUpdatablePackages(value map[string][]string) error {
	return v.service.EmitPropertyChanged(v, "ClassifiedUpdatablePackages", value)
}

func (v *Updater) setPropAutoInstallUpdates(value bool) (changed bool) {
	if v.AutoInstallUpdates != value {
		v.AutoInstallUpdates = value
		v.emitPropChangedAutoInstallUpdates(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedAutoInstallUpdates(value bool) error {
	return v.service.EmitPropertyChanged(v, "AutoInstallUpdates", value)
}

func (v *Updater) setPropAutoInstallUpdateType(value system.UpdateType) (changed bool) {
	if v.AutoInstallUpdateType != value {
		v.AutoInstallUpdateType = value
		v.emitPropChangedAutoInstallUpdateType(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedAutoInstallUpdateType(value system.UpdateType) error {
	return v.service.EmitPropertyChanged(v, "AutoInstallUpdateType", value)
}

func (v *Updater) setPropIdleDownloadConfig(value string) (changed bool) {
	if v.IdleDownloadConfig != value {
		v.IdleDownloadConfig = value
		v.emitPropChangedIdleDownloadConfig(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedIdleDownloadConfig(value string) error {
	return v.service.EmitPropertyChanged(v, "IdleDownloadConfig", value)
}

func (v *Updater) setPropDownloadSpeedLimitConfig(value string) (changed bool) {
	if v.DownloadSpeedLimitConfig != value {
		v.DownloadSpeedLimitConfig = value
		v.emitPropChangedDownloadSpeedLimitConfig(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedDownloadSpeedLimitConfig(value string) error {
	return v.service.EmitPropertyChanged(v, "DownloadSpeedLimitConfig", value)
}

func (v *Updater) setPropUpdateTarget(value string) (changed bool) {
	if v.UpdateTarget != value {
		v.UpdateTarget = value
		v.emitPropChangedUpdateTarget(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedUpdateTarget(value string) error {
	return v.service.EmitPropertyChanged(v, "UpdateTarget", value)
}

func (v *Updater) setPropOfflineInfo(value string) (changed bool) {
	if v.OfflineInfo != value {
		v.OfflineInfo = value
		v.emitPropChangedOfflineInfo(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedOfflineInfo(value string) error {
	return v.service.EmitPropertyChanged(v, "OfflineInfo", value)
}

func (v *Updater) setPropP2PUpdateEnable(value bool) (changed bool) {
	if v.P2PUpdateEnable != value {
		v.P2PUpdateEnable = value
		v.emitPropChangedP2PUpdateEnable(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedP2PUpdateEnable(value bool) error {
	return v.service.EmitPropertyChanged(v, "P2PUpdateEnable", value)
}

func (v *Updater) setPropP2PUpdateSupport(value bool) (changed bool) {
	if v.P2PUpdateSupport != value {
		v.P2PUpdateSupport = value
		v.emitPropChangedP2PUpdateSupport(value)
		return true
	}
	return false
}

func (v *Updater) emitPropChangedP2PUpdateSupport(value bool) error {
	return v.service.EmitPropertyChanged(v, "P2PUpdateSupport", value)
}

func (v *Job) setPropId(value string) (changed bool) {
	if v.Id != value {
		v.Id = value
		v.emitPropChangedId(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedId(value string) error {
	return v.service.EmitPropertyChanged(v, "Id", value)
}

func (v *Job) setPropName(value string) (changed bool) {
	if v.Name != value {
		v.Name = value
		v.emitPropChangedName(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedName(value string) error {
	return v.service.EmitPropertyChanged(v, "Name", value)
}

func (v *Job) setPropPackages(value []string) {
	v.Packages = value
	v.emitPropChangedPackages(value)
}

func (v *Job) emitPropChangedPackages(value []string) error {
	return v.service.EmitPropertyChanged(v, "Packages", value)
}

func (v *Job) setPropCreateTime(value int64) (changed bool) {
	if v.CreateTime != value {
		v.CreateTime = value
		v.emitPropChangedCreateTime(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedCreateTime(value int64) error {
	return v.service.EmitPropertyChanged(v, "CreateTime", value)
}

func (v *Job) setPropDownloadSize(value int64) (changed bool) {
	if v.DownloadSize != value {
		v.DownloadSize = value
		v.emitPropChangedDownloadSize(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedDownloadSize(value int64) error {
	return v.service.EmitPropertyChanged(v, "DownloadSize", value)
}

func (v *Job) setPropType(value string) (changed bool) {
	if v.Type != value {
		v.Type = value
		v.emitPropChangedType(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedType(value string) error {
	return v.service.EmitPropertyChanged(v, "Type", value)
}

func (v *Job) setPropStatus(value system.Status) (changed bool) {
	if v.Status != value {
		v.Status = value
		v.emitPropChangedStatus(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedStatus(value system.Status) error {
	return v.service.EmitPropertyChanged(v, "Status", value)
}

func (v *Job) setPropProgress(value float64) (changed bool) {
	if v.Progress != value {
		v.Progress = value
		v.emitPropChangedProgress(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedProgress(value float64) error {
	return v.service.EmitPropertyChanged(v, "Progress", value)
}

func (v *Job) setPropDescription(value string) (changed bool) {
	if v.Description != value {
		v.Description = value
		v.emitPropChangedDescription(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedDescription(value string) error {
	return v.service.EmitPropertyChanged(v, "Description", value)
}

func (v *Job) setPropSpeed(value int64) (changed bool) {
	if v.Speed != value {
		v.Speed = value
		v.emitPropChangedSpeed(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedSpeed(value int64) error {
	return v.service.EmitPropertyChanged(v, "Speed", value)
}

func (v *Job) setPropCancelable(value bool) (changed bool) {
	if v.Cancelable != value {
		v.Cancelable = value
		v.emitPropChangedCancelable(value)
		return true
	}
	return false
}

func (v *Job) emitPropChangedCancelable(value bool) error {
	return v.service.EmitPropertyChanged(v, "Cancelable", value)
}

func (v *Manager) setPropJobList(value []dbus.ObjectPath) {
	v.JobList = value
	v.emitPropChangedJobList(value)
}

func (v *Manager) emitPropChangedJobList(value []dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "JobList", value)
}

func (v *Manager) setPropUpgradableApps(value []string) {
	v.UpgradableApps = value
	v.emitPropChangedUpgradableApps(value)
}

func (v *Manager) emitPropChangedUpgradableApps(value []string) error {
	return v.service.EmitPropertyChanged(v, "UpgradableApps", value)
}

func (v *Manager) setPropSystemOnChanging(value bool) (changed bool) {
	if v.SystemOnChanging != value {
		v.SystemOnChanging = value
		v.emitPropChangedSystemOnChanging(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedSystemOnChanging(value bool) error {
	return v.service.EmitPropertyChanged(v, "SystemOnChanging", value)
}

func (v *Manager) setPropAutoClean(value bool) (changed bool) {
	if v.AutoClean != value {
		v.AutoClean = value
		v.emitPropChangedAutoClean(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedAutoClean(value bool) error {
	return v.service.EmitPropertyChanged(v, "AutoClean", value)
}

func (v *Manager) setPropUpdateMode(value system.UpdateType) (changed bool) {
	if v.UpdateMode != value {
		v.UpdateMode = value
		v.emitPropChangedUpdateMode(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedUpdateMode(value system.UpdateType) error {
	return v.service.EmitPropertyChanged(v, "UpdateMode", value)
}

func (v *Manager) setPropCheckUpdateMode(value system.UpdateType) (changed bool) {
	if v.CheckUpdateMode != value {
		v.CheckUpdateMode = value
		v.emitPropChangedCheckUpdateMode(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedCheckUpdateMode(value system.UpdateType) error {
	return v.service.EmitPropertyChanged(v, "CheckUpdateMode", value)
}

func (v *Manager) setPropUpdateStatus(value string) (changed bool) {
	if v.UpdateStatus != value {
		v.UpdateStatus = value
		v.emitPropChangedUpdateStatus(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedUpdateStatus(value string) error {
	return v.service.EmitPropertyChanged(v, "UpdateStatus", value)
}

func (v *Manager) setPropHardwareId(value string) (changed bool) {
	if v.HardwareId != value {
		v.HardwareId = value
		v.emitPropChangedHardwareId(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedHardwareId(value string) error {
	return v.service.EmitPropertyChanged(v, "HardwareId", value)
}
