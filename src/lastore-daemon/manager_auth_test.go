// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/stretchr/testify/assert"
)

func TestInitTrustedCallerUIDs(t *testing.T) {
	oldLookup := lookupUserByName
	lookupUserByName = func(name string) (uint32, error) {
		if name == lightdmUserName {
			return 620, nil
		}
		return 0, errors.New("unexpected user")
	}
	defer func() {
		lookupUserByName = oldLookup
	}()

	uids := initTrustedCallerUIDs()
	_, ok := uids[620]
	assert.True(t, ok)
}

func TestIsTrustedSender(t *testing.T) {
	m := &Manager{
		trustedCallerUIDs:    map[uint32]struct{}{620: {}},
		allowCallServiceList: strv.Strv{":1.20"},
	}

	assert.True(t, m.isTrustedSender(0, ":1.1"))
	assert.True(t, m.isTrustedSender(620, ":1.2"))
	assert.True(t, m.isTrustedSender(1000, ":1.20"))
	assert.False(t, m.isTrustedSender(1000, ":1.21"))
}

func TestSetAllowCallerPersistsRuntimeState(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), ":1.42").Return(":1.42", nil)
	sysBus.MockInterfaceDbusIfc.On("GetId", dbus.Flags(0)).Return("bus-id-1", nil)

	m := &Manager{
		sysDBusDaemon: sysBus,
	}

	busErr := m.SetAllowCaller(":1.42")
	assert.Nil(t, busErr)
	assert.Equal(t, strv.Strv{":1.42"}, m.allowCallServiceList)

	kf := keyfile.NewKeyFile()
	assert.NoError(t, kf.LoadFromFile(allowCallerStateFile))

	busID, err := kf.GetString("AuthState", "BusId")
	assert.NoError(t, err)
	assert.Equal(t, "bus-id-1", busID)

	callers, err := kf.GetStringList("AuthState", callerKey)
	assert.NoError(t, err)
	assert.Equal(t, []string{":1.42"}, callers)

	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}

func TestSetAllowCallerRejectsNameWithoutOwner(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), ":1.404").Return("", errors.New("name has no owner"))

	m := &Manager{
		sysDBusDaemon: sysBus,
	}

	busErr := m.SetAllowCaller(":1.404")
	assert.NotNil(t, busErr)
	assert.Empty(t, m.allowCallServiceList)

	_, err := os.Stat(allowCallerStateFile)
	assert.True(t, os.IsNotExist(err))

	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}

func TestLoadAllowCallerFiltersStaleRuntimeState(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	kf := keyfile.NewKeyFile()
	kf.SetString("AuthState", "BusId", "bus-id-1")
	kf.SetStringList("AuthState", callerKey, []string{":1.8", ":1.9"})
	assert.NoError(t, kf.SaveToFile(allowCallerStateFile))

	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetId", dbus.Flags(0)).Return("bus-id-1", nil)
	sysBus.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), ":1.8").Return(":1.8", nil)
	sysBus.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), ":1.9").Return("", errors.New("name has no owner"))

	m := &Manager{
		sysDBusDaemon: sysBus,
	}

	m.loadAllowCaller()
	assert.Equal(t, strv.Strv{":1.8"}, m.allowCallServiceList)

	kf = keyfile.NewKeyFile()
	assert.NoError(t, kf.LoadFromFile(allowCallerStateFile))
	callers, err := kf.GetStringList("AuthState", callerKey)
	assert.NoError(t, err)
	assert.Equal(t, []string{":1.8"}, callers)

	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}

func TestLoadAllowCallerDropsRuntimeStateAfterBusRestart(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	kf := keyfile.NewKeyFile()
	kf.SetString("AuthState", "BusId", "bus-id-old")
	kf.SetStringList("AuthState", callerKey, []string{":1.10"})
	assert.NoError(t, kf.SaveToFile(allowCallerStateFile))

	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetId", dbus.Flags(0)).Return("bus-id-new", nil)

	m := &Manager{
		sysDBusDaemon: sysBus,
	}

	m.loadAllowCaller()
	assert.Empty(t, m.allowCallServiceList)
	_, err := os.Stat(allowCallerStateFile)
	assert.True(t, os.IsNotExist(err))

	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}

func TestRemoveAllowCallerPersistsRuntimeState(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetId", dbus.Flags(0)).Return("bus-id-1", nil)

	m := &Manager{
		sysDBusDaemon:        sysBus,
		allowCallServiceList: strv.Strv{":1.12", ":1.13"},
	}

	kf := keyfile.NewKeyFile()
	kf.SetString("AuthState", "BusId", "bus-id-1")
	kf.SetStringList("AuthState", callerKey, []string{":1.12", ":1.13"})
	assert.NoError(t, os.MkdirAll(filepath.Dir(allowCallerStateFile), 0755))
	assert.NoError(t, kf.SaveToFile(allowCallerStateFile))

	m.removeAllowCaller(":1.12")
	assert.Equal(t, strv.Strv{":1.13"}, m.allowCallServiceList)

	kf = keyfile.NewKeyFile()
	assert.NoError(t, kf.LoadFromFile(allowCallerStateFile))
	callers, err := kf.GetStringList("AuthState", callerKey)
	assert.NoError(t, err)
	assert.Equal(t, []string{":1.13"}, callers)

	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}
