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
