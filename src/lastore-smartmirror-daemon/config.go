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

	log "github.com/cihub/seelog"
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
		log.Warnf("Can't load config file: %v\n", err)
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
