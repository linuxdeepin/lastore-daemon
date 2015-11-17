package main

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"net/http"
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

	MirrorSource   string
	OfficialSource string
	mirrorSources  map[string]MirrorSource

	upgradableInfos []system.UpgradeInfo

	b system.System

	UpdatableApps     []string
	UpdatablePackages []string
}

type MirrorSource struct {
	Id   string
	Name string
	Url  string

	location    string
	localeNames map[string]string
}

func NewUpdater(b system.System) *Updater {
	// TODO: Reload the cache
	ms := LoadMirrorSources("http://api.lastore.deepin.org")
	u := Updater{
		OfficialSource: "http://packages.linuxdeepin.com",
		mirrorSources:  make(map[string]MirrorSource),
		b:              b,
	}
	for _, item := range ms {
		u.mirrorSources[item.Id] = item
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
	//TODO: sync the value
	if _, ok := u.mirrorSources[id]; !ok {
		return fmt.Errorf("Can't find the mirror source %q", id)
	}
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

// ListMirrors 返回当前支持的镜像源列表．顺序按优先级降序排
// 其中Name会根据传递进来的lang进行本地化
func (u Updater) ListMirrorSources(lang string) []MirrorSource {
	// TODO: sort it
	var r []MirrorSource
	for _, ms := range u.mirrorSources {
		localeMS := MirrorSource{
			Id:   ms.Id,
			Url:  ms.Url,
			Name: ms.Name,
		}
		if v, ok := ms.localeNames[lang]; ok {
			localeMS.Name = v
		}

		r = append(r, localeMS)
	}
	return r
}

// LoadMirrorSources return supported MirrorSource from remote server
func LoadMirrorSources(server string) []MirrorSource {
	rep, err := http.Get(server + "/mirrors")
	if err != nil {
		log.Warnf("LoadMirrorSources:%v", err)
		return nil
	}
	defer rep.Body.Close()

	d := json.NewDecoder(rep.Body)
	var v struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Data          []struct {
			Id       string                       `json:"id"`
			Name     string                       `json:"name"`
			Url      string                       `json:"url"`
			Location string                       `json:"location"`
			Locale   map[string]map[string]string `json:"locale"`
		} `json:"data"`
	}
	err = d.Decode(&v)
	if err != nil {
		log.Warnf("LoadMirrorSources: %v", err)
		return nil
	}

	if v.StatusCode != 0 {
		log.Warnf("LoadMirrorSources: featch(%q) error: %q",
			server+"/mirrors", v.StatusMessage)
		return nil
	}

	var r []MirrorSource
	for _, raw := range v.Data {
		s := MirrorSource{
			Id:          raw.Id,
			Name:        raw.Name,
			Url:         raw.Url,
			location:    raw.Location,
			localeNames: make(map[string]string),
		}
		for k, v := range raw.Locale {
			s.localeNames[k] = v["name"]
		}
		r = append(r, s)
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
