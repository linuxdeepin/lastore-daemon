// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

type ApplicationInfo struct {
	Id         string            `json:"id"`
	Category   string            `json:"category"`
	Icon       string            `json:"icon"`
	Name       string            `json:"name"`
	LocaleName map[string]string `json:"locale_name"`
}

func (u *Updater) updateUpdatableApps() {
	var apps []string
	appInfos := applicationInfos()
	u.PropsMu.RLock()
	for _, id := range u.UpdatablePackages {
		if _, ok := appInfos[id]; ok {
			apps = append(apps, id)
		}
	}
	u.PropsMu.RUnlock()
	u.setUpdatableApps(apps)
}

func applicationInfos() map[string]ApplicationInfo {
	r := make(map[string]ApplicationInfo)
	_ = system.DecodeJson(path.Join(system.VarLibDir, "applications.json"), &r)
	return r
}

func packageIconInfos() map[string]string {
	r := make(map[string]string)
	_ = system.DecodeJson(path.Join(system.VarLibDir, "package_icon.json"), &r)
	return r
}
