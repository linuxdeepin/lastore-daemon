package main

import (
	"encoding/json"
	"fmt"
	"internal/system"
	"os"
	"path"
	"pkg.deepin.io/lib/dbus"
)

var VarLibDir = "/var/lib/lastore"

type Updater struct {
	AutoCheckUpdates bool

	MirrorSource  string
	mirrorSources []MirrorSource

	upgradableInfos []system.UpgradeInfo

	b system.System

	//测试使用
	UpdatableApps1     []string
	UpdatablePackages1 []string
}

func NewUpdater(b system.System) *Updater {
	// TODO: Reload the cache
	u := &Updater{
		b: b,
	}

	go func() {
		u.updateMirrorSources()
		u.UpdatablePackages1 = UpdatableNames(u.b.UpgradeInfo())
	}()

	u.UpdatableApps1 = []string{"abiword", "anjuta", "deepin-movie", "d-feet"}

	return u
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

func DecodeJson(fpath string, data interface{}) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	return d.Decode(data)
}
func EncodeJson(fpath string, data interface{}) error {
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewEncoder(f)
	return d.Encode(data)
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

func GetUpdatablePackageInfo(fpath string) (map[string]system.UpgradeInfo, error) {
	d := make(map[string]system.UpgradeInfo)
	err := DecodeJson(fpath, &d)
	return d, err
}

func GetAppInfos(fpath string) (map[string]AppInfo, error) {
	d := make(map[string]AppInfo)
	err := DecodeJson(fpath, &d)
	return d, err
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
	appInfos, err := GetAppInfos(path.Join(VarLibDir, "application_infos.json"))
	fmt.Println("XX", appInfos)
	if err != nil {
		return nil, err
	}
	updateInfos, err := GetUpdatablePackageInfo(path.Join(VarLibDir, "update_infos.json"))
	if err != nil {
		return nil, err
	}

	iconInfos := GetPackageIconInfo()

	var r []ApplicationUpdateInfo
	for pkgId, uInfo := range updateInfos {
		appInfo, ok := appInfos[pkgId]
		if !ok {
			continue
		}
		info := ApplicationUpdateInfo{
			Id:             pkgId,
			Name:           appInfo.LocaleNames[lang],
			Icon:           iconInfos[pkgId],
			CurrentVersion: uInfo.CurrentVersion,
			LastVersion:    uInfo.LastVersion,
		}
		if info.Name == "" {
			info.Name = appInfo.Name
		}
		if info.Icon == "" {
			info.Icon = pkgId
		}
		r = append(r, info)
	}
	return r, nil
}

func (u *Updater) SetAutoCheckUpdates(enable bool) error {
	//TODO: sync the value
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
