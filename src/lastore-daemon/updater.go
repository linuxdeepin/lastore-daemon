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
	"internal/system"
	"io/ioutil"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/strv"
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
}

func NewUpdater(service *dbusutil.Service, m *Manager, config *Config) *Updater {
	u := &Updater{
		manager:             m,
		service:             service,
		config:              config,
		AutoCheckUpdates:    config.AutoCheckUpdates,
		AutoDownloadUpdates: config.AutoDownloadUpdates,
		MirrorSource:        config.MirrorSource,
		UpdateNotify:        config.UpdateNotify,
	}
	u.ClassifiedUpdatablePackages = make(map[string][]string)
	go u.waitOnlineCheck()
	go u.loopCheck()
	return u
}

func (u *Updater) waitOnlineCheck() {
	err := exec.Command("nm-online", "-t", "3600").Run()
	if err == nil {
		if u.AutoCheckUpdates {
			_, err = u.manager.updateSource(u.UpdateNotify)
			if err != nil {
				logger.Warning(err)
			}
		}
		if !u.config.DisableUpdateMetadata {
			startUpdateMetadataInfoService()
		}
	}
}

func (u *Updater) loopCheck() {

	calcDelay := func() time.Duration {
		elapsed := time.Since(u.config.LastCheckTime)
		remained := u.config.CheckInterval - elapsed
		if remained < 0 {
			return 0
		}
		return remained
	}

	for {
		// ensure delay at least have 10 seconds
		delay := calcDelay() + time.Second*10

		logger.Infof("Next updater check will trigger at %v", time.Now().Add(delay))
		time.Sleep(delay)

		if u.AutoCheckUpdates {
			_, err := u.manager.updateSource(u.UpdateNotify)
			if err != nil {
				logger.Warning(err)
			}
		}

		if !u.config.DisableUpdateMetadata {
			startUpdateMetadataInfoService()
		}

		_ = u.config.UpdateLastCheckTime()
	}
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
		for _, info := range infos.UpgradeInfo {
			appsMap[info.Package] = struct{}{}
		}
	}
	for name := range appsMap {
		apps = append(apps, name)
	}
	return apps
}

func (u *Updater) GetCheckIntervalAndTime() (interval float64, checkTime string, busErr *dbus.Error) {
	interval = u.config.CheckInterval.Hours()
	checkTime = u.config.LastCheckTime.String()
	return
}

func (u *Updater) SetUpdateNotify(enable bool) *dbus.Error {
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
	err := u.restoreSystemSource()
	if err != nil {
		logger.Warning("failed to restore system source:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (u *Updater) setClassifiedUpdatablePackages(infosMap system.SourceUpgradeInfoMap) {
	changed := false
	updatablePackages := make(map[string][]string)

	u.PropsMu.RLock()
	classifiedUpdatablePackages := u.ClassifiedUpdatablePackages
	u.PropsMu.RUnlock()

	for updateType, infos := range infosMap {
		var packages []string
		for _, info := range infos.UpgradeInfo {
			packages = append(packages, info.Package)
		}
		updatablePackages[updateType] = packages
		if !changed {
			newData := strv.Strv(packages)
			oldData := strv.Strv(classifiedUpdatablePackages[updateType])
			changed = !newData.Equal(oldData)
		}
	}
	if changed {
		u.PropsMu.Lock()
		defer u.PropsMu.Unlock()
		u.setPropClassifiedUpdatablePackages(updatablePackages)
	}
}
