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
	log "github.com/cihub/seelog"
	"internal/system"
	"io/ioutil"
	"os/exec"
	"path"
	"pkg.deepin.io/lib/dbus"
	"time"
)

type ApplicationUpdateInfo struct {
	Id             string
	Name           string
	Icon           string
	CurrentVersion string
	LastVersion    string

	// There  hasn't support
	changeLog string
}

type Updater struct {
	AutoCheckUpdates    bool
	AutoDownloadUpdates bool

	MirrorSource string

	config *Config

	UpdatableApps     []string
	UpdatablePackages []string
}

func NewUpdater(b system.System, config *Config) *Updater {
	u := &Updater{
		config:              config,
		AutoCheckUpdates:    config.AutoCheckUpdates,
		AutoDownloadUpdates: config.AutoDownloadUpdates,
		MirrorSource:        config.MirrorSource,
	}
	go u.loopCheck()
	return u
}

func (u *Updater) loopCheck() {
	doUpdate := func() {
		err := exec.Command("systemctl", "start", "lastore-update-metadata-info.service").Run()
		if err != nil {
			log.Warnf("AutoCheck Update failed: %v", err)
		}
		u.config.UpdateLastCheckTime()
	}

	calcDelay := func() time.Duration {
		elapsed := time.Now().Sub(u.config.LastCheckTime)
		remaind := u.config.CheckInterval - elapsed
		if remaind < 0 {
			return 0
		}
		return remaind
	}

	for {
		// ensure delay at least have 10 seconds
		delay := calcDelay() + time.Second*10

		fmt.Println("HH", time.Now().Add(delay), u.AutoCheckUpdates)
		if u.AutoCheckUpdates {
			log.Warnf("Next Check Updates will trigger at %v", time.Now().Add(delay))
		}
		<-time.After(delay)
		if !u.AutoCheckUpdates {
			continue
		}
		doUpdate()
	}
}

func SetAPTSmartMirror(url string) error {
	return ioutil.WriteFile("/etc/apt/apt.conf.d/99mirrors.conf",
		([]byte)(fmt.Sprintf("Acquire::SmartMirrors::MirrorSource %q;", url)),
		0644)
}

// 设置用于下载软件的镜像源
func (u *Updater) SetMirrorSource(id string) error {
	if u.MirrorSource == id {
		return nil
	}
	for _, m := range u.ListMirrorSources("") {
		if m.Id != id {
			continue
		}

		if m.Url == "" {
			return system.NotFoundError
		}
		if err := SetAPTSmartMirror(m.Url); err != nil {
			log.Warnf("SetMirrorSource(%q) failed:%v\n", id, err)
			return err
		}
	}

	err := u.config.SetMirrorSource(id)
	if err != nil {
		return err
	}
	u.MirrorSource = u.config.MirrorSource
	dbus.NotifyChange(u, "MirrorSource")
	return nil
}

type LocaleMirrorSource struct {
	Id   string
	Url  string
	Name string
}

// ListMirrors 返回当前支持的镜像源列表．顺序按优先级降序排
// 其中Name会根据传递进来的lang进行本地化
func (u Updater) ListMirrorSources(lang string) []LocaleMirrorSource {
	var raws []system.MirrorSource
	system.DecodeJson(path.Join(system.VarLibDir, "mirrors.json"), &raws)

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

func UpdatableNames(infos []system.UpgradeInfo) []string {
	var apps []string
	for _, info := range infos {
		apps = append(apps, info.Package)
	}
	return apps
}

func (u *Updater) SetAutoCheckUpdates(enable bool) error {
	if u.AutoCheckUpdates == enable {
		return nil
	}

	// save the config to disk
	err := u.config.SetAutoCheckUpdates(enable)
	if err != nil {
		return err
	}

	u.AutoCheckUpdates = enable
	dbus.NotifyChange(u, "AutoCheckUpdates")
	return nil
}

func (u *Updater) SetAutoDownloadUpdates(enable bool) error {
	if u.AutoDownloadUpdates == enable {
		return nil
	}

	// save the config to disk
	err := u.config.SetAutoDownloadUpdates(enable)
	if err != nil {
		return err
	}

	u.AutoDownloadUpdates = enable
	dbus.NotifyChange(u, "AutoDownloadUpdates")
	return nil
}
