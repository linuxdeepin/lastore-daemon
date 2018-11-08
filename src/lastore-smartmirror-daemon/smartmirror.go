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
	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

// SmartMirror handle core smart mirror data
type SmartMirror struct {
	service *dbusutil.Service

	methods *struct {
		Query func() `in:"url" out:"url"`
	}
}

// GetInterfaceName export dbus interface name
func (s *SmartMirror) GetInterfaceName() string {
	return "com.deepin.lastore.Smartmirror"
}

// NewSmartMirror return a object with dbus
func NewSmartMirror(service *dbusutil.Service) *SmartMirror {
	u := &SmartMirror{
		service: service,
	}
	return u
}

// Query the best source
func (s *SmartMirror) Query(url string) (string, *dbus.Error) {
	log.Info(url)
	return url, nil
}
