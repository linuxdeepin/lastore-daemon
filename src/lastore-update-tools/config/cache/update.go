package cache

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
	"gopkg.in/yaml.v2"
)

var logger = log.NewLogger("lastore/update-tools/config/cache")

// TODO deprecated
type CacheConfig struct {
	Cache map[string]CacheInfo `json:"CacheConfig" yaml:"CacheConfig"` // CacheCfg
}

type CacheInfo struct {
	UUID           string        `json:"UUID" yaml:"UUID"`                                           // uuid
	CachePath      string        `json:"CachePath" yaml:"CachePath"`                                 // path with cache.yaml
	Status         string        `json:"Status" yaml:"Status"`                                       // status
	Type           string        `json:"Type" yaml:"Type" default:""`                                // type with fix/update/secrity
	UpdateMetaInfo UpdateInfo    `json:"UpdateInfo" yaml:"UpdateInfo"`                               // updateinfo
	InternalState  InternalState `json:"State" yaml:"State"`                                         // state
	Verbose        bool          `json:"Verbose,omitempty" yaml:"Verbose,omitempty" default:"false"` // verbose
	UpdateTime     string        `json:"UpdateTime,omitempty" yaml:"UpdateTime,omitempty"`           // time
	WorkStation    string        `json:"WorkStation" yaml:"WorkStation"`                             // WorkStation
	ApiVersion     string        `json:"ApiVersion" yaml:"ApiVersion" default:"1.0"`                 // ApiVersion
}

func (ts *CacheConfig) Loader(path string) error {

	if _, err := os.Stat(path); err != nil {
		return err
	}
	cfgRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("loader read config failed: %v", err)
	}

	if err := yaml.Unmarshal(cfgRaw, &ts); err != nil {
		return fmt.Errorf("loader copy config failed: %v", err)
	}

	return nil
}

func (ts *CacheConfig) LoaderCacheInfoWithUpdateMetaInfo(path, uuid string, cache CacheInfo) error {
	return nil
}

func (ts *CacheConfig) UpdateUUID(path, uuid string, cache CacheInfo) error {

	if _, ok := ts.Cache[uuid]; ok {
		return fmt.Errorf("%s not exists", uuid)
	} else {
		ts.Cache[uuid] = cache
	}
	output, err := yaml.Marshal(&ts)

	if err != nil {
		return fmt.Errorf("uuid convert config failed: %v", err)
	}

	err = ioutil.WriteFile(path, output, 0644)
	if err != nil {
		return fmt.Errorf("uuid save cache failed: %v", err)
	}

	return nil
}

func (ts *CacheInfo) ClearUUID(path, uuid string) error {
	logger.Debugf("clear uuid: %s", uuid)
	archiveFile := fmt.Sprintf("%s/%s-archive.tar.gz", path, uuid)
	if err := fs.CheckFileExistState(archiveFile); err == nil {
		logger.Debugf("remove archive: %s", archiveFile)
		os.RemoveAll(archiveFile)
	}
	workUUID := path + "/" + uuid
	if err := fs.CheckFileExistState(workUUID); err == nil {
		logger.Debugf("remove path: %s", workUUID)
		os.RemoveAll(workUUID)
	}
	return nil
}
