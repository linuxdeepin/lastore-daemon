package dstore

import (
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/go-ini/ini"
)

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
		log.Infof("fail to read file: %v", err)
		s.sysCfg, err = ini.Load(appstoreConfPathDefault)
		if err != nil {
			log.Errorf("fail to read file:", err)
		}
	}
	return s
}

func (s *Store) GetMetadataServer() string {
	metadataServer := s.sysCfg.Section("General").Key("Server").String()
	if metadataServer == "" {
		metadataServer = "https://store.chinauos.com"
	}
	return metadataServer
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

// 获取上架的apt应用信息
func (s *Store) GetPackageApplication(path string) (v []*PackageInfo, err error) {
	// cachePath := filepath.Join(system.VarLibDir, "packages.cache.json")
	cachePath := path + ".cache.json"
	apiAppURL := s.GetMetadataServer() + "/api/public/packages"

	packages := make(packageApps)
	err = cacheFetchJSON(&packages, apiAppURL, cachePath, expireDelay)

	for dpk, app := range packages {
		app.PackageURI = dpk
		app.PackageName = strings.Replace(dpk, "dpk://deb/", "", -1)
		v = append(v, app)
	}
	return
}
