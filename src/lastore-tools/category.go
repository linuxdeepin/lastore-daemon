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
	"internal/dstore"
	"internal/utils"
	"net/http"

	log "github.com/cihub/seelog"
)

// type CategoryInfo struct {
// 	Id      string `json:"id"`
// 	Name    string `json:"name"`
// 	Locales map[string]struct {
// 		Name string `json:"name"`
// 	}
// }

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
	log.Warnf("this method has deprecated")
	return nil
	// appstoreURI := "http://api.appstore.deepin.org"
	// url := appstoreURI + "/" + "categories"

	// var d []CategoryInfo
	// err := decodeData(true, url, &d)
	// if err != nil {
	// 	return log.Warnf("GenerateCategory failed %v\n", err)
	// }
	// return writeData(fpath, d)
}

func genApplications(v []*dstore.PackageInfo, fpath string) error {
	apps := make(map[string]*dstore.AppInfo)

	for _, app := range v {
		appInfo := &dstore.AppInfo{
			Category:    app.Category,
			PackageName: app.PackageName,
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
	s := dstore.NewStore()

	list, err := s.GetPackageApplication(fpath)
	if err != nil {
		return err
	}

	err = genApplications(list, fpath)
	if err != nil {
		return err
	}

	return nil
}
