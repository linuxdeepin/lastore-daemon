// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/dstore"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
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
