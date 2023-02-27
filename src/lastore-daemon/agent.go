// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"internal/utils"
	"io/ioutil"
	"sync"

	"github.com/godbus/dbus"
	lastoreAgent "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore.agent"
	dbus2 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/strv"
)

const userAgentRecordPath = "/tmp/lastoreAgentCache"

type userAgentMap struct {
	mu         sync.Mutex
	uidItemMap map[string]*sessionAgentMapItem // key 是 uid
	activeUid  string
}

type sessionAgentMapItem struct {
	sessions map[dbus.ObjectPath]login1.Session     // key 是 session 的路径
	agents   map[dbus.ObjectPath]lastoreAgent.Agent // key 是 agent 的路径
	lang     string
}

func newUserAgentMap(service *dbusutil.Service) *userAgentMap {
	u := recoverLastoreAgents(userAgentRecordPath, service)
	if u != nil {
		logger.Info("recover agent from", userAgentRecordPath)
		return u
	}
	return &userAgentMap{
		uidItemMap: make(map[string]*sessionAgentMapItem, 1),
	}
}

func (m *userAgentMap) addAgent(uid string, agent lastoreAgent.Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.uidItemMap[uid]
	if ok {
		if item.agents == nil {
			item.agents = make(map[dbus.ObjectPath]lastoreAgent.Agent)
		}
		if len(item.agents) > 10 {
			// 限制数量
			return
		}
		item.agents[agent.Path_()] = agent
	} else {
		m.uidItemMap[uid] = &sessionAgentMapItem{
			agents: map[dbus.ObjectPath]lastoreAgent.Agent{
				agent.Path_(): agent,
			},
		}
	}
}

func (m *userAgentMap) removeAgent(uid string, agentPath dbus.ObjectPath) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.uidItemMap[uid]
	if !ok {
		return errors.New("invalid uid")
	}

	if _, ok := item.agents[agentPath]; !ok {
		return errors.New("invalid agent path")
	}
	delete(item.agents, agentPath)
	return nil
}

func (m *userAgentMap) setActiveUid(uid string) {
	logger.Info("active user's uid is", uid)
	m.mu.Lock()
	m.activeUid = uid
	m.mu.Unlock()
}

func (m *userAgentMap) handleNameLost(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range m.uidItemMap {
		for path, agent := range item.agents {
			if agent.ServiceName_() == name {
				logger.Debug("remove agent", name, path)
				delete(item.agents, path)
			}
		}
	}
}

func (m *userAgentMap) addUser(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.uidItemMap[uid]
	if !ok {
		m.uidItemMap[uid] = &sessionAgentMapItem{
			sessions: make(map[dbus.ObjectPath]login1.Session),
			agents:   make(map[dbus.ObjectPath]lastoreAgent.Agent),
		}
	}
}

func (m *userAgentMap) removeUser(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.uidItemMap[uid]
	if !ok {
		return
	}

	for sessionPath, session := range item.sessions {
		session.RemoveAllHandlers()
		delete(item.sessions, sessionPath)
	}
	delete(m.uidItemMap, uid)
}

func (m *userAgentMap) hasUser(uid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.uidItemMap[uid]
	return ok
}

func (m *userAgentMap) addSession(uid string, session login1.Session) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.uidItemMap[uid]
	if !ok {
		return false
	}

	_, ok = item.sessions[session.Path_()]
	if ok {
		return false
	}
	item.sessions[session.Path_()] = session
	return true
}

func (m *userAgentMap) removeSession(sessionPath dbus.ObjectPath) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range m.uidItemMap {
		for sPath, session := range item.sessions {
			if sPath == sessionPath {
				if session != nil {
					session.RemoveAllHandlers()
				}
				delete(item.sessions, sPath)
			}
		}
	}
}

func (m *userAgentMap) addLang(uid, lang string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.uidItemMap[uid]
	if ok {
		item.lang = lang
	} else {
		m.uidItemMap[uid] = &sessionAgentMapItem{lang: lang}
	}

}

func (m *userAgentMap) getActiveLastoreAgentLang() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeUid == "" {
		return ""
	}

	item := m.uidItemMap[m.activeUid]
	if item == nil {
		return ""
	}
	return item.lang
}

const lastoreAgentPath = "/com/deepin/lastore/agent"

func (m *userAgentMap) getActiveLastoreAgent() lastoreAgent.Agent {
	return m.getActiveAgent(lastoreAgentPath)
}

func (m *userAgentMap) getActiveAgent(path dbus.ObjectPath) lastoreAgent.Agent {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeUid == "" {
		return nil
	}

	item := m.uidItemMap[m.activeUid]
	if item == nil {
		return nil
	}
	return item.agents[path]
}

type sessionAgentMapInfo struct {
	Sessions []dbus.ObjectPath          // key 是 session 的路径
	Agents   map[dbus.ObjectPath]string // key 是 agent 的路径, value 是agent 的serviceName(即sender)
	Lang     string
}

// userAgentInfoMap 用来持久化数据,数据来源是userAgentMap
type userAgentInfoMap struct {
	ActiveUid  string
	UidInfoMap map[string]*sessionAgentMapInfo // key 是 uid
}

// 将userAgentMap数据转换为json字符串，供lastore闲时退出时保存
func (m *userAgentMap) getAgentsInfo() *userAgentInfoMap {
	infoMap := &userAgentInfoMap{
		ActiveUid:  m.activeUid,
		UidInfoMap: make(map[string]*sessionAgentMapInfo),
	}
	for uid, item := range m.uidItemMap {
		var sessions []dbus.ObjectPath
		agentsMap := make(map[dbus.ObjectPath]string)
		for sessionPath := range item.sessions {
			sessions = append(sessions, sessionPath)
		}
		for agentPath, agent := range item.agents {
			agentsMap[agentPath] = agent.ServiceName_()
		}
		infoMap.UidInfoMap[uid] = &sessionAgentMapInfo{
			Sessions: sessions,
			Agents:   agentsMap,
			Lang:     item.lang,
		}
	}
	return infoMap
}

// 将agent数据序列化成JSON格式写入recordFilePath中
func (m *userAgentMap) saveRecordContent(recordFilePath string) {
	err := utils.WriteData(recordFilePath, m.getAgentsInfo())
	if err != nil {
		logger.Warning(err)
	}
}

// 根据配置文件，恢复之前注册的Agents数据
func recoverLastoreAgents(recordFilePath string, service *dbusutil.Service) *userAgentMap {
	var infoMap userAgentInfoMap
	var agentMap userAgentMap
	err := decodeJson(recordFilePath, &infoMap)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	logger.Info("record agent info:", infoMap)
	login1Obj := login1.NewManager(service.Conn())
	sessionInfos, err := login1Obj.ListSessions(0)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	sessionList := strv.Strv{}
	for _, session := range sessionInfos {
		if fmt.Sprintf("%v", session.UID) == infoMap.ActiveUid {
			sessionList = append(sessionList, string(session.Path))
		}
	}
	dbusObj := dbus2.NewDBus(service.Conn())
	agentMap.activeUid = infoMap.ActiveUid
	agentMap.uidItemMap = make(map[string]*sessionAgentMapItem, 1)
	for uid, uidInfo := range infoMap.UidInfoMap {
		var item sessionAgentMapItem
		item.sessions = make(map[dbus.ObjectPath]login1.Session)
		item.agents = make(map[dbus.ObjectPath]lastoreAgent.Agent)
		for _, sessionPath := range uidInfo.Sessions {
			// 校验sessionPath是否还有效
			if !sessionList.Contains(string(sessionPath)) {
				logger.Warningf("record session path:%s is invalid", sessionPath)
				continue
			}
			session, err := login1.NewSession(service.Conn(), sessionPath)
			if err != nil {
				logger.Warning(err)
				continue
			}
			item.sessions[sessionPath] = session
		}
		for agentPath, agentSender := range uidInfo.Agents {
			// 校验agentSender是否还有效
			hasOwner, err := dbusObj.NameHasOwner(0, agentSender)
			if err != nil || !hasOwner {
				logger.Warningf("record agent name:%s is invalid", agentSender)
				continue
			}
			agent, err := lastoreAgent.NewAgent(service.Conn(), agentSender, agentPath)
			if err != nil {
				logger.Warning(err)
				continue
			}
			item.agents[agentPath] = agent
		}
		item.lang = uidInfo.Lang
		agentMap.uidItemMap[uid] = &item
	}
	gettext.SetLocale(gettext.LcAll, agentMap.getActiveLastoreAgentLang())
	return &agentMap
}

func decodeJson(fpath string, d interface{}) error {
	content, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, &d)
}
