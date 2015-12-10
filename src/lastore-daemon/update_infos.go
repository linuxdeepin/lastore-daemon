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

func (u *Updater) loadUpdateInfos() {
	u.setPropUpdatablePackages(UpdatableNames(u.b.UpgradeInfo()))

	var apps []string
	appInfos := applicationInfos()
	for _, id := range u.UpdatablePackages {
		if _, ok := appInfos[id]; ok {
			apps = append(apps, id)
		}
	}
	u.setPropUpdatableApps(apps)
}

func (u *Updater) ApplicationUpdateInfos(lang string) []ApplicationUpdateInfo {
	if len(u.UpdatableApps) == 0 {
		return nil
	}

	iInfos := packageIconInfos()
	aInfos := applicationInfos()
	uInfos := u.b.UpgradeInfo()

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
	return r
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
