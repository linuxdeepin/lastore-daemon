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

package system

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/cihub/seelog"
)

type MirrorSource struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Url  string `json:"url"`

	NameLocale  map[string]string `json:"name_locale"`
	Weight      int               `json:"weight"`
	Country     string            `json:"country"`
	AdjustDelay int               `json:"adjust_delay"` // ms
}

var RepoInfos []RepositoryInfo

type RepositoryInfo struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Mirror string `json:"mirror"`
}

func DecodeJson(fpath string, d interface{}) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&d)
}

func EncodeJson(fpath string, d interface{}) error {
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(d)
}

func NormalFileExists(fpath string) bool {
	info, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return true
}

func SystemUpgradeInfo() ([]UpgradeInfo, error) {
	filename := path.Join(VarLibDir, "update_infos.json")
	var r []UpgradeInfo
	err := DecodeJson(filename, &r)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}

		var updateInfoErr UpdateInfoError
		err2 := DecodeJson(filename, &updateInfoErr)
		if err2 == nil {
			return nil, &updateInfoErr
		}
		return nil, fmt.Errorf("Invalid update_infos: %v\n", err)
	}
	return r, nil
}

// 用于设置UpdateMode属性,最大支持64位
const (
	SystemUpdate   = 1 << 0 // 系统更新
	AppStoreUpdate = 1 << 1 // 应用更新
	SecurityUpdate = 1 << 2 // 安全更新
)

func UpdateCustomSourceDir(mode uint64) error {
	const (
		lastoreSourcesPath = "/var/lib/lastore/sources.list"
		customSourceDir    = "/var/lib/lastore/sources.list.d"
		sourceListPath     = "/etc/apt/sources.list"
		originSourceDir    = "/etc/apt/sources.list.d"
	)

	// 移除旧的sources.list.d内容,再根据最新配置重新填充
	err := os.RemoveAll(customSourceDir)
	if err != nil {
		_ = log.Warn(err)
	}
	err = os.MkdirAll(customSourceDir, 0755)
	if err != nil {
		_ = log.Warn(err)
	}

	// 移除旧的sources.list,再根据最新配置重新创建链接
	err = os.Remove(lastoreSourcesPath)
	if err != nil {
		if !os.IsNotExist(err) {
			_ = log.Warn(err)
		}
	}

	var customSourceFilePaths []string
	if mode&SystemUpdate == SystemUpdate {
		customSourceFilePaths = append(customSourceFilePaths, sourceListPath)
	}
	sourceDirFileInfos, err := ioutil.ReadDir(originSourceDir)
	if err != nil {
		_ = log.Warn(err)
	}
	for _, fileInfo := range sourceDirFileInfos {
		name := fileInfo.Name()
		if strings.HasSuffix(name, ".list") {
			switch name {
			case "appstore.list":
				if mode&AppStoreUpdate == AppStoreUpdate {
					customSourceFilePaths = append(customSourceFilePaths, filepath.Join(originSourceDir, name))
				}
			case "safe.list":
				if mode&SecurityUpdate == SecurityUpdate {
					customSourceFilePaths = append(customSourceFilePaths, filepath.Join(originSourceDir, name))
				}
			default:
				if mode&SystemUpdate == SystemUpdate {
					customSourceFilePaths = append(customSourceFilePaths, filepath.Join(originSourceDir, name))
				}
			}
		}
	}

	// 创建对应的软链接
	for _, customFilePath := range customSourceFilePaths {
		var customFileLinkPath string
		if customFilePath == sourceListPath {
			customFileLinkPath = lastoreSourcesPath
		} else {
			customFileLinkPath = filepath.Join(customSourceDir, filepath.Base(customFilePath))
		}

		err = os.Symlink(customFilePath, customFileLinkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", customFileLinkPath, err)
		}
	}
	return nil
}
