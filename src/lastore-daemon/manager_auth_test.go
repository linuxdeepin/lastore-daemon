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
	"github.com/stretchr/testify/require"
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

func TestInitTrustedCallerUIDs_Error(t *testing.T) {
	oldLookup := lookupUserByName
	lookupUserByName = func(name string) (uint32, error) {
		return 0, errors.New("no such user")
	}
	defer func() {
		lookupUserByName = oldLookup
	}()

	uids := initTrustedCallerUIDs()
	assert.Empty(t, uids)
}

func TestReadAllowCallerState_NotExist(t *testing.T) {
	state, err := readAllowCallerState(filepath.Join(t.TempDir(), "no-such-file.ini"))
	assert.NoError(t, err)
	assert.Nil(t, state)
}

func TestReadAllowCallerState_Malformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allow-callers.ini")
	assert.NoError(t, os.WriteFile(path, []byte("this is not a keyfile"), 0644))
	state, err := readAllowCallerState(path)
	assert.Error(t, err)
	assert.Nil(t, state)
}

func TestReadAllowCallerState_Valid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allow-callers.ini")
	kf := keyfile.NewKeyFile()
	kf.SetString(allowCallerStateSection, allowCallerBusIDKey, "bus-id-1")
	kf.SetStringList(allowCallerStateSection, callerKey, []string{":1.1", "", ":1.1", ":1.2"})
	assert.NoError(t, kf.SaveToFile(path))

	state, err := readAllowCallerState(path)
	assert.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, "bus-id-1", state.BusID)
	assert.Equal(t, strv.Strv{":1.1", ":1.2"}, state.Callers)
}

func TestReadAllowCallerState_MissingCallerList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allow-callers.ini")
	kf := keyfile.NewKeyFile()
	kf.SetString(allowCallerStateSection, allowCallerBusIDKey, "bus-id-1")
	assert.NoError(t, kf.SaveToFile(path))

	state, err := readAllowCallerState(path)
	assert.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, "bus-id-1", state.BusID)
	assert.Empty(t, state.Callers)
}

func TestPersistAllowCallerState_EmptyRemovesFile(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	assert.NoError(t, os.WriteFile(allowCallerStateFile, []byte("x"), 0644))
	m := &Manager{}
	assert.NoError(t, m.persistAllowCallerState(strv.Strv{}))
	_, err := os.Stat(allowCallerStateFile)
	assert.True(t, os.IsNotExist(err))
}

func TestPersistAllowCallerState_NilBusError(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	m := &Manager{}
	err := m.persistAllowCallerState(strv.Strv{":1.1"})
	assert.Error(t, err)
}

func TestAddAllowCaller_NotUniqueName(t *testing.T) {
	m := &Manager{}
	err := m.addAllowCaller("abc")
	assert.Error(t, err)
}

func TestAddAllowCaller_NilBus(t *testing.T) {
	m := &Manager{}
	err := m.addAllowCaller(":1.1")
	assert.Error(t, err)
}

func TestAddAllowCaller_GetNameOwnerError(t *testing.T) {
	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), ":1.1").Return("", errors.New("no such name"))
	m := &Manager{sysDBusDaemon: sysBus}

	err := m.addAllowCaller(":1.1")
	assert.Error(t, err)
	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}

func TestAddAllowCaller_OwnerMismatch(t *testing.T) {
	sysBus := &ofdbus.MockDBus{}
	sysBus.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), ":1.1").Return(":1.2", nil)
	m := &Manager{sysDBusDaemon: sysBus}

	err := m.addAllowCaller(":1.1")
	assert.Error(t, err)
	sysBus.MockInterfaceDbusIfc.AssertExpectations(t)
}

func TestLoadAllowCaller_ReadError(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	assert.NoError(t, os.WriteFile(allowCallerStateFile, []byte("not a keyfile"), 0644))
	m := &Manager{sysDBusDaemon: &ofdbus.MockDBus{}}
	m.loadAllowCaller()
	assert.Empty(t, m.allowCallServiceList)
}

func TestLoadAllowCaller_NilBusError(t *testing.T) {
	oldPath := allowCallerStateFile
	allowCallerStateFile = filepath.Join(t.TempDir(), "allow-callers.ini")
	defer func() {
		allowCallerStateFile = oldPath
	}()

	kf := keyfile.NewKeyFile()
	kf.SetString(allowCallerStateSection, allowCallerBusIDKey, "bus-id-1")
	kf.SetStringList(allowCallerStateSection, callerKey, []string{":1.1"})
	assert.NoError(t, kf.SaveToFile(allowCallerStateFile))

	m := &Manager{}
	m.loadAllowCaller()
	assert.Empty(t, m.allowCallServiceList)
}

func TestRemoveAllowCaller_NotPresent(t *testing.T) {
	m := &Manager{
		allowCallServiceList: strv.Strv{":1.1"},
	}
	m.removeAllowCaller(":1.99")
	assert.Equal(t, strv.Strv{":1.1"}, m.allowCallServiceList)
}
