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
	"syscall"

	log "github.com/cihub/seelog"
	"github.com/godbus/dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
)

type methodCaller uint

const (
	methodCallerOtherCaller methodCaller = iota
	methodCallerControlCenter
	methodCallerAppStore
)

func mapMethodCaller(execPath string) methodCaller {
	switch execPath {
	case appStoreDaemonPath, oldAppStoreDaemonPath:
		return methodCallerAppStore
	case controlCenterPath:
		return methodCallerControlCenter
	default:
		return methodCallerOtherCaller
	}
}

func (m *Manager) updateSystemOnChanging(onChanging bool, caller methodCaller) {
	if onChanging && m.inhibitFd == -1 {
		var why string
		switch caller {
		case methodCallerControlCenter:
			why = Tr("Updating the system, please do not shut down or reboot now.")
		case methodCallerAppStore:
			why = Tr("Tasks are running...")
		default:
			why = Tr("Preventing from shutting down")
		}
		fd, err := Inhibitor("shutdown", dbusServiceName, why)
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
	systemConn, err := dbus.SystemBus()
	if err != nil {
		return 0, err
	}
	m := login1.NewManager(systemConn)
	return m.Inhibit(0, what, who, why, "block")
}
