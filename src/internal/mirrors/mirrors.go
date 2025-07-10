// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package mirrors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
)

func GenerateMirrors(repository string, fpath string) error {
	ms, err := LoadMirrorSources("")
	if err != nil {
		return err
	}
	return utils.WriteData(fpath, ms)
}

func GenerateUnpublishedMirrors(url, fpath string) error {
	ms, err := getUnpublishedMirrorSources(url)
	if err != nil {
		return err
	}
	return utils.WriteData(fpath, ms)
}

type mirror struct {
	Id          string                       `json:"id"`
	Weight      int                          `json:"weight"`
	AdjustDelay int                          `json:"adjustDelay"`
	Name        string                       `json:"name"`
	UrlHttp     string                       `json:"urlHttp"`
	UrlHttps    string                       `json:"urlHttps"`
	UrlFtp      string                       `json:"urlFtp"`
	Country     string                       `json:"country"`
	Locale      map[string]map[string]string `json:"locale"`
}

type unpublishedMirrors struct {
	Error   string  `json:"error"`
	Mirrors mirrors `json:"mirrors"`
}

type mirrors []*mirror

// implement sort.Interface interface

func (v mirrors) Len() int {
	return len(v)
}

func (v mirrors) Less(i, j int) bool {
	return v[i].Weight > v[j].Weight
}

func (v mirrors) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

func getUnpublishedMirrorSources(url string) ([]system.MirrorSource, error) {
	fmt.Println("mirrors api url:", url)

	rep, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	d := json.NewDecoder(rep.Body)
	var v unpublishedMirrors
	err = d.Decode(&v)
	if err != nil {
		return nil, err
	}

	if rep.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("callApiMirrors: fetch %q is not ok, status: %q",
			url, rep.Status)
	}

	mirrorsSources := toMirrorsSourceList(v.Mirrors)
	return mirrorsSources, nil
}

// LoadMirrorSources return supported MirrorSource from remote server
func LoadMirrorSources(url string) ([]system.MirrorSource, error) {
	config := config.NewConfig(path.Join("/var/lib/lastore", "config.json"))
	var mirrorsUrl string
	if url != "" {
		mirrorsUrl = url
	} else {
		// get mirrorsUrl from config file
		data, err := os.ReadFile(filepath.Join(system.VarLibDir, "config.json"))
		if err != nil {
			if os.IsNotExist(err) {
				mirrorsUrl = config.MirrorsUrl
			} else {
				return nil, err
			}
		} else {
			cfg := struct {
				MirrorsUrl string
			}{config.MirrorsUrl}

			err = json.Unmarshal(data, &cfg)
			if err != nil {
				return nil, err
			}
			mirrorsUrl = cfg.MirrorsUrl
		}
	}

	fmt.Println("mirrorsUrl:", mirrorsUrl)

	rep, err := http.Get(mirrorsUrl)
	if err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	d := json.NewDecoder(rep.Body)
	var v mirrors
	err = d.Decode(&v)
	if err != nil {
		return nil, err
	}

	if rep.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LoadMirrorSources: fetch %q is not ok, status: %q",
			mirrorsUrl, rep.Status)
	}

	mirrorsSources := toMirrorsSourceList(v)
	if len(mirrorsSources) == 0 {
		return nil, system.NotFoundError("fetch mirrors")
	}
	return mirrorsSources, nil
}

func toMirrorsSourceList(v mirrors) []system.MirrorSource {
	var result []system.MirrorSource
	sort.Sort(v)
	for _, raw := range v {
		s := system.MirrorSource{
			Id:          raw.Id,
			Name:        raw.Name,
			Weight:      raw.Weight,
			NameLocale:  make(map[string]string),
			Country:     raw.Country,
			AdjustDelay: raw.AdjustDelay,
		}
		for k, v := range raw.Locale {
			s.NameLocale[k] = v["name"]
		}

		if raw.UrlHttps != "" {
			s.Url = "https://" + raw.UrlHttps
		} else if raw.UrlHttp != "" {
			s.Url = "http://" + raw.UrlHttp
		}

		if s.Url != "" {
			result = append(result, s)
		}
	}
	return result
}
