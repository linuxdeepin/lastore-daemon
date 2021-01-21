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

package main

import (
	"internal/system"
	"time"

	log "github.com/cihub/seelog"
)

const MinCheckInterval = time.Minute
const ConfigVersion = "0.1"

var DefaultConfig = Config{
	CheckInterval:               time.Hour * 24 * 7,
	CleanInterval:               time.Hour * 24 * 7,
	CleanIntervalCacheOverLimit: time.Hour * 24,
	AutoCheckUpdates:            true,
	DisableUpdateMetadata:       false,
	AutoDownloadUpdates:         true,
	UpdateNotify:                true,
	AutoClean:                   true,
	MirrorsUrl:                  system.DefaultMirrorsUrl,
	UpdateMode:                  SystemUpdate | AppStoreUpdate, // 默认开启系统更新和应用更新,安全更新暂不默认开启
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
	UpdateMode            uint64

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
}

func NewConfig(fpath string) *Config {
	c := DefaultConfig
	err := system.DecodeJson(fpath, &c)
	if err != nil {
		_ = log.Warnf("Can't load config file: %v\n", err)
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
	return &c
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

func (c *Config) SetUpdateMode(mode uint64) error {
	c.UpdateMode = mode
	return c.save()
}

func (c *Config) save() error {
	return system.EncodeJson(c.filePath, c)
}
