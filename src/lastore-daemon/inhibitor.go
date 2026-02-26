// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sync"
	"syscall"

	"github.com/godbus/dbus/v5"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
)

var (
	sharedInhibitMu  sync.Mutex
	sharedInhibitRef int
	sharedInhibitFd  dbus.UnixFD = -1
)

var (
	inhibitorFn    = Inhibitor
	closeInhibitFd = func(fd dbus.UnixFD) error { return syscall.Close(int(fd)) }
)

type methodCaller uint

const (
	methodCallerOtherCaller methodCaller = iota
	methodCallerControlCenter
	methodCallerAppStore
)

func mapMethodCaller(execPath string, cmdLine string) methodCaller {
	logger.Debug("execPath:", execPath, "cmdLine:", cmdLine)
	switch execPath {
	case appStoreDaemonPath, oldAppStoreDaemonPath:
		return methodCallerAppStore
	case controlCenterPath:
		return methodCallerControlCenter
	default:
		switch cmdLine {
		case controlCenterCmdLine:
			return methodCallerControlCenter
		default:
			return methodCallerOtherCaller
		}
	}
}

func (m *Manager) updateSystemOnChanging(onChanging bool, caller methodCaller) {
	if onChanging && m.inhibitFd == -1 {
		var why string
		switch caller {
		case methodCallerControlCenter:
			why = Tr("Installing updates...")
		case methodCallerAppStore:
			why = Tr("Tasks are running...")
		default:
			why = Tr("Preventing from shutting down")
		}
		fd, err := sharedInhibitAcquire(why)
		if err != nil {
			logger.Infof("Prevent shutdown failed: err:%v\n", err)
			return
		}
		logger.Info("Prevent shutdown...")
		m.inhibitFd = fd
	} else if !onChanging && m.inhibitFd != -1 {
		err := sharedInhibitRelease()
		if err != nil {
			logger.Infof("Enable shutdown failed: err:%v\n", err)
		} else {
			logger.Info("Enable shutdown...")
		}
		m.inhibitFd = -1
	}
}

func sharedInhibitAcquire(why string) (dbus.UnixFD, error) {
	sharedInhibitMu.Lock()
	defer sharedInhibitMu.Unlock()

	if sharedInhibitRef == 0 {
		fd, err := inhibitorFn("shutdown:sleep", dbusServiceName, why)
		if err != nil {
			return 0, err
		}
		sharedInhibitFd = fd
	}
	sharedInhibitRef++
	return sharedInhibitFd, nil
}

func sharedInhibitRelease() error {
	sharedInhibitMu.Lock()
	defer sharedInhibitMu.Unlock()

	if sharedInhibitRef == 0 {
		return nil
	}
	sharedInhibitRef--
	if sharedInhibitRef != 0 {
		return nil
	}
	if sharedInhibitFd == -1 {
		return nil
	}
	fd := sharedInhibitFd
	sharedInhibitFd = -1
	return closeInhibitFd(fd)
}

func Inhibitor(what, who, why string) (dbus.UnixFD, error) {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		return 0, err
	}
	m := login1.NewManager(systemConn)
	return m.Inhibit(0, what, who, why, "block")
}
