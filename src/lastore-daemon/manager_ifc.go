// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strconv"

	"github.com/godbus/dbus"
	agent "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore.agent"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (m *Manager) RegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	logger.Info("RegisterAgent:", path)
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	uidStr := strconv.Itoa(int(uid))
	m.userAgents.addUser(uidStr)

	sessionDetails, err := m.loginManager.ListSessions(0)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	sysBus := m.service.Conn()
	for _, detail := range sessionDetails {
		if detail.UID == uid {
			session, err := login1.NewSession(sysBus, detail.Path)
			if err != nil {
				logger.Warning(err)
				continue
			}
			newlyAdded := m.userAgents.addSession(uidStr, session)
			if newlyAdded {
				m.watchSession(uidStr, session)
			}
		}
	}

	a, err := agent.NewAgent(m.service.Conn(), string(sender), path)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	m.userAgents.addAgent(uidStr, a)
	return nil
}

func (m *Manager) UnRegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	uidStr := strconv.Itoa(int(uid))
	err = m.userAgents.removeAgent(uidStr, path)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	logger.Debugf("agent unregistered, sender: %q, agentPath: %q", sender, path)
	return nil
}
