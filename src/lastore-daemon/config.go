/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	log "github.com/cihub/seelog"
	"internal/system"
	"time"
)

const MinCheckInterval = time.Minute

var DefaultConfig = Config{
	CheckInterval:       time.Minute * 180,
	AutoCheckUpdates:    true,
	AutoDownloadUpdates: false,
}

type Config struct {
	AutoCheckUpdates    bool
	AutoDownloadUpdates bool
	MirrorSource        string
	CheckInterval       time.Duration
	AppstoreRegion      string
	LastCheckTime       time.Time
	Repository          string

	filePath string
}

func NewConfig(fpath string) *Config {
	c := DefaultConfig
	err := system.DecodeJson(fpath, &c)
	if err != nil {
		log.Warnf("Can't load config file: %v\n", err)
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

	return &c
}

func (c *Config) UpdateLastCheckTime() error {
	c.LastCheckTime = time.Now()
	return c.save()
}

func (c *Config) SetAutoCheckUpdates(enable bool) error {
	c.AutoCheckUpdates = enable
	return c.save()
}

func (c *Config) SetAutoDownloadUpdates(enable bool) error {
	c.AutoDownloadUpdates = enable
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

func (c *Config) save() error {
	return system.EncodeJson(c.filePath, c)
}
