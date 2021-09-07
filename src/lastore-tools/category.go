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
	"internal/dstore"
	"internal/utils"
)

// writeData 把数据 data 序列化为 JSON 格式写入 fpath 路径的文件。
func writeData(fpath string, data interface{}) error {
	return utils.WriteData(fpath, data)
}

// 废弃
func GenerateCategory(repo, fpath string) error {
	logger.Warningf("this method has deprecated")
	return nil
}

// genApplications 把 v 中数据格式转换，再保存在 fpath 路径的文件中，以 JSON 格式。
func genApplications(v []*dstore.PackageInfo, fpath string) error {
	apps := make(map[string]*dstore.AppInfo)

	for _, app := range v {
		appInfo := &dstore.AppInfo{
			Category:    app.Category,
			Name:        app.Name,
			PackageName: app.PackageName,
		}

		// set LocaleName
		appInfo.LocaleName = make(map[string]string)
		for localeCode, desc := range app.Locale {
			localizedName := desc.Description.Name
			appInfo.LocaleName[localeCode] = localizedName
		}

		apps[app.PackageName] = appInfo
	}

	return writeData(fpath, apps)
}

// GenerateApplications 在 fpath 路径生成 applications.json 文件，此文件内容为上架应用的信息。
// fpath 一般为/var/lib/lastore/applications.json 。
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
