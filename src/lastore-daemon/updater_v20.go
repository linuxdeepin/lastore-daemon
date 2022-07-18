package main

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusServiceNameV20 = "com.deepin.lastore"
	dbusPathV20 = "/com/deepin/lastore"
	dbusInterfaceV20   = "com.deepin.lastore.Updater"
)

type UpdaterV20 struct {
	service             *dbusutil.Service
	updater 			*Updater

	UpdatableApps []string
	UpdatablePackages []string
}

func NewUpdaterV20(service *dbusutil.Service, updater *Updater) *UpdaterV20 {
	updaterV20 := &UpdaterV20{
		service:service,
		updater:updater,
	}

	return updaterV20
}

func (updater *UpdaterV20) GetInterfaceName() string {
	return dbusInterfaceV20
}

func (updater *UpdaterV20) syncUpdatableApps(apps []string) {
	updater.UpdatableApps = apps
	updater.emitPropChangedUpdatableApps(updater.UpdatableApps)
}

func (updater *UpdaterV20) emitPropChangedUpdatableApps(value []string) error {
	return updater.service.EmitPropertyChanged(updater, "UpdatableApps", value)
}

func (updater *UpdaterV20) syncUpdatablePackages(apps []string) {
	updater.UpdatablePackages = apps
	updater.emitPropChangedUpdatablePackages(updater.UpdatablePackages)
}

func (updater *UpdaterV20) emitPropChangedUpdatablePackages(value []string) error {
	return updater.service.EmitPropertyChanged(updater, "UpdatablePackages", value)
}