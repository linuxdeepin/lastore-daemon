package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
	"gopkg.in/yaml.v2"
)

// DefaultDynHookTimeout is the fallback hook timeout in seconds.
const DefaultDynHookTimeout = 1800

// CoreConfig holds the upgrade check configuration. yaml.v2 does not interpret
// default tags; defaults are defined in defaultCoreConfig and applied by LoaderCfg.
type CoreConfig struct {
	CacheList      string `json:"CacheList" yaml:"CacheList"`           // cachelist
	Base           string `json:"Base" yaml:"Base"`                     // work base
	DebugMode      bool   `json:"DebugMode" yaml:"DebugMode"`           // Debug Mode
	ApiVersion     string `json:"ApiVersion" yaml:"ApiVersion"`         // ApiVersion
	DynHookTimeout int    `json:"DynHookTimeout" yaml:"DynHookTimeout"` // dyn hook timeout in seconds
}

func defaultCoreConfig() CoreConfig {
	return CoreConfig{
		CacheList:      "caches.yaml",
		Base:           "/var/lib/lastore/check/",
		DebugMode:      false,
		DynHookTimeout: DefaultDynHookTimeout,
	}
}

// LoadOrCreateCfg restores a missing configuration without replacing existing files.
// It returns true only when it successfully creates the configuration.
func (ts *CoreConfig) LoadOrCreateCfg(path string) (bool, error) {
	if err := ts.LoaderCfg(path); !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	defaults := defaultCoreConfig()
	output, err := yaml.Marshal(&defaults)
	if err != nil {
		return false, fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("create config directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		if retryErr := ts.LoaderCfg(path); retryErr != nil {
			return false, fmt.Errorf("retry load after race on config create: %w", retryErr)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create default config: %w", err)
	}
	_, writeErr := file.Write(output)
	closeErr := file.Close()
	if writeErr != nil {
		return false, fmt.Errorf("write default config: %w", writeErr)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close default config: %w", closeErr)
	}
	*ts = defaults
	return true, nil
}

func (ts *CoreConfig) LoaderCfg(path string) error {
	cfgRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("loader/config read config failed: %w", err)
	}
	var loaded CoreConfig
	if err := yaml.Unmarshal(cfgRaw, &loaded); err != nil {
		return fmt.Errorf("loader/config copy config failed: %w", err)
	}
	defaults := defaultCoreConfig()
	if loaded.Base == "" {
		loaded.Base = defaults.Base
	}
	if loaded.CacheList == "" {
		loaded.CacheList = defaults.CacheList
	}
	if loaded.DynHookTimeout <= 0 {
		loaded.DynHookTimeout = defaults.DynHookTimeout
	}
	*ts = loaded
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
