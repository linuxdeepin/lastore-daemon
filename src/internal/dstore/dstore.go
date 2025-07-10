// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dstore

import (
	"errors"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/dstore")

const (
	appstoreConfPath        = "/usr/share/deepin-app-store/settings.ini"
	appstoreConfPathDefault = "/usr/share/deepin-app-store/settings.ini.default"
)

type Store struct {
	sysCfg *ini.File
}

func NewStore() *Store {
	var err error
	s := &Store{}

	s.sysCfg, err = ini.Load(appstoreConfPath)
	if err != nil {
		logger.Infof("fail to read file: %v", err)
		s.sysCfg, err = ini.Load(appstoreConfPathDefault)
		if err != nil {
			logger.Errorf("fail to read file:", err)
		}
	}
	return s
}

func (s *Store) GetMetadataServer() (string, error) {
	var metadataServer string
	if s.sysCfg != nil {
		metadataServer = s.sysCfg.Section("General").Key("Server").String()
	}
	if metadataServer == "" {
		return "", errors.New("no metadata server")
	}
	return metadataServer, nil
}

type AppInfo struct {
	Category    string            `json:"category"`
	Name        string            `json:"name"`
	PackageName string            `json:"package_name"`
	LocaleName  map[string]string `json:"locale_name"`
}

type PackageInfo struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	PackageURI  string `json:"packageURI"`
	PackageName string `json:"package_name"`
	Locale      map[string]struct {
		Description struct {
			Name string `json:"name"`
		} `json:"description"`
	} `json:"locale"`
}

var expireDelay = time.Hour * 24

type packageApps map[string]*PackageInfo

// GetPackageApplication 通过元数据服务器获取商店软件包信息，会把数据缓存在 path + .cache.json 文件中，
// 缓存过期时长由 expireDelay 确定。
func (s *Store) GetPackageApplication(path string) (v []*PackageInfo, err error) {
	// cachePath := filepath.Join(system.VarLibDir, "packages.cache.json")
	cachePath := path + ".cache.json"
	server, err := s.GetMetadataServer()
	if err != nil {
		return nil, err
	}
	apiAppURL := server + "/api/public/packages"

	packages := make(packageApps)
	err = cacheFetchJSON(&packages, apiAppURL, cachePath, expireDelay)
	if err != nil {
		return nil, err
	}
	for dpk, app := range packages {
		app.PackageURI = dpk
		app.PackageName = strings.Replace(dpk, "dpk://deb/", "", -1)
		v = append(v, app)
	}
	return
}
