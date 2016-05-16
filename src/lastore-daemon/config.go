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

type Config struct {
	AutoCheckUpdates bool
	MirrorSource     string
	CheckInterval    time.Duration
	AppstoreRegion   string
	LastCheckTime    time.Time
	Repository       string

	fpath string
}

func NewConfig(fpath string) *Config {
	r := Config{
		CheckInterval:    time.Minute * 180,
		AutoCheckUpdates: true,
		fpath:            fpath,
	}

	err := system.DecodeJson(fpath, &r)
	if err != nil {
		log.Warnf("Can't load config file: %v\n", err)
	}

	if r.CheckInterval < MinCheckInterval {
		r.CheckInterval = MinCheckInterval
	}
	if r.Repository == "" || r.MirrorSource == "" {
		info := system.DetectDefaultRepoInfo(system.RepoInfos)
		r.Repository = info.Name
		r.MirrorSource = "default" //info.Mirror
	}

	return &r
}

func (c *Config) UpdateLastCheckTime() error {
	c.LastCheckTime = time.Now()
	return c.save()
}

func (c *Config) SetAutoCheckUpdates(enable bool) error {
	c.AutoCheckUpdates = enable
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
	return system.EncodeJson(c.fpath, c)
}
