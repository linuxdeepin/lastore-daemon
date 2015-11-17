package main

import (
	log "github.com/cihub/seelog"
	"internal/system"
	"path"
	"pkg.deepin.io/lib/dbus"
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
	AutoCheckUpdates bool

	MirrorSource string

	b system.System

	UpdatableApps     []string
	UpdatablePackages []string
}

func NewUpdater(b system.System) *Updater {
	// TODO: Reload the cache
	u := Updater{
		b: b,
	}

	dm := system.NewDirMonitor(system.VarLibDir)
	dm.Add(func(fpath string, op uint32) {
		u.loadUpdateInfos()
	}, "update_infos.json", "package_icons.json", "applications.json")
	err := dm.Start()
	if err != nil {
		log.Warnf("Can't create inotify on %s: %v\n", system.VarLibDir, err)
	}

	u.loadUpdateInfos()
	return &u
}

// 设置用于下载软件的镜像源
func (u *Updater) SetMirrorSource(id string) error {
	u.MirrorSource = id
	dbus.NotifyChange(u, "MirrorSource")
	return nil
}

func (u *Updater) SetAutoCheckUpdates(enable bool) error {
	//TODO: sync the value
	if u.AutoCheckUpdates != enable {
		u.AutoCheckUpdates = enable
		dbus.NotifyChange(u, "AutoCheckUpdates")
	}
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

	var r []LocaleMirrorSource
	for _, raw := range raws {
		ms := LocaleMirrorSource{
			Id:   raw.Id,
			Url:  raw.Url,
			Name: raw.Name,
		}
		if v, ok := raw.NameLocale[lang]; ok {
			ms.Name = v
		}

		r = append(r, ms)
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
