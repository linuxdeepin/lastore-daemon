package main

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"net/http"
	"pkg.deepin.io/lib/dbus"
)

var ServerAPI = "http://api.lastore.deepin.org"

var DefaultMirror = MirrorSource{
	Id:   "default",
	Url:  "http://cdn.packages.linuxdeepin.com/packages-debian",
	Name: "default",
}

type MirrorSource struct {
	Id   string
	Name string
	Url  string

	location    string
	localeNames map[string]string
}

// 设置用于下载软件的镜像源
func (u *Updater) SetMirrorSource(id string) error {
	//TODO: sync the value
	index := indexMirrors(id, u.mirrorSources)
	if index == -1 {
		return fmt.Errorf("Can't find the mirror source %q", id)
	}

	mirror := u.mirrorSources[index]
	u.MirrorSource = mirror.Id

	dbus.NotifyChange(u, "MirrorSource")
	return nil
}

// ListMirrors 返回当前支持的镜像源列表．顺序按优先级降序排(服务器端排序)
// 其中Name会根据传递进来的lang进行本地化
func (u *Updater) ListMirrorSources(lang string) (mirrors []MirrorSource, err error) {
	var r []MirrorSource
	if u.mirrorSources == nil {
		err := u.updateMirrorSources()
		if err != nil {
			return nil, log.Errorf("UpdateMirrorSources: %v", err)
		}
	}
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
	return r, nil
}

func (u *Updater) updateMirrorSources() error {
	list, err := LoadMirrorSources(ServerAPI)
	if err != nil {
		return err
	}

	if indexMirrors(DefaultMirror.Id, list) == -1 {
		list = append([]MirrorSource{DefaultMirror}, list...)
	}

	u.mirrorSources = list
	return nil
}

func indexMirrors(id string, mirrors []MirrorSource) int {
	for i, mirror := range mirrors {
		if mirror.Id == id {
			return i
		}
	}
	return -1
}

// LoadMirrorSources return supported MirrorSource from remote server
func LoadMirrorSources(server string) ([]MirrorSource, error) {
	rep, err := http.Get(server + "/mirrors")
	if err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	d := json.NewDecoder(rep.Body)
	var v struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Data          []struct {
			Id       string                       `json:"id"`
			Weight   int                          `json:"weight"`
			Name     string                       `json:"name"`
			Url      string                       `json:"url"`
			Location string                       `json:"location"`
			Locale   map[string]map[string]string `json:"locale"`
		} `json:"data"`
	}
	err = d.Decode(&v)
	if err != nil {
		fmt.Println("XXX:", err)
		return nil, err
	}

	if v.StatusCode != 0 {
		return nil, fmt.Errorf("LoadMirrorSources: featch(%q) error: %q",
			server+"/mirrors", v.StatusMessage)
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
	return r, nil
}
