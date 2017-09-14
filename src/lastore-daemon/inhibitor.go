/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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
	"dbus/org/freedesktop/login1"
	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/gettext"
	"syscall"
)

func (m *Manager) updateSystemOnChaning(onChanging bool) {
	if onChanging && m.inhibitFd == -1 {
		fd, err := Inhibitor("shutdown", gettext.Tr("Deepin Store"),
			gettext.Tr("System is updating, please shut down or reboot later."))
		log.Infof("Prevent shutdown...: fd:%v\n", fd)
		if err != nil {
			log.Infof("Prevent shutdown failed: fd:%v, err:%v\n", fd, err)
			return
		}
		m.inhibitFd = fd
	} else if !onChanging && m.inhibitFd != -1 {
		err := syscall.Close(int(m.inhibitFd))
		if err != nil {
			log.Infof("Enable shutdown...: fd:%d, err:%s\n", m.inhibitFd, err)
		} else {
			log.Infof("Enable shutdown...")
		}
		m.inhibitFd = -1
	}
}

func Inhibitor(what, who, why string) (dbus.UnixFD, error) {
	m, err := login1.NewManager("org.freedesktop.login1", "/org/freedesktop/login1")
	if err != nil {
		return -1, err
	}
	defer login1.DestroyManager(m)
	return m.Inhibit(what, who, why, "block")
}
