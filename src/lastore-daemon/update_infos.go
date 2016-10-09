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
	"internal/system"
	"path"
)

type ApplicationInfo struct {
	Id         string            `json:"id"`
	Category   string            `json:"category"`
	Icon       string            `json:"icon"`
	Name       string            `json:"name"`
	LocaleName map[string]string `json:"locale_name"`
}

func (u *Updater) loadUpdateInfos(info []system.UpgradeInfo) {
	u.setPropUpdatablePackages(UpdatableNames(info))

	var apps []string
	appInfos := applicationInfos()
	for _, id := range u.UpdatablePackages {
		if _, ok := appInfos[id]; ok {
			apps = append(apps, id)
		}
	}
	u.setPropUpdatableApps(apps)
}

func (u *Updater) ApplicationUpdateInfos(lang string) ([]ApplicationUpdateInfo, error) {
	if len(u.UpdatableApps) == 0 {
		return nil, nil
	}

	iInfos := packageIconInfos()
	aInfos := applicationInfos()
	uInfos, err := system.SystemUpgradeInfo()
	if err != nil {
		return nil, err
	}

	var r []ApplicationUpdateInfo
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
		r = append(r, info)
	}
	return r, nil
}

func applicationInfos() map[string]ApplicationInfo {
	r := make(map[string]ApplicationInfo)
	system.DecodeJson(path.Join(system.VarLibDir, "applications.json"), &r)
	return r
}

func packageIconInfos() map[string]string {
	r := make(map[string]string)
	system.DecodeJson(path.Join(system.VarLibDir, "package_icon.json"), &r)
	return r
}
