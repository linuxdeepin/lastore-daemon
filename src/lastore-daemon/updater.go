package main

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"net/http"
	"pkg.deepin.io/lib/dbus"
)

type Updater struct {
	AutoCheckUpdates bool

	MirrorSource   string
	OfficialSource string
	mirrorSources  map[string]MirrorSource
}

type MirrorSource struct {
	Id   string
	Name string
	Url  string

	location    string
	localeNames map[string]string
}

func NewUpdater() *Updater {
	// TODO: Reload the cache
	ms := LoadMirrorSources("http://api.lastore.deepin.org")
	u := Updater{
		OfficialSource: "http://packages.linuxdeepin.com",
		mirrorSources:  make(map[string]MirrorSource),
	}
	for _, item := range ms {
		u.mirrorSources[item.Id] = item
	}
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

func (u Updater) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "org.deepin.lastore",
		ObjectPath: "/org/deepin/lastore",
		Interface:  "org.deepin.lastore.Updater",
	}
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
