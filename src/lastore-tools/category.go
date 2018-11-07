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
	"internal/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"

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
	return utils.WriteData(fpath, data)
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
	Id         string            `json:"id"`
	Category   string            `json:"category"`
	Name       string            `json:"name"`
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

	const localeEnUS = "en_US"
	for _, app := range v.Apps {
		appInfo := AppInfo{
			Id:       app.Name,
			Category: app.Category,
		}

		// set Name
		enDesc, ok := app.Locale[localeEnUS]
		if ok {
			appInfo.Name = enDesc.Description.Name
		}
		if appInfo.Name == "" {
			appInfo.Name = app.Name
		}

		// set LocaleName
		for localeCode, desc := range app.Locale {
			localizedName := desc.Description.Name
			if localizedName != appInfo.Name {
				if appInfo.LocaleName == nil {
					appInfo.LocaleName = make(map[string]string)
				}
				appInfo.LocaleName[localeCode] = localizedName
			}
		}

		apps[app.Name] = appInfo
	}

	if _, hasAppstore := apps["deepin-appstore"]; !hasAppstore {
		apps["deepin-appstore"] = AppInfo{
			Id:       "deepin-appstore",
			Category: "system",
			Name:     "deepin store",
			LocaleName: map[string]string{
				"zh_CN": "深度商店",
			},
		}
	}

	return writeData(fpath, apps)
}

func GenerateApplications(repo, fpath string) error {
	appJsonFile := filepath.Join(filepath.Dir(fpath), "app.json")

	tempFile := appJsonFile + ".tmp"
	output, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	defer output.Close()

	apiAppUrl := "https://dstore-metadata.deepin.cn/api/app"

	resp, err := http.Get(apiAppUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = output.Chmod(0644)
	if err != nil {
		return err
	}

	teeReader := io.TeeReader(resp.Body, output)
	jsonDec := json.NewDecoder(teeReader)
	var v apiAppApps
	err = jsonDec.Decode(&v)
	if err != nil {
		return err
	}

	err = os.Rename(tempFile, appJsonFile)
	if err != nil {
		return err
	}
	err = genApplications(v, fpath)
	if err != nil {
		return err
	}

	return nil
}
