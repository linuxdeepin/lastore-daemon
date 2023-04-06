// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"os"
	"path"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"

	"github.com/godbus/dbus/v5"
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

func (u *Updater) ApplicationUpdateInfos(lang string) (updateInfos []ApplicationUpdateInfo, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	iInfos := packageIconInfos()
	aInfos := applicationInfos()
	var uInfosMap system.SourceUpgradeInfoMap
	var err error
	repeatCount := 0
	for {
		if repeatCount > 5 {
			break
		}
		uInfosMap, err = u.manager.SystemUpgradeInfo()
		if os.IsNotExist(err) {
			time.Sleep(1 * time.Second)
			repeatCount++
		} else if err != nil {
			updateInfoErr, ok := err.(*system.UpdateInfoError)
			if ok {
				return nil, dbusutil.MakeErrorJSON(u, "UpdateInfoError", updateInfoErr)
			}
			return nil, dbusutil.ToError(err)
		} else {
			break
		}
	}

	for _, uInfos := range uInfosMap {
		for _, uInfo := range uInfos {
			id := uInfo.Package

			aInfo, ok := aInfos[id]
			if !ok {
				continue
			}

			info := ApplicationUpdateInfo{
				Id:             id,
				Name:           aInfo.LocaleName[lang],
				Icon:           iInfos[id],
				CurrentVersion: uInfo.CurrentVersion,
				LastVersion:    uInfo.LastVersion,
			}
			if info.Name == "" {
				info.Name = id
			}
			if info.Icon == "" {
				info.Icon = id
			}
			updateInfos = append(updateInfos, info)
		}
	}
	logger.Info("ApplicationUpdateInfos: ", updateInfos)
	return updateInfos, nil
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
