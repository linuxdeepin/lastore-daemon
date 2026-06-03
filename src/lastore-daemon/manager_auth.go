// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/strv"
)

const (
	allowCallerStateSection = "AuthState"
	allowCallerBusIDKey     = "BusId"
	callerKey               = "AllowCallerList"
	lightdmUserName         = "lightdm"
)

var (
	// 允许调用者名单只保存在运行时目录，避免写入公共可写路径。
	allowCallerStateFile = "/run/lastore/allow-callers.ini"
	lookupUserByName     = func(name string) (uint32, error) {
		u, err := user.Lookup(name)
		if err != nil {
			return 0, err
		}

		uid, err := strconv.ParseUint(u.Uid, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint32(uid), nil
	}
)

type allowCallerState struct {
	BusID   string
	Callers strv.Strv
}

var allowCallerStateMu sync.Mutex

func isAllowCallerUniqueName(name string) bool {
	return strings.HasPrefix(name, ":")
}

func initTrustedCallerUIDs() map[uint32]struct{} {
	trustedUIDs := make(map[uint32]struct{})

	// greeter 这类特殊 uid 调用方不走 allow-caller 注册，直接按 uid 信任。
	uid, err := lookupUserByName(lightdmUserName)
	if err != nil {
		logger.Warning(err)
		return trustedUIDs
	}
	trustedUIDs[uid] = struct{}{}
	return trustedUIDs
}

func (m *Manager) isTrustedSender(uid uint32, sender dbus.Sender) bool {
	if uid == 0 {
		return true
	}
	if _, ok := m.trustedCallerUIDs[uid]; ok {
		return true
	}

	// loader 启动的前端通过 SetAllowCaller 记录到 unique name 白名单里。
	m.PropsMu.RLock()
	ok := m.allowCallServiceList.Contains(string(sender))
	m.PropsMu.RUnlock()
	return ok
}

func (m *Manager) getSystemBusID() (string, error) {
	if m.sysDBusDaemon == nil {
		return "", fmt.Errorf("system bus daemon is nil")
	}
	return m.sysDBusDaemon.GetId(dbus.Flags(0))
}

func readAllowCallerState(path string) (*allowCallerState, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	kf := keyfile.NewKeyFile()
	err = kf.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	busID, err := kf.GetString(allowCallerStateSection, allowCallerBusIDKey)
	if err != nil {
		return nil, err
	}

	callers, err := kf.GetStringList(allowCallerStateSection, callerKey)
	if err != nil {
		if _, ok := err.(keyfile.KeyNotFoundError); !ok {
			return nil, err
		}
	}

	return &allowCallerState{
		BusID:   busID,
		Callers: strv.Strv(callers).FilterEmpty().Uniq(),
	}, nil
}

func (m *Manager) persistAllowCallerState(callers strv.Strv) error {
	allowCallerStateMu.Lock()
	defer allowCallerStateMu.Unlock()

	callers = callers.FilterEmpty().Uniq()
	if len(callers) == 0 {
		err := os.Remove(allowCallerStateFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	busID, err := m.getSystemBusID()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(allowCallerStateFile), 0700)
	if err != nil {
		return err
	}

	kf := keyfile.NewKeyFile()
	kf.SetString(allowCallerStateSection, allowCallerBusIDKey, busID)
	kf.SetStringList(allowCallerStateSection, callerKey, callers)
	return kf.SaveToFile(allowCallerStateFile)
}

func (m *Manager) loadAllowCaller() {
	allowCallerStateMu.Lock()
	state, err := readAllowCallerState(allowCallerStateFile)
	if err != nil {
		allowCallerStateMu.Unlock()
		logger.Warning(err)
		return
	}
	if state == nil {
		allowCallerStateMu.Unlock()
		return
	}

	currentBusID, err := m.getSystemBusID()
	if err != nil {
		allowCallerStateMu.Unlock()
		logger.Warning(err)
		return
	}

	// unique name 只在当前 system bus 生命周期内有效，bus 变了就整体丢弃。
	if state.BusID != currentBusID {
		allowCallerStateMu.Unlock()
		m.PropsMu.Lock()
		m.allowCallServiceList = nil
		m.PropsMu.Unlock()
		err = os.Remove(allowCallerStateFile)
		if err != nil && !os.IsNotExist(err) {
			logger.Warning(err)
		}
		return
	}
	// 拷贝出待验证列表后即释放 allowCallerStateMu，避免在 IPC 期间持有该锁。
	callersToCheck := append(strv.Strv(nil), state.Callers...)
	allowCallerStateMu.Unlock()

	validCallers := make(strv.Strv, 0, len(callersToCheck))
	for _, name := range callersToCheck {
		if !isAllowCallerUniqueName(name) {
			continue
		}
		// 运行时文件里可能残留已断开的连接，启动时按当前 owner 重新过滤一次。
		owner, err := m.sysDBusDaemon.GetNameOwner(dbus.Flags(0), name)
		if err != nil {
			continue
		}
		if owner == name {
			validCallers = append(validCallers, name)
		}
	}

	m.PropsMu.Lock()
	m.allowCallServiceList = validCallers
	m.PropsMu.Unlock()

	if !validCallers.Equal(callersToCheck) {
		allowCallerStateMu.Lock()
		defer allowCallerStateMu.Unlock()

		if len(validCallers) == 0 {
			err = os.Remove(allowCallerStateFile)
			if err != nil && !os.IsNotExist(err) {
				logger.Warning(err)
			}
			return
		}

		kf := keyfile.NewKeyFile()
		kf.SetString(allowCallerStateSection, allowCallerBusIDKey, currentBusID)
		kf.SetStringList(allowCallerStateSection, callerKey, validCallers)
		err = kf.SaveToFile(allowCallerStateFile)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (m *Manager) addAllowCaller(uniqueName string) error {
	if !isAllowCallerUniqueName(uniqueName) {
		return fmt.Errorf("%q is not a dbus unique name", uniqueName)
	}
	if m.sysDBusDaemon == nil {
		return fmt.Errorf("system bus daemon is nil")
	}
	// 受信任 daemon 允许代前端注册，但 unique name 必须是当前真实存在的连接。
	owner, err := m.sysDBusDaemon.GetNameOwner(dbus.Flags(0), uniqueName)
	if err != nil {
		return err
	}
	if owner != uniqueName {
		return fmt.Errorf("%q is not owned by current dbus connection", uniqueName)
	}

	m.PropsMu.Lock()
	oldList := append(strv.Strv(nil), m.allowCallServiceList...)
	newList, _ := m.allowCallServiceList.Add(uniqueName)
	m.allowCallServiceList = newList
	snapshot := append(strv.Strv(nil), m.allowCallServiceList...)
	m.PropsMu.Unlock()

	err = m.persistAllowCallerState(snapshot)
	if err != nil {
		m.PropsMu.Lock()
		m.allowCallServiceList = oldList
		m.PropsMu.Unlock()
		return err
	}
	return nil
}

func (m *Manager) removeAllowCaller(uniqueName string) {
	m.PropsMu.Lock()
	newList, removed := m.allowCallServiceList.Delete(uniqueName)
	if !removed {
		m.PropsMu.Unlock()
		return
	}
	m.allowCallServiceList = newList
	snapshot := append(strv.Strv(nil), m.allowCallServiceList...)
	m.PropsMu.Unlock()

	err := m.persistAllowCallerState(snapshot)
	if err != nil {
		logger.Warning(err)
	}
}
