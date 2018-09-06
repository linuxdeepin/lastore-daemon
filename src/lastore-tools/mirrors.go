/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/json"
	"fmt"
	"internal/system"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

func GenerateMirrors(repository string, fpath string) error {
	ms, err := LoadMirrorSources("")
	if err != nil {
		return err
	}
	return writeData(fpath, ms)
}

func GenerateUnpublishedMirrors(url, fpath string) error {
	ms, err := getUnpublishedMirrorSources(url)
	if err != nil {
		return err
	}
	return writeData(fpath, ms)
}

type mirror struct {
	Id       string                       `json:"id"`
	Weight   int                          `json:"weight"`
	Name     string                       `json:"name"`
	UrlHttp  string                       `json:"urlHttp"`
	UrlHttps string                       `json:"urlHttps"`
	UrlFtp   string                       `json:"urlFtp"`
	Country  string                       `json:"country"`
	Locale   map[string]map[string]string `json:"locale"`
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
	var mirrorsUrl string
	if url != "" {
		mirrorsUrl = url
	} else {
		// get mirrorsUrl from config file
		data, err := ioutil.ReadFile(filepath.Join(system.VarLibDir, "config.json"))
		if err != nil {
			if os.IsNotExist(err) {
				mirrorsUrl = system.DefaultMirrorsUrl
			} else {
				return nil, err
			}
		} else {
			cfg := struct {
				MirrorsUrl string
			}{system.DefaultMirrorsUrl}

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
			Id:         raw.Id,
			Name:       raw.Name,
			Weight:     raw.Weight,
			NameLocale: make(map[string]string),
			Country:    raw.Country,
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
