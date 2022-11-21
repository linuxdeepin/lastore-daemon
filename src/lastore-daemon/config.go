// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"internal/system"
	"path/filepath"
	"time"
)

const MinCheckInterval = time.Minute
const ConfigVersion = "0.1"

var DefaultConfig = Config{
	CheckInterval:               time.Hour * 24 * 7,
	CleanInterval:               time.Hour * 24 * 7,
	CleanIntervalCacheOverLimit: time.Hour * 24,
	AutoCheckUpdates:            true,
	DisableUpdateMetadata:       false,
	AutoDownloadUpdates:         false,
	UpdateNotify:                true,
	AutoClean:                   true,
	MirrorsUrl:                  system.DefaultMirrorsUrl,
	UpdateMode:                  system.SystemUpdate | system.SecurityUpdate, // 默认打开系统更新和安全更新

	AutoInstallUpdates:    false,
	AutoInstallUpdateType: system.OnlySecurityUpdate, // 开启状态下,默认只开启安全更新的自动安装

	AllowPostSystemUpgradeMessageVersion: []string{"Professional"},
}

type Config struct {
	Version               string
	AutoCheckUpdates      bool
	DisableUpdateMetadata bool
	AutoDownloadUpdates   bool
	AutoClean             bool
	MirrorSource          string
	UpdateNotify          bool
	CheckInterval         time.Duration
	CleanInterval         time.Duration
	UpdateMode            system.UpdateType

	// 缓存大小超出限制时的清理时间间隔
	CleanIntervalCacheOverLimit    time.Duration
	AppstoreRegion                 string
	LastCheckTime                  time.Time
	LastCleanTime                  time.Time
	LastCheckCacheSizeTime         time.Time
	Repository                     string
	MirrorsUrl                     string
	filePath                       string
	AllowInstallRemovePkgExecPaths []string
	AutoInstallUpdates             bool
	AutoInstallUpdateType          system.UpdateType

	AllowPostSystemUpgradeMessageVersion []string //只有数组内的系统版本被允许发送更新完成的数据

	UpgradeStatus system.UpgradeStatusAndReason
}

func NewConfig(fpath string) *Config {
	c := getDefaultConfig()
	err := system.DecodeJson(fpath, &c)
	if err != nil {
		logger.Debugf("Can't load config file: %v\n", err)
	}
	c.filePath = fpath

	if c.CheckInterval < MinCheckInterval {
		c.CheckInterval = MinCheckInterval
	}
	if c.Repository == "" || c.MirrorSource == "" {
		info := system.DetectDefaultRepoInfo(system.RepoInfos)
		c.Repository = info.Name
		c.MirrorSource = "default" //info.Mirror
	}
	if c.Version == "" {
		c.Version = ConfigVersion
		c.CheckInterval = time.Hour * 24 * 7
		c.CleanInterval = time.Hour * 24 * 7
		_ = c.save()
	}
	return c
}

func getDefaultConfig() *Config {
	var c *Config
	defaultConfigPath := filepath.Join(system.VarLibDir, "default_config.json")
	err := system.DecodeJson(defaultConfigPath, &c)
	if err != nil {
		logger.Debugf("Can't load default config file: %v\n", err)
		c = &DefaultConfig
	}
	return c
}

func (c *Config) UpdateLastCheckTime() error {
	c.LastCheckTime = time.Now()
	return c.save()
}

func (c *Config) UpdateLastCleanTime() error {
	c.LastCleanTime = time.Now()
	return c.save()
}

func (c *Config) UpdateLastCheckCacheSizeTime() error {
	c.LastCheckCacheSizeTime = time.Now()
	return c.save()
}

func (c *Config) SetAutoCheckUpdates(enable bool) error {
	c.AutoCheckUpdates = enable
	return c.save()
}

func (c *Config) SetUpdateNotify(enable bool) error {
	c.UpdateNotify = enable
	return c.save()
}

func (c *Config) SetAutoDownloadUpdates(enable bool) error {
	c.AutoDownloadUpdates = enable
	return c.save()
}

func (c *Config) SetAutoClean(enable bool) error {
	c.AutoClean = enable
	return c.save()
}

func (c *Config) SetMirrorSource(id string) error {
	c.MirrorSource = id
	return c.save()
}

func (c *Config) SetAppstoreRegion(region string) error {
	c.AppstoreRegion = region
	return c.save()
}

func (c *Config) SetUpdateMode(mode system.UpdateType) error {
	c.UpdateMode = mode
	return c.save()
}

func (c *Config) SetAutoInstallUpdates(autoInstall bool) error {
	c.AutoInstallUpdates = autoInstall
	return c.save()
}

func (c *Config) SetAutoInstallUpdateType(updateType system.UpdateType) error {
	c.AutoInstallUpdateType = updateType
	return c.save()
}

func (c *Config) SetUpgradeStatusAndReason(status system.UpgradeStatusAndReason) error {
	c.UpgradeStatus = status
	return c.save()
}

func (c *Config) save() error {
	return system.EncodeJson(c.filePath, c)
}
