// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

var defaultConfig = config{
	Enable: true,
}

type config struct {
	Enable   bool
	filePath string
}

func newConfig(fpath string) *config {
	c := defaultConfig
	err := system.DecodeJson(fpath, &c)
	if err != nil {
		logger.Warningf("Can't load config file: %v\n", err)
	}
	c.filePath = fpath
	return &c
}

func (c *config) setEnable(enable bool) error {
	c.Enable = enable
	return c.save()
}

func (c *config) save() error {
	return system.EncodeJson(c.filePath, c)
}
