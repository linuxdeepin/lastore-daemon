// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"io/ioutil"
	"os/exec"
	"path"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
)

type ApplicationUpdateInfo struct {
	Id             string
	Name           string
	Icon           string
	CurrentVersion string
	LastVersion    string
}

type Updater struct {
	manager             *Manager
	service             *dbusutil.Service
	PropsMu             sync.RWMutex
	AutoCheckUpdates    bool
	AutoDownloadUpdates bool
	UpdateNotify        bool
	MirrorSource        string

	config *Config
	// dbusutil-gen: equal=nil
	UpdatableApps []string
	// dbusutil-gen: equal=nil
	UpdatablePackages []string
	// dbusutil-gen: equal=nil
	ClassifiedUpdatablePackages map[string][]string

	AutoInstallUpdates    bool              `prop:"access:rw"`
	AutoInstallUpdateType system.UpdateType `prop:"access:rw"`
}

func NewUpdater(service *dbusutil.Service, m *Manager, config *Config) *Updater {
	u := &Updater{
		manager:               m,
		service:               service,
		config:                config,
		AutoCheckUpdates:      config.AutoCheckUpdates,
		AutoDownloadUpdates:   config.AutoDownloadUpdates,
		MirrorSource:          config.MirrorSource,
		UpdateNotify:          config.UpdateNotify,
		AutoInstallUpdates:    config.AutoInstallUpdates,
		AutoInstallUpdateType: config.AutoInstallUpdateType,
	}
	u.ClassifiedUpdatablePackages = make(map[string][]string)
	return u
}

func startUpdateMetadataInfoService() {
	logger.Info("start update metadata info service")
	err := exec.Command("systemctl", "start", "lastore-update-metadata-info.service").Run()
	if err != nil {
		logger.Warningf("AutoCheck Update failed: %v", err)
	}
}

func SetAPTSmartMirror(url string) error {
	return ioutil.WriteFile("/etc/apt/apt.conf.d/99mirrors.conf",
		([]byte)(fmt.Sprintf("Acquire::SmartMirrors::MirrorSource %q;", url)),
		0644) // #nosec G306
}

// 设置用于下载软件的镜像源
func (u *Updater) SetMirrorSource(id string) *dbus.Error {
	u.service.DelayAutoQuit()
	err := u.setMirrorSource(id)
	return dbusutil.ToError(err)
}

func (u *Updater) setMirrorSource(id string) error {
	if id == "" || u.MirrorSource == id {
		return nil
	}

	found := false
	for _, m := range u.listMirrorSources("") {
		if m.Id != id {
			continue
		}
		found = true
		if m.Url == "" {
			return system.NotFoundError("empty url")
		}
		if err := SetAPTSmartMirror(m.Url); err != nil {
			logger.Warningf("SetMirrorSource(%q) failed:%v\n", id, err)
			return err
		}
	}
	if !found {
		return system.NotFoundError("invalid mirror source id")
	}
	err := u.config.SetMirrorSource(id)
	if err != nil {
		return err
	}
	u.MirrorSource = u.config.MirrorSource
	_ = u.emitPropChangedMirrorSource(u.MirrorSource)
	return nil
}

type LocaleMirrorSource struct {
	Id   string
	Url  string
	Name string
}

// ListMirrors 返回当前支持的镜像源列表．顺序按优先级降序排
// 其中Name会根据传递进来的lang进行本地化
func (u *Updater) ListMirrorSources(lang string) (mirrorSources []LocaleMirrorSource, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	return u.listMirrorSources(lang), nil
}

func (u *Updater) listMirrorSources(lang string) []LocaleMirrorSource {
	var raws []system.MirrorSource
	_ = system.DecodeJson(path.Join(system.VarLibDir, "mirrors.json"), &raws)

	makeLocaleMirror := func(lang string, m system.MirrorSource) LocaleMirrorSource {
		ms := LocaleMirrorSource{
			Id:   m.Id,
			Url:  m.Url,
			Name: m.Name,
		}
		if v, ok := m.NameLocale[lang]; ok {
			ms.Name = v
		}
		return ms
	}

	var r []LocaleMirrorSource
	for _, raw := range raws {
		if raw.Weight < 0 {
			continue
		}
		r = append(r, makeLocaleMirror(lang, raw))
	}

	return r
}

func UpdatableNames(infosMap system.SourceUpgradeInfoMap) []string {
	// 去重,防止出现下载量出现偏差（同一包，重复出现在系统仓库和商店仓库）
	var apps []string
	appsMap := make(map[string]struct{})
	for _, infos := range infosMap {
		for _, info := range infos {
			appsMap[info.Package] = struct{}{}
		}
	}
	for name := range appsMap {
		apps = append(apps, name)
	}
	return apps
}

func (u *Updater) GetCheckIntervalAndTime() (interval float64, checkTime string, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	interval = u.config.CheckInterval.Hours()
	checkTime = u.config.LastCheckTime.Format("2006-01-02 15:04:05.999999999 -0700 MST")
	return
}

func (u *Updater) SetUpdateNotify(enable bool) *dbus.Error {
	u.service.DelayAutoQuit()
	if u.UpdateNotify == enable {
		return nil
	}
	err := u.config.SetUpdateNotify(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}
	u.UpdateNotify = enable

	_ = u.emitPropChangedUpdateNotify(enable)

	return nil
}

func (u *Updater) SetAutoCheckUpdates(enable bool) *dbus.Error {
	u.service.DelayAutoQuit()
	if u.AutoCheckUpdates == enable {
		return nil
	}

	// save the config to disk
	err := u.config.SetAutoCheckUpdates(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	u.AutoCheckUpdates = enable
	_ = u.emitPropChangedAutoCheckUpdates(enable)
	return nil
}

func (u *Updater) SetAutoDownloadUpdates(enable bool) *dbus.Error {
	u.service.DelayAutoQuit()
	if u.AutoDownloadUpdates == enable {
		return nil
	}

	// save the config to disk
	err := u.config.SetAutoDownloadUpdates(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	u.AutoDownloadUpdates = enable
	_ = u.emitPropChangedAutoDownloadUpdates(enable)
	return nil
}

const (
	aptSource       = "/etc/apt/sources.list"
	aptSourceOrigin = aptSource + ".origin"
)

func (u *Updater) restoreSystemSource() error {
	// write backup file
	current, err := ioutil.ReadFile(aptSource)
	if err == nil {
		err = ioutil.WriteFile(aptSource+".bak", current, 0644) // #nosec G306
		if err != nil {
			logger.Warning(err)
		}
	} else {
		logger.Warning(err)
	}

	origin, err := ioutil.ReadFile(aptSourceOrigin)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(aptSource, origin, 0644) // #nosec G306
	return err
}

func (u *Updater) RestoreSystemSource() *dbus.Error {
	u.service.DelayAutoQuit()
	err := u.restoreSystemSource()
	if err != nil {
		logger.Warning("failed to restore system source:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (u *Updater) setClassifiedUpdatablePackages(infosMap system.SourceUpgradeInfoMap) {
	changed := false
	newUpdatablePackages := make(map[string][]string)

	u.PropsMu.RLock()
	oldUpdatablePackages := u.ClassifiedUpdatablePackages
	u.PropsMu.RUnlock()

	for updateType, infos := range infosMap {
		var packages []string
		for _, info := range infos {
			packages = append(packages, info.Package)
		}
		newUpdatablePackages[updateType] = packages
	}
	for _, updateType := range system.AllUpdateType() {
		if !changed {
			newData := strv.Strv(newUpdatablePackages[updateType.JobType()])
			oldData := strv.Strv(oldUpdatablePackages[updateType.JobType()])
			changed = !newData.Equal(oldData)
			if changed {
				break
			}
		}
	}
	if changed {
		u.PropsMu.Lock()
		defer u.PropsMu.Unlock()
		u.setPropClassifiedUpdatablePackages(newUpdatablePackages)
	}
}

func (u *Updater) autoInstallUpdatesWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	return dbusutil.ToError(u.config.SetAutoInstallUpdates(pw.Value.(bool)))
}

func (u *Updater) autoInstallUpdatesSuitesWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	return dbusutil.ToError(u.config.SetAutoInstallUpdateType(system.UpdateType(pw.Value.(uint64))))
}
