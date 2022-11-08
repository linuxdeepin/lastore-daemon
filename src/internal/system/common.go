// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
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
var logger = log.NewLogger("lastore")

type RepositoryInfo struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Mirror string `json:"mirror"`
}

func DecodeJson(fpath string, d interface{}) error {
	// #nosec G304
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	return json.NewDecoder(f).Decode(&d)
}

func EncodeJson(fpath string, d interface{}) error {
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

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

type UpdateType uint64

// 用于设置UpdateMode属性,最大支持64位
const (
	SystemUpdate       UpdateType = 1 << 0 // 系统更新
	AppStoreUpdate     UpdateType = 1 << 1 // 应用更新(1050版本中应用更新不开启)
	SecurityUpdate     UpdateType = 1 << 2 // 1050及以上版本,安全更新项废弃,改为仅安全更新
	UnknownUpdate      UpdateType = 1 << 3 // 未知来源更新
	OnlySecurityUpdate UpdateType = 1 << 4 // 仅开启安全更新（该选项开启时，其他更新关闭）
)

func (m UpdateType) JobType() string {
	switch m {
	case SystemUpdate:
		return SystemUpgradeJobType
	case AppStoreUpdate:
		return AppStoreUpgradeJobType
	case SecurityUpdate, OnlySecurityUpdate:
		return SecurityUpgradeJobType
	case UnknownUpdate:
		return UnknownUpgradeJobType
	default:
		return ""
	}
}

func AllUpdateType() []UpdateType {
	return []UpdateType{
		SystemUpdate,
		//AppStoreUpdate,
		OnlySecurityUpdate,
		UnknownUpdate,
	}
}

const (
	LastoreSourcesPath = "/var/lib/lastore/sources.list"
	CustomSourceDir    = "/var/lib/lastore/sources.list.d"
	OriginSourceDir    = "/etc/apt/sources.list.d"
	SystemSourceFile   = "/etc/apt/sources.list"
	SystemSourceDir    = "/var/lib/lastore/systemSource.d"
	AppStoreList       = "appstore.list"
	AppStoreSourceFile = "/etc/apt/sources.list.d/" + AppStoreList
	UnstableSourceList = "deepin-unstable-source.list"
	UnstableSourceFile = "/etc/apt/sources.list.d/" + UnstableSourceList
	HweSourceList      = "hwe.list"
	HweSourceFile      = "/etc/apt/sources.list.d/" + HweSourceList
	DriverList         = "driver.list"
	SecurityList       = "security.list"
	SecuritySourceFile = "/etc/apt/sources.list.d/" + SecurityList // 安全更新源路径
	UnknownSourceDir   = "/var/lib/lastore/unknownSource.d"        // 未知来源更新的源个数不定,需要创建软链接放在同一目录内
)

func GetCategorySourceMap() map[UpdateType]string {
	return map[UpdateType]string{
		SystemUpdate: SystemSourceDir,
		//AppStoreUpdate:     AppStoreSourceFile,
		OnlySecurityUpdate: SecuritySourceFile,
		UnknownUpdate:      UnknownSourceDir,
	}
}

func UpdateUnknownSourceDir() error {
	// 移除旧版本内容
	err := os.RemoveAll(CustomSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	err = os.RemoveAll(LastoreSourcesPath)
	if err != nil {
		logger.Warning(err)
	}
	// 移除旧数据
	err = os.RemoveAll(UnknownSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	// #nosec G301
	err = os.MkdirAll(UnknownSourceDir, 0755)
	if err != nil {
		logger.Warning(err)
	}

	var unknownSourceFilePaths []string
	sourceDirFileInfos, err := ioutil.ReadDir(OriginSourceDir)
	if err != nil {
		logger.Warning(err)
		return err
	}
	nonUnknownSourceFileList := strv.Strv{
		AppStoreList,
		SecurityList,
		DriverList,
		UnstableSourceFile,
		HweSourceList,
	}
	for _, fileInfo := range sourceDirFileInfos {
		name := fileInfo.Name()
		if strings.HasSuffix(name, ".list") {
			if !nonUnknownSourceFileList.Contains(name) {
				unknownSourceFilePaths = append(unknownSourceFilePaths, filepath.Join(OriginSourceDir, name))
			}
		}
	}

	// 创建对应的软链接
	for _, filePath := range unknownSourceFilePaths {
		linkPath := filepath.Join(UnknownSourceDir, filepath.Base(filePath))
		err = os.Symlink(filePath, linkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", filePath, err)
		}
	}
	return nil
}

func UpdateSystemSourceDir() error {

	err := os.RemoveAll(SystemSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	// #nosec G301
	err = os.MkdirAll(SystemSourceDir, 0755)
	if err != nil {
		logger.Warning(err)
	}

	var systemSourceFilePaths = []string{UnstableSourceFile, SystemSourceFile, HweSourceFile}

	// 创建对应的软链接
	for _, filePath := range systemSourceFilePaths {
		linkPath := filepath.Join(SystemSourceDir, filepath.Base(filePath))
		err = os.Symlink(filePath, linkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", filePath, err)
		}
	}
	return nil
}
