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
	"compress/gzip"
	"encoding/json"
	"internal/utils"
	"io/ioutil"
	"net/http"

	log "github.com/cihub/seelog"
)

const appstoreURI = "http://api.appstore.deepin.org"

type CategoryInfo struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Locales map[string]struct {
		Name string `json:"name"`
	}
}

func decodeData(wrap bool, url string, data interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return log.Warnf("can't get %q \n", url)
	}
	defer resp.Body.Close()
	d := json.NewDecoder(resp.Body)

	if wrap {
		var wrap struct {
			StatusCode    int         `json:"status_code"`
			StatusMessage string      `json:"status_message"`
			Data          interface{} `json:"data"`
		}
		wrap.Data = data
		err = d.Decode(&wrap)
	} else {
		err = d.Decode(&data)
	}

	if err != nil {
		return err
	}
	return nil
}

func writeData(fpath string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	utils.EnsureBaseDir(fpath)
	return ioutil.WriteFile(fpath, content, 0644)
}

func GenerateCategory(repo, fpath string) error {
	url := appstoreURI + "/" + "categories"

	var d []CategoryInfo
	err := decodeData(true, url, &d)
	if err != nil {
		return log.Warnf("GenerateCategory failed %v\n", err)
	}
	return writeData(fpath, d)
}

type AppInfo struct {
	Category   string            `json:"category"`
	LocaleName map[string]string `json:"locale_name"`
}

type apiAppApps struct {
	Apps []struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Locale   map[string]struct {
			Description struct {
				Name string `json:"name"`
			} `json:"description"`
		} `json:"locale"`
	} `json:"apps"`
}

func genApplications(v apiAppApps, fpath string) error {
	apps := make(map[string]AppInfo)

	for _, app := range v.Apps {
		appInfo := AppInfo{
			Category: app.Category,
		}

		// set LocaleName
		appInfo.LocaleName = make(map[string]string)
		for localeCode, desc := range app.Locale {
			localizedName := desc.Description.Name
			appInfo.LocaleName[localeCode] = localizedName
		}

		apps[app.Name] = appInfo
	}

	return writeData(fpath, apps)
}

func GenerateApplications(repo, fpath string) error {
	apiAppUrl := "https://dstore-metadata.deepin.cn/api/app?query={apps{name,category,locale{en_US{description{name}},zh_CN{description{name}}}}}"
	client := http.DefaultClient
	request, err := http.NewRequest("GET", apiAppUrl, nil)
	request.Header.Add("Accept-Encoding", "gzip")

	resp, err := client.Do(request)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}

	jsonDec := json.NewDecoder(gzipReader)
	var v apiAppApps
	err = jsonDec.Decode(&v)
	if err != nil {
		return err
	}

	err = genApplications(v, fpath)
	if err != nil {
		return err
	}

	return nil
}
