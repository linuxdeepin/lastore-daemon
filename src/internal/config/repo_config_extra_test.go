// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestRepoTypeString(t *testing.T) {
	tests := []struct {
		name string
		r    RepoType
		want string
	}{
		{"OSDefaultRepo", OSDefaultRepo, "OSDefaultRepo"},
		{"OemDefaultRepo", OemDefaultRepo, "OemDefaultRepo"},
		{"CustomRepo", CustomRepo, "CustomRepo"},
		{"Unknown", RepoType("UNKNOWN"), ""},
		{"Empty", RepoType(""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.r.String())
		})
	}
}

func TestRepoTypeIsValid(t *testing.T) {
	assert.True(t, OSDefaultRepo.IsValid())
	assert.True(t, OemDefaultRepo.IsValid())
	assert.True(t, CustomRepo.IsValid())
	assert.False(t, RepoType("UNKNOWN").IsValid())
	assert.False(t, RepoType("").IsValid())
}

func TestGetOemRepoInfoNonExistDir(t *testing.T) {
	sys, sec := GetOemRepoInfo("/nonexistent/path/for/test")
	assert.Nil(t, sys)
	assert.Nil(t, sec)
}

func TestGetOemRepoInfoEmptyDir(t *testing.T) {
	dir := t.TempDir()
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.Equal(t, system.SystemUpdate, sys.UpdateType)
	assert.Equal(t, system.SecurityUpdate, sec.UpdateType)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoWithFiles(t *testing.T) {
	dir := t.TempDir()

	systemRepo := OemRepoConfig{
		UpdateType:     system.SystemUpdate,
		RepoShowNameZh: "系统仓库",
		RepoShowNameEn: "System Repo",
		RepoUrl:        []string{"http://example.com/system"},
	}
	data, err := json.Marshal(&systemRepo)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "system.json"), data, 0644))

	securityRepo := OemRepoConfig{
		UpdateType:     system.SecurityUpdate,
		RepoShowNameZh: "安全仓库",
		RepoShowNameEn: "Security Repo",
		RepoUrl:        []string{"http://example.com/security"},
	}
	data, err = json.Marshal(&securityRepo)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "security.json"), data, 0644))

	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.Equal(t, "系统仓库", sys.RepoShowNameZh)
	assert.Equal(t, "System Repo", sys.RepoShowNameEn)
	assert.Equal(t, []string{"http://example.com/system"}, sys.RepoUrl)
	assert.True(t, sys.hasSet)

	assert.Equal(t, "安全仓库", sec.RepoShowNameZh)
	assert.Equal(t, "Security Repo", sec.RepoShowNameEn)
	assert.Equal(t, []string{"http://example.com/security"}, sec.RepoUrl)
	assert.True(t, sec.hasSet)
}

func TestGetOemRepoInfoInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid json}"), 0644))
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoInvalidUpdateType(t *testing.T) {
	dir := t.TempDir()
	badRepo := OemRepoConfig{
		UpdateType: system.UpdateType(999),
		RepoUrl:    []string{"http://example.com/bad"},
	}
	data, err := json.Marshal(&badRepo)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad_type.json"), data, 0644))
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoSkipNonJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
	sys, sec := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	require.NotNil(t, sec)
	assert.False(t, sys.hasSet)
	assert.False(t, sec.hasSet)
}

func TestGetOemRepoInfoMultipleFilesSameType(t *testing.T) {
	dir := t.TempDir()

	repo1 := OemRepoConfig{
		UpdateType:     system.SystemUpdate,
		RepoShowNameZh: "第一个",
		RepoUrl:        []string{"http://first.com"},
	}
	data, _ := json.Marshal(&repo1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json"), data, 0644))

	repo2 := OemRepoConfig{
		UpdateType:     system.SystemUpdate,
		RepoShowNameZh: "第二个",
		RepoUrl:        []string{"http://second.com"},
	}
	data, _ = json.Marshal(&repo2)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.json"), data, 0644))

	sys, _ := GetOemRepoInfo(dir)
	require.NotNil(t, sys)
	assert.True(t, sys.hasSet)
	assert.Equal(t, "第二个", sys.RepoShowNameZh)
	assert.Equal(t, []string{"http://second.com"}, sys.RepoUrl)
}

func TestGetPlatformStatusDisable(t *testing.T) {
	c := &Config{PlatformDisabled: DisabledVersion | DisabledUpdateLog}
	assert.True(t, c.GetPlatformStatusDisable(DisabledVersion))
	assert.True(t, c.GetPlatformStatusDisable(DisabledUpdateLog))
	assert.False(t, c.GetPlatformStatusDisable(DisabledTargetPkgLists))

	c2 := &Config{PlatformDisabled: 0}
	assert.False(t, c2.GetPlatformStatusDisable(DisabledVersion))
}

func TestConfigUseIncrementalUpdate(t *testing.T) {
	c := &Config{IncrementalUpdate: true}
	assert.True(t, c.UseIncrementalUpdate())
	c2 := &Config{IncrementalUpdate: false}
	assert.False(t, c2.UseIncrementalUpdate())
}

func TestConfigResetDSettingsNoManager(t *testing.T) {
	c := &Config{}
	err := c.ResetDSettings("some-key")
	assert.NoError(t, err)
}

func TestRecoveryAndApplyOemFlagInvalidType(t *testing.T) {
	c := &Config{}
	err := c.recoveryAndApplyOemFlag(system.UpdateType(1 << 10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid oem update type")
}

func TestReloadOemRepoConfigNoDir(t *testing.T) {
	c := &Config{}
	// OemRepoDirPath is a const pointing at a root-owned path that does not
	// exist in the test environment; GetOemRepoInfo therefore returns nil,nil.
	require.NotPanics(t, func() {
		c.reloadOemRepoConfig()
	})
	assert.Empty(t, c.SystemOemSourceConfig.RepoShowNameZh)
	assert.Empty(t, c.SecurityOemSourceConfig.RepoShowNameZh)
}

func TestReloadSourcesDirManagerError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySystemRepoType).
		Return(dbus.Variant{}, errors.New("not available"))
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySecurityRepoType).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.ReloadSourcesDir()
	})
	// Value 返回错误,仓库类型保持零值,switch 不落入任何需要写 root 路径的分支。
	assert.Equal(t, RepoType(""), c.SystemRepoType)
	assert.Equal(t, RepoType(""), c.SecurityRepoType)
}

func TestConfigResetDSettingsWithManager(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Reset", dbus.Flags(0), "some-key").Return(nil)

	c := &Config{dsLastoreManager: mgr}
	err := c.ResetDSettings("some-key")
	require.NoError(t, err)
	mgr.MockInterfaceManager.AssertCalled(t, "Reset", dbus.Flags(0), "some-key")
}

func TestConfigResetDSettingsManagerError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Reset", dbus.Flags(0), "some-key").Return(errors.New("reset failed"))

	c := &Config{dsLastoreManager: mgr}
	err := c.ResetDSettings("some-key")
	require.Error(t, err)
	assert.EqualError(t, err, "reset failed")
}

func TestRecoveryAndApplyOemFlagSystemUpdate(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevMoveFile := moveFileFn
	prevRemoveAll := removeAllFn
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		moveFileFn = prevMoveFile
		removeAllFn = prevRemoveAll
	})

	systemFlagPath := filepath.Join(OemRepoDirPath, UseOemSystemRepoFlagFile)

	var movedFrom, movedTo string
	var removed []string
	isFileExistFn = func(path string) bool {
		return path == systemFlagPath || path == BackupOemSystemSourceListFilePath
	}
	moveFileFn = func(from, to string) error {
		movedFrom, movedTo = from, to
		return nil
	}
	removeAllFn = func(path string) error {
		removed = append(removed, path)
		return nil
	}

	cfg := newTestConfig(t)
	err := cfg.recoveryAndApplyOemFlag(system.SystemUpdate)
	require.NoError(t, err)
	assert.Equal(t, OemDefaultRepo, cfg.SystemRepoType)
	assert.Equal(t, BackupOemSystemSourceListFilePath, movedFrom)
	assert.Equal(t, system.OriginSourceFile, movedTo)
	assert.Contains(t, removed, systemFlagPath)
}

func TestRecoveryAndApplyOemFlagBackupMissing(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevMoveFile := moveFileFn
	prevRemoveAll := removeAllFn
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		moveFileFn = prevMoveFile
		removeAllFn = prevRemoveAll
	})

	systemFlagPath := filepath.Join(OemRepoDirPath, UseOemSystemRepoFlagFile)

	moveCalled := false
	isFileExistFn = func(path string) bool {
		return path == systemFlagPath
	}
	moveFileFn = func(from, to string) error {
		moveCalled = true
		return nil
	}
	removeAllFn = func(path string) error { return nil }

	cfg := newTestConfig(t)
	err := cfg.recoveryAndApplyOemFlag(system.SystemUpdate)
	require.NoError(t, err)
	assert.False(t, moveCalled)
	assert.Equal(t, OemDefaultRepo, cfg.SystemRepoType)
}

func TestRecoveryAndApplyOemFlagSecurityUpdate(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevMoveFile := moveFileFn
	prevRemoveAll := removeAllFn
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		moveFileFn = prevMoveFile
		removeAllFn = prevRemoveAll
	})

	securityFlagPath := filepath.Join(OemRepoDirPath, UseOemSecurityRepoFlagFile)

	var movedFrom, movedTo string
	isFileExistFn = func(path string) bool {
		return path == securityFlagPath || path == BackupOemSecuritySourceListFilePath
	}
	moveFileFn = func(from, to string) error {
		movedFrom, movedTo = from, to
		return nil
	}
	removeAllFn = func(path string) error { return nil }

	cfg := newTestConfig(t)
	err := cfg.recoveryAndApplyOemFlag(system.SecurityUpdate)
	require.NoError(t, err)
	assert.Equal(t, OemDefaultRepo, cfg.SecurityRepoType)
	assert.Equal(t, BackupOemSecuritySourceListFilePath, movedFrom)
	assert.Equal(t, system.SecuritySourceFile, movedTo)
}

func TestRecoveryAndApplyOemFlagApplyError(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevMoveFile := moveFileFn
	prevRemoveAll := removeAllFn
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		moveFileFn = prevMoveFile
		removeAllFn = prevRemoveAll
	})

	isFileExistFn = func(path string) bool { return true }
	moveFileFn = func(from, to string) error { return nil }
	removeAllFn = func(path string) error {
		t.Fatal("removeAll must not be called when applyFunc fails")
		return nil
	}

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("SetValue", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("apply failed"))

	cfg := &Config{dsLastoreManager: mgr}
	err := cfg.recoveryAndApplyOemFlag(system.SystemUpdate)
	require.Error(t, err)
	assert.EqualError(t, err, "apply failed")
}

func TestReloadOemRepoConfigWithConfigs(t *testing.T) {
	prevGetOemRepoInfo := getOemRepoInfoFn
	t.Cleanup(func() { getOemRepoInfoFn = prevGetOemRepoInfo })

	getOemRepoInfoFn = func(dir string) (*OemRepoConfig, *OemRepoConfig) {
		assert.Equal(t, OemRepoDirPath, dir)
		return &OemRepoConfig{RepoShowNameZh: "系统仓库", RepoUrl: []string{"http://sys.example"}},
			&OemRepoConfig{RepoShowNameZh: "安全仓库", RepoUrl: []string{"http://sec.example"}}
	}

	c := &Config{}
	c.reloadOemRepoConfig()
	assert.Equal(t, "系统仓库", c.SystemOemSourceConfig.RepoShowNameZh)
	assert.Equal(t, []string{"http://sys.example"}, c.SystemOemSourceConfig.RepoUrl)
	assert.Equal(t, "安全仓库", c.SecurityOemSourceConfig.RepoShowNameZh)
	assert.Equal(t, []string{"http://sec.example"}, c.SecurityOemSourceConfig.RepoUrl)
}

func TestReloadSourcesDirOSDefault(t *testing.T) {
	prevUpdateSystem := updateSystemDefaultSourceDirFn
	prevUpdateSecurity := updateSecurityDefaultSourceDirFn
	prevUpdateUseUrl := updateSourceDirUseUrlFn
	prevGetOemRepoInfo := getOemRepoInfoFn
	t.Cleanup(func() {
		updateSystemDefaultSourceDirFn = prevUpdateSystem
		updateSecurityDefaultSourceDirFn = prevUpdateSecurity
		updateSourceDirUseUrlFn = prevUpdateUseUrl
		getOemRepoInfoFn = prevGetOemRepoInfo
	})

	getOemRepoInfoFn = func(dir string) (*OemRepoConfig, *OemRepoConfig) { return nil, nil }

	var systemList, securityList []string
	updateSystemDefaultSourceDirFn = func(sourceList []string) error {
		systemList = sourceList
		return nil
	}
	updateSecurityDefaultSourceDirFn = func(sourceList []string) error {
		securityList = sourceList
		return nil
	}
	updateSourceDirUseUrlFn = func(_ system.UpdateType, _ []string, _, _ string) error {
		t.Fatal("updateSourceDirUseUrl must not be called for OSDefaultRepo")
		return nil
	}

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySystemRepoType).
		Return(dbus.MakeVariant("UOS_DEFAULT"), nil)
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySecurityRepoType).
		Return(dbus.MakeVariant("UOS_DEFAULT"), nil)

	c := &Config{dsLastoreManager: mgr, SystemSourceList: []string{"sys.list"}, SecuritySourceList: []string{"sec.list"}}
	c.ReloadSourcesDir()
	assert.Equal(t, OSDefaultRepo, c.SystemRepoType)
	assert.Equal(t, OSDefaultRepo, c.SecurityRepoType)
	assert.Equal(t, []string{"sys.list"}, systemList)
	assert.Equal(t, []string{"sec.list"}, securityList)
}

func TestReloadSourcesDirOemDefault(t *testing.T) {
	prevUpdateSystem := updateSystemDefaultSourceDirFn
	prevUpdateSecurity := updateSecurityDefaultSourceDirFn
	prevUpdateUseUrl := updateSourceDirUseUrlFn
	prevGetOemRepoInfo := getOemRepoInfoFn
	t.Cleanup(func() {
		updateSystemDefaultSourceDirFn = prevUpdateSystem
		updateSecurityDefaultSourceDirFn = prevUpdateSecurity
		updateSourceDirUseUrlFn = prevUpdateUseUrl
		getOemRepoInfoFn = prevGetOemRepoInfo
	})

	getOemRepoInfoFn = func(dir string) (*OemRepoConfig, *OemRepoConfig) {
		return &OemRepoConfig{RepoUrl: []string{"http://sys-oem"}},
			&OemRepoConfig{RepoUrl: []string{"http://sec-oem"}}
	}
	updateSystemDefaultSourceDirFn = func(_ []string) error {
		t.Fatal("updateSystemDefaultSourceDir must not be called for OemDefaultRepo")
		return nil
	}
	updateSecurityDefaultSourceDirFn = func(_ []string) error {
		t.Fatal("updateSecurityDefaultSourceDir must not be called for OemDefaultRepo")
		return nil
	}

	type useUrlCall struct {
		typ      system.UpdateType
		urls     []string
		fileName string
	}
	var calls []useUrlCall
	updateSourceDirUseUrlFn = func(typ system.UpdateType, urls []string, fileName, _ string) error {
		calls = append(calls, useUrlCall{typ, urls, fileName})
		return nil
	}

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySystemRepoType).
		Return(dbus.MakeVariant("OEM_DEFAULT"), nil)
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySecurityRepoType).
		Return(dbus.MakeVariant("OEM_DEFAULT"), nil)

	c := &Config{dsLastoreManager: mgr}
	c.ReloadSourcesDir()
	assert.Equal(t, OemDefaultRepo, c.SystemRepoType)
	assert.Equal(t, OemDefaultRepo, c.SecurityRepoType)
	require.Len(t, calls, 2)
	assert.Equal(t, useUrlCall{system.SystemUpdate, []string{"http://sys-oem"}, "system-oem-sources.list"}, calls[0])
	assert.Equal(t, useUrlCall{system.SecurityUpdate, []string{"http://sec-oem"}, "security-oem-sources.list"}, calls[1])
}

func TestReloadSourcesDirCustom(t *testing.T) {
	prevUpdateSystem := updateSystemDefaultSourceDirFn
	prevUpdateSecurity := updateSecurityDefaultSourceDirFn
	prevUpdateUseUrl := updateSourceDirUseUrlFn
	prevGetOemRepoInfo := getOemRepoInfoFn
	t.Cleanup(func() {
		updateSystemDefaultSourceDirFn = prevUpdateSystem
		updateSecurityDefaultSourceDirFn = prevUpdateSecurity
		updateSourceDirUseUrlFn = prevUpdateUseUrl
		getOemRepoInfoFn = prevGetOemRepoInfo
	})

	getOemRepoInfoFn = func(dir string) (*OemRepoConfig, *OemRepoConfig) { return nil, nil }
	updateSystemDefaultSourceDirFn = func(_ []string) error {
		t.Fatal("updateSystemDefaultSourceDir must not be called for CustomRepo")
		return nil
	}
	updateSecurityDefaultSourceDirFn = func(_ []string) error {
		t.Fatal("updateSecurityDefaultSourceDir must not be called for CustomRepo")
		return nil
	}

	type useUrlCall struct {
		typ      system.UpdateType
		urls     []string
		fileName string
	}
	var calls []useUrlCall
	updateSourceDirUseUrlFn = func(typ system.UpdateType, urls []string, fileName, _ string) error {
		calls = append(calls, useUrlCall{typ, urls, fileName})
		return nil
	}

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySystemRepoType).
		Return(dbus.MakeVariant("CUSTOM"), nil)
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySecurityRepoType).
		Return(dbus.MakeVariant("CUSTOM"), nil)

	c := &Config{
		dsLastoreManager:     mgr,
		SystemCustomSource:   []string{"http://sys-custom"},
		SecurityCustomSource: []string{"http://sec-custom"},
	}
	c.ReloadSourcesDir()
	assert.Equal(t, CustomRepo, c.SystemRepoType)
	assert.Equal(t, CustomRepo, c.SecurityRepoType)
	require.Len(t, calls, 2)
	assert.Equal(t, useUrlCall{system.SystemUpdate, []string{"http://sys-custom"}, "system-custom-sources.list"}, calls[0])
	assert.Equal(t, useUrlCall{system.SecurityUpdate, []string{"http://sec-custom"}, "security-custom-sources.list"}, calls[1])
}

func TestHandleValueChangedBoolFields(t *testing.T) {
	tests := []struct {
		name string
		key  string
		get  func(c *Config) bool
	}{
		{"intranet", DSettingsKeyIntranetUpdate, func(c *Config) bool { return c.IntranetUpdate }},
		{"platformUpdate", DSettingsKeyPlatformUpdate, func(c *Config) bool { return c.PlatformUpdate }},
		{"getHardwareIdByHelper", DSettingsKeyGetHardwareIdByHelper, func(c *Config) bool { return c.GetHardwareIdByHelper }},
		{"includeDiskInfo", DSettingsKeyIncludeDiskInfo, func(c *Config) bool { return c.IncludeDiskInfo }},
		{"incremental", DSettingsKeyIncrementalUpdate, func(c *Config) bool { return c.IncrementalUpdate }},
		{"autoDownload", DSettingsKeyAutoDownloadUpdates, func(c *Config) bool { return c.AutoDownloadUpdates }},
		{"upgradeDelivery", DSettingsKeyUpgradeDeliveryEnabled, func(c *Config) bool { return c.UpgradeDeliveryEnabled }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &ConfigManager.MockManager{}
			mgr.MockInterfaceManager.On("Value", dbus.Flags(0), tt.key).Return(dbus.MakeVariant(true), nil)

			c := &Config{dsLastoreManager: mgr}
			c.handleValueChanged(tt.key)
			assert.True(t, tt.get(c))
		})
	}
}

func TestHandleValueChangedPlatformUrl(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyPlatformUrl).
		Return(dbus.MakeVariant("https://example.com"), nil)

	c := &Config{dsLastoreManager: mgr}
	c.handleValueChanged(DSettingsKeyPlatformUrl)
	assert.Equal(t, "https://example.com", c.PlatformUrl)
}

func TestHandleValueChangedValueError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyPlatformUrl).
		Return(dbus.Variant{}, errors.New("value failed"))

	c := &Config{dsLastoreManager: mgr, PlatformUrl: "unchanged"}
	c.handleValueChanged(DSettingsKeyPlatformUrl)
	assert.Equal(t, "unchanged", c.PlatformUrl)
}

func TestHandleValueChangedFiresCallback(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyIntranetUpdate).
		Return(dbus.MakeVariant(true), nil)

	got := make(chan [2]bool, 1)
	c := &Config{dsLastoreManager: mgr}
	c.dsettingsChangedCbMap = map[string]func(interface{}, interface{}){
		DSettingsKeyIntranetUpdate: func(old, new interface{}) {
			got <- [2]bool{old.(bool), new.(bool)}
		},
	}
	c.handleValueChanged(DSettingsKeyIntranetUpdate)

	select {
	case pair := <-got:
		assert.False(t, pair[0])
		assert.True(t, pair[1])
	case <-time.After(2 * time.Second):
		t.Fatal("value-changed callback was not invoked")
	}
}

func TestHandleValueChangedLastoreDaemonStatus(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyLastoreDaemonStatus).
		Return(dbus.MakeVariant(int64(DisableUpdate)), nil)

	c := &Config{dsLastoreManager: mgr}
	c.handleValueChanged(DSettingsKeyLastoreDaemonStatus)
	assert.Equal(t, LastoreDaemonStatus(DisableUpdate), c.lastoreDaemonStatus)
}

func TestUpdateLastoreDaemonStatusFromDSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mgr := &ConfigManager.MockManager{}
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyLastoreDaemonStatus).
			Return(dbus.MakeVariant(int64(DisableUpdate|CanUpgrade)), nil)

		c := &Config{dsLastoreManager: mgr}
		c.updateLastoreDaemonStatus()
		assert.Equal(t, LastoreDaemonStatus(DisableUpdate|CanUpgrade), c.lastoreDaemonStatus)
	})
	t.Run("error", func(t *testing.T) {
		mgr := &ConfigManager.MockManager{}
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyLastoreDaemonStatus).
			Return(dbus.Variant{}, errors.New("not available"))

		c := &Config{dsLastoreManager: mgr, lastoreDaemonStatus: CanUpgrade}
		c.updateLastoreDaemonStatus()
		assert.Equal(t, LastoreDaemonStatus(CanUpgrade), c.lastoreDaemonStatus)
	})
}
