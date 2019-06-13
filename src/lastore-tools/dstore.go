package main

import (
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/go-ini/ini"
)

const (
	appstoreConfPath        = "/usr/share/deepin-appstore/settings.ini"
	appstoreConfPathDefault = "/usr/share/deepin-appstore/settings.ini.default"
)

type Store struct {
	sysCfg *ini.File
}

func NewStore() *Store {
	var err error
	s := &Store{}

	s.sysCfg, err = ini.Load(appstoreConfPath)
	if err != nil {
		log.Infof("fail to read file: %v", err)
		s.sysCfg, err = ini.Load(appstoreConfPathDefault)
		if err != nil {
			log.Errorf("fail to read file:", err)
		}
	}
	return s
}

func (s *Store) GetMetadataServer() string {
	return s.sysCfg.Section("General").Key("metadataServer").String()
}

type packageInfo struct {
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

type packageApps map[string]*packageInfo

// 获取上架的apt应用信息
func (s *Store) GetPackageApplication(path string) (v apiAppApps, err error) {
	apiAppUrl := s.GetMetadataServer() + "/api/v3/packages"

	packages := make(packageApps)
	err = cacheFetchJSON(&packages, apiAppUrl, path, time.Second*10)

	for dpk, app := range packages {
		app.PackageURI = dpk
		app.PackageName = strings.Replace(dpk, "dpk://deb/", "", -1)
		v.Apps = append(v.Apps, app)
	}
	return
}
