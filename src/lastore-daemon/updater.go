package main

import (
	log "github.com/cihub/seelog"
	"internal/system"
	"path"
	"pkg.deepin.io/lib/dbus"
)

var VarLibDir = "/var/lib/lastore"

type Updater struct {
	AutoCheckUpdates bool

	MirrorSource  string
	mirrorSources []MirrorSource

	b system.System

	UpdatablePackages     []string
	updatablePackageInfos []system.UpgradeInfo

	UpdatableApps     []string
	updatableAppInfos []AppInfo
}

func NewUpdater(b system.System) *Updater {
	// TODO: Reload the cache
	u := &Updater{
		b:            b,
		MirrorSource: DefaultMirror.Id,
	}

	go u._ReloadInfo()

	return u
}

func (u *Updater) _ReloadInfo() error {
	u.updateMirrorSources()

	u.setUpdatablePackages(UpdatableNames(u.b.UpgradeInfo()))

	u.updateUpdatableApps()

	return nil
}

type AppInfo struct {
	Id          string
	Category    string
	Icon        string
	Name        string
	LocaleNames map[string]string
}

type ApplicationUpdateInfo struct {
	Id             string
	Name           string
	Icon           string
	CurrentVersion string
	LastVersion    string
}

func GetPackageIconInfo() map[string]string {
	desktops := make(map[string]string)
	icons := make(map[string]string)
	DecodeJson(path.Join(VarLibDir, "desktop_package.json"), desktops)
	DecodeJson(path.Join(VarLibDir, "desktop_icon.json"), icons)

	r := make(map[string]string)
	for desktop, pkg := range desktops {
		icon, ok := icons[desktop]
		if !ok {
			continue
		}
		r[pkg] = icon
	}
	return r
}

func (u *Updater) updateUpdatableApps() {
	apps := GetAppInfos(VarLibDir)

	var r []AppInfo
	for _, pkg := range u.UpdatablePackages {
		info, ok := apps[pkg]
		if ok {
			r = append(r, info)
		}
	}
	u.setUpdatableApps(r)
}

func GetUpdatablePackageInfo(fpath string) (map[string]system.UpgradeInfo, error) {
	d := make(map[string]system.UpgradeInfo)
	err := DecodeJson(fpath, &d)
	return d, err
}

func GetAppInfos(baseDir string) map[string]AppInfo {
	fpath := path.Join(VarLibDir, "application_infos.json")
	d := make(map[string]AppInfo)
	err := DecodeJson(fpath, &d)
	if err != nil {
		log.Warnf("GetAppInfos:%v\n", err)
	}
	return d
}

func (u *Updater) updateUpdateInfo() error {
	infos := u.b.UpgradeInfo()
	r := make(map[string]system.UpgradeInfo)
	for _, info := range infos {
		r[info.Package] = info
	}
	return EncodeJson(path.Join(VarLibDir, "update_infos.json"), r)
}

func (u *Updater) ApplicationUpdateInfos1(lang string) ([]ApplicationUpdateInfo, error) {
	if u.UpdatableApps == nil {
		u.updateUpdatableApps()
	}

	iconInfos := GetPackageIconInfo()

	var r []ApplicationUpdateInfo
	for _, appInfo := range u.updatableAppInfos {
		var uInfo system.UpgradeInfo
		for _, pkgInfo := range u.updatablePackageInfos {
			if pkgInfo.Package == appInfo.Id {
				uInfo = pkgInfo
				break
			}
		}
		if uInfo.Package == "" {
			log.Warnf("Invalid UpdateInfo for %v  (%v)\n", appInfo, u.updatablePackageInfos)
		}

		updateInfo := ApplicationUpdateInfo{
			Id:             appInfo.Id,
			Name:           appInfo.LocaleNames[lang],
			Icon:           iconInfos[appInfo.Id],
			CurrentVersion: uInfo.CurrentVersion,
			LastVersion:    uInfo.LastVersion,
		}
		if updateInfo.Name == "" {
			updateInfo.Name = appInfo.Id
		}
		if updateInfo.Icon == "" {
			updateInfo.Icon = appInfo.Id
		}
		r = append(r, updateInfo)
	}
	return r, nil
}

func (u *Updater) SetAutoCheckUpdates(enable bool) error {
	if u.AutoCheckUpdates != enable {
		u.AutoCheckUpdates = enable
		dbus.NotifyChange(u, "AutoCheckUpdates")
	}
	return nil
}

func UpdatableNames(infos []system.UpgradeInfo) []string {
	var apps []string
	for _, info := range infos {
		apps = append(apps, info.Package)
	}
	return apps
}
