package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
	"gopkg.in/yaml.v2"
)

type CoreConfig struct {
	CacheList  string `json:"CacheList" yaml:"CacheList"`                 // cachelist
	Base       string `json:"Base" yaml:"Base"`                           // work base
	DebugMode  bool   `json:"DebugMode" yaml:"DebugMode" default:"false"` // Debug Mode
	ApiVersion string `json:"ApiVersion" yaml:"ApiVersion" default:"1.0"` // ApiVersion
}

func (ts *CoreConfig) LoaderCfg(path string) error {

	if _, err := os.Stat(path); err != nil {
		return err
	}
	cfgRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("loader/config read config failed: %v", err)
	}
	if err := yaml.Unmarshal(cfgRaw, &ts); err != nil {
		return fmt.Errorf("loader/config copy config failed: %v", err)
	}
	return nil
}

func (ts *CoreConfig) UpdateCfg(path string) error {

	output, err := yaml.Marshal(&ts)

	if err != nil {
		return fmt.Errorf("flush/config convert config failed: %v", err)
	}
	err = ioutil.WriteFile(path, output, 0644)
	if err != nil {
		return fmt.Errorf("flush/config save cache failed: %v", err)
	}

	return nil
}

func (ts *CoreConfig) LoaderCache(cachecfg *cache.CacheConfig) error {

	cacheFileName := ts.Base + "/" + ts.CacheList
	if _, err := os.Stat(cacheFileName); err != nil {
		//return err
		if _, err := fs.CreateFile(cacheFileName); err != nil {
			return err
		}
	}
	cfgRaw, err := ioutil.ReadFile(cacheFileName)
	if err != nil {
		return fmt.Errorf("loader/cache: can not read file: %v", err)
	}

	if err := yaml.Unmarshal(cfgRaw, &cachecfg); err != nil {
		return fmt.Errorf("loader/cache: load failed: %v", err)
	}

	return nil
}

func (ts *CoreConfig) UpdateCache(cachecfg *cache.CacheConfig) error {

	// cachecfg.UpdateTime = time.Now().Format(time.RFC3339)

	output, err := yaml.Marshal(&cachecfg)
	if err != nil {
		return fmt.Errorf("flush/cache convert config failed: %v", err)
	}
	err = ioutil.WriteFile(ts.Base+"/"+ts.CacheList, output, 0644)
	if err != nil {
		return fmt.Errorf("flush/cache save cache failed: %v", err)
	}

	return nil
}

// load config to cache
func (ts *CoreConfig) LoaderCfgCache(path string, cachecfg *cache.CacheConfig) error {
	if err := ts.LoaderCfg(path); err != nil {
		return fmt.Errorf("core config load failed :%v", err)
	}
	if err := ts.LoaderCache(cachecfg); err != nil {
		return fmt.Errorf("load cache failed:%v", err)
	}
	return nil
}
