// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"sync"
	"time"

	"internal/system"

	"github.com/godbus/dbus"
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
type idleDownloadConfig struct {
	IdleDownloadEnabled bool
	BeginTime           string
	EndTime             string
}

type downloadSpeedLimitConfig struct {
	DownloadSpeedLimitEnabled bool
	LimitSpeed                string
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

	IdleDownloadConfig          string
	idleDownloadConfigObj       idleDownloadConfig
	DownloadSpeedLimitConfig    string
	downloadSpeedLimitConfigObj downloadSpeedLimitConfig

	setDownloadSpeedLimitTimer *time.Timer
	setIdleDownloadConfigTimer *time.Timer
}

func NewUpdater(service *dbusutil.Service, m *Manager, config *Config) *Updater {
	u := &Updater{
		manager:                  m,
		service:                  service,
		config:                   config,
		AutoCheckUpdates:         config.AutoCheckUpdates,
		AutoDownloadUpdates:      config.AutoDownloadUpdates,
		MirrorSource:             config.MirrorSource,
		UpdateNotify:             config.UpdateNotify,
		AutoInstallUpdates:       config.AutoInstallUpdates,
		AutoInstallUpdateType:    config.AutoInstallUpdateType,
		IdleDownloadConfig:       config.idleDownloadConfig,
		DownloadSpeedLimitConfig: config.downloadSpeedLimitConfig,
	}
	u.ClassifiedUpdatablePackages = make(map[string][]string)
	err := json.Unmarshal([]byte(u.IdleDownloadConfig), &u.idleDownloadConfigObj)
	if err != nil {
		logger.Warning(err)
	}
	err = json.Unmarshal([]byte(u.DownloadSpeedLimitConfig), &u.downloadSpeedLimitConfigObj)
	if err != nil {
		logger.Warning(err)
	}
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
