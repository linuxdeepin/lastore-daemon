// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// setupAllValueSuccess sets up mock Value returns for every key read by loadFromDSettings.
func setupAllValueSuccess(mgr *ConfigManager.MockManager) {
	boolKeys := []string{
		dSettingsKeyUseDSettings, dSettingsKeyAutoCheckUpdates, dSettingsKeyDisableUpdateMetadata,
		DSettingsKeyAutoDownloadUpdates, dSettingsKeyAutoClean, DSettingsKeyIncrementalUpdate,
		DSettingsKeyIntranetUpdate, dSettingsKeyUpdateNotify, dSettingsKeyAutoInstallUpdates,
		DSettingsKeyPlatformUpdate, DSettingsKeyIncludeDiskInfo, dSettingsKeyEnableVersionCheck,
		dSettingsKeyUpdateProcessUpload, dSettingsKeyEnableCoreList,
		DSettingsKeyUpgradeDeliveryEnabled, DSettingsKeyGetHardwareIdByHelper,
	}
	for _, key := range boolKeys {
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), key).Return(dbus.MakeVariant(true), nil)
	}

	stringKeys := []string{
		dSettingsKeyVersion, dSettingsKeyMirrorSource, dSettingsKeyAppstoreRegion,
		dSettingsKeyRepository, dSettingsKeyMirrorsUrl, dSettingsKeyIdleDownloadConfig,
		DSettingsKeyDownloadSpeedLimit, dSettingsKeyLocalDownloadSpeedLimit,
		DSettingsKeyDeliveryRemoteDownloadGlobalLimit, DSettingsKeyDeliveryRemoteUploadGlobalLimit,
		DSettingsKeyDeliveryRemoteDownloadPeakLimit, DSettingsKeyDeliveryRemoteUploadPeakLimit,
		DSettingsKeyDeliveryRemoteDownloadOffPeakLimit, DSettingsKeyDeliveryRemoteUploadOffPeakLimit,
		dSettingsKeyDeliveryLocalDownloadGlobalLimit, dSettingsKeyDeliveryLocalUploadGlobalLimit,
		dSettingsKeyDeliveryLocalDownloadPeakLimit, dSettingsKeyDeliveryLocalUploadPeakLimit,
		dSettingsKeyDeliveryLocalDownloadOffPeakLimit, dSettingsKeyDeliveryLocalUploadOffPeakLimit,
		dSettingsKeyUpdateStatus, DSettingsKeyPlatformUrl, dSettingsKeyPlatformRepoComponents,
		dSettingsKeyPostUpgradeOnCalendar, dSettingsKeyUpdateTime, dSettingsKeyClientPackageName,
		dSettingsKeyUpgradeStatus,
	}
	for _, key := range stringKeys {
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), key).Return(dbus.MakeVariant("test-value"), nil)
	}

	int64Keys := []string{
		DSettingsKeyCheckInterval, dSettingsKeyCleanInterval, dSettingsKeyUpdateMode,
		dSettingsKeyCleanIntervalCacheOverLimit, dSettingsKeyAutoInstallUpdateType,
		DSettingsKeyLastoreDaemonStatus, dSettingsKeyCheckUpdateMode, dSettingsKeyPlatformDisabled,
	}
	for _, key := range int64Keys {
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), key).Return(dbus.MakeVariant(int64(3600)), nil)
	}

	timeStrKeys := []string{
		dSettingsKeyLastCheckTime, dSettingsKeyLastCleanTime, dSettingsKeyLastCheckCacheSizeTime,
	}
	for _, key := range timeStrKeys {
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), key).Return(dbus.MakeVariant("2024-01-01T00:00:00+08:00"), nil)
	}

	sliceVariantKeys := []string{
		dSettingsKeyAllowPostSystemUpgradeMessageVersion, dSettingsKeySystemSourceList,
		dSettingsKeyNonUnknownList, dSettingsKeySystemCustomSource, dSettingsKeySecurityCustomSource,
	}
	for _, key := range sliceVariantKeys {
		mgr.MockInterfaceManager.On("Value", dbus.Flags(0), key).Return(dbus.MakeVariant([]dbus.Variant{
			dbus.MakeVariant("elem1"),
		}), nil)
	}

	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyStartCheckRange).
		Return(dbus.MakeVariant([]dbus.Variant{
			dbus.MakeVariant(int64(1000)),
			dbus.MakeVariant(int64(2000)),
		}), nil)

	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeyUpgradeStatus).
		Return(dbus.MakeVariant(`{}`), nil)

	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySystemRepoType).
		Return(dbus.MakeVariant("UOS_DEFAULT"), nil)
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeySecurityRepoType).
		Return(dbus.MakeVariant("UOS_DEFAULT"), nil)
}

func TestLoadFromDSettingsSuccess(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }
	classifiedCachePath = filepath.Join(t.TempDir(), "classified_cache.json")
	onlineCachePath = filepath.Join(t.TempDir(), "online_cache.json")

	mgr := &ConfigManager.MockManager{}
	setupAllValueSuccess(mgr)
	mgr.MockInterfaceManager.On("SetValue", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})

	assert.True(t, c.useDSettings)
	assert.Equal(t, "test-value", c.Version)
	assert.True(t, c.AutoCheckUpdates)
	assert.True(t, c.AutoDownloadUpdates)
	assert.Equal(t, system.UpdateType(3600), c.UpdateMode)
	assert.Equal(t, "test-value", c.Repository)
	assert.True(t, c.PlatformUpdate)
	assert.True(t, c.IncludeDiskInfo)
	assert.Equal(t, []int{1000, 2000}, c.StartCheckRange)
	assert.True(t, c.UpgradeDeliveryEnabled)
}

func TestLoadFromDSettingsAllErrors(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }
	classifiedCachePath = filepath.Join(t.TempDir(), "classified_cache.json")
	onlineCachePath = filepath.Join(t.TempDir(), "online_cache.json")

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", mock.Anything, mock.Anything).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})

	assert.False(t, c.useDSettings)
	assert.Equal(t, "", c.Version)
	assert.False(t, c.AutoCheckUpdates)
	assert.True(t, c.IncludeDiskInfo)
	assert.Equal(t, []int{1800, 21600}, c.StartCheckRange)
}

func TestLoadFromDSettingsCheckRangeSwapped(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }
	classifiedCachePath = filepath.Join(t.TempDir(), "classified_cache.json")
	onlineCachePath = filepath.Join(t.TempDir(), "online_cache.json")

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyStartCheckRange).
		Return(dbus.MakeVariant([]dbus.Variant{
			dbus.MakeVariant(int64(5000)),
			dbus.MakeVariant(int64(2000)),
		}), nil)
	setupAllValueSuccess(mgr)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})

	assert.Equal(t, []int{2000, 5000}, c.StartCheckRange)
}

func TestLoadFromDSettingsCheckRangeInvalid(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }
	classifiedCachePath = filepath.Join(t.TempDir(), "classified_cache.json")
	onlineCachePath = filepath.Join(t.TempDir(), "online_cache.json")

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyStartCheckRange).
		Return(dbus.MakeVariant([]dbus.Variant{
			dbus.MakeVariant(int64(1000)),
		}), nil)
	setupAllValueSuccess(mgr)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})

	assert.Equal(t, []int{1800, 21600}, c.StartCheckRange)
}

func TestLoadFromDSettingsTimeParseError(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }
	classifiedCachePath = filepath.Join(t.TempDir(), "classified_cache.json")
	onlineCachePath = filepath.Join(t.TempDir(), "online_cache.json")

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeyLastCheckTime).
		Return(dbus.MakeVariant("invalid-time-format"), nil)
	setupAllValueSuccess(mgr)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})

	assert.True(t, c.LastCheckTime.IsZero())
}

func TestLoadFromDSettingsUpgradeStatusInvalidJSON(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }
	classifiedCachePath = filepath.Join(t.TempDir(), "classified_cache.json")
	onlineCachePath = filepath.Join(t.TempDir(), "online_cache.json")

	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), dSettingsKeyUpgradeStatus).
		Return(dbus.MakeVariant("not-json"), nil)
	setupAllValueSuccess(mgr)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})
}

func TestLoadFromDSettingsClassifiedCacheRead(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }

	dir := t.TempDir()
	classifiedCachePath = filepath.Join(dir, "classified_cache.json")
	onlineCachePath = filepath.Join(dir, "online_cache.json")

	classifiedData := `{"system":["pkg1","pkg2"]}`
	require.NoError(t, os.WriteFile(classifiedCachePath, []byte(classifiedData), 0644))
	require.NoError(t, os.WriteFile(onlineCachePath, []byte("online-cache-data"), 0644))

	mgr := &ConfigManager.MockManager{}
	setupAllValueSuccess(mgr)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})

	require.NotNil(t, c.ClassifiedUpdatablePackages)
	assert.Equal(t, []string{"pkg1", "pkg2"}, c.ClassifiedUpdatablePackages["system"])
	assert.Equal(t, "online-cache-data", c.OnlineCache)
}

func TestLoadFromDSettingsClassifiedCacheInvalidJSON(t *testing.T) {
	prevIsFileExist := isFileExistFn
	prevClassifiedPath := classifiedCachePath
	prevOnlinePath := onlineCachePath
	t.Cleanup(func() {
		isFileExistFn = prevIsFileExist
		classifiedCachePath = prevClassifiedPath
		onlineCachePath = prevOnlinePath
	})

	isFileExistFn = func(string) bool { return false }

	dir := t.TempDir()
	classifiedCachePath = filepath.Join(dir, "classified_cache.json")
	onlineCachePath = filepath.Join(dir, "online_cache.json")

	require.NoError(t, os.WriteFile(classifiedCachePath, []byte("invalid-json"), 0644))

	mgr := &ConfigManager.MockManager{}
	setupAllValueSuccess(mgr)

	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.loadFromDSettings()
	})
}

func TestSetClassifiedUpdatablePackagesWriteError(t *testing.T) {
	prevPath := classifiedCachePath
	t.Cleanup(func() { classifiedCachePath = prevPath })

	classifiedCachePath = "/nonexistent/dir/deep/path/classified_cache.json"

	cfg := &Config{}
	err := cfg.SetClassifiedUpdatablePackages(map[string][]string{"system": {"pkg1"}})
	require.Error(t, err)
}

func TestSetOnlineCacheWriteError(t *testing.T) {
	prevPath := onlineCachePath
	t.Cleanup(func() { onlineCachePath = prevPath })

	onlineCachePath = "/nonexistent/dir/deep/path/online_cache.json"

	cfg := &Config{}
	err := cfg.SetOnlineCache("test-cache")
	require.Error(t, err)
}

func TestHandleValueChangedIntranetError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyIntranetUpdate).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, IntranetUpdate: true}
	c.handleValueChanged(DSettingsKeyIntranetUpdate)
	assert.True(t, c.IntranetUpdate)
}

func TestHandleValueChangedPlatformUpdateError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyPlatformUpdate).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, PlatformUpdate: true}
	c.handleValueChanged(DSettingsKeyPlatformUpdate)
	assert.True(t, c.PlatformUpdate)
}

func TestHandleValueChangedGetHardwareIdByHelperError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyGetHardwareIdByHelper).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, GetHardwareIdByHelper: true}
	c.handleValueChanged(DSettingsKeyGetHardwareIdByHelper)
	assert.True(t, c.GetHardwareIdByHelper)
}

func TestHandleValueChangedIncludeDiskInfoError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyIncludeDiskInfo).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, IncludeDiskInfo: false}
	c.handleValueChanged(DSettingsKeyIncludeDiskInfo)
	assert.False(t, c.IncludeDiskInfo)
}

func TestHandleValueChangedIncrementalError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyIncrementalUpdate).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, IncrementalUpdate: true}
	c.handleValueChanged(DSettingsKeyIncrementalUpdate)
	assert.True(t, c.IncrementalUpdate)
}

func TestHandleValueChangedAutoDownloadError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyAutoDownloadUpdates).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, AutoDownloadUpdates: true}
	c.handleValueChanged(DSettingsKeyAutoDownloadUpdates)
	assert.True(t, c.AutoDownloadUpdates)
}

func TestHandleValueChangedUpgradeDeliveryError(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyUpgradeDeliveryEnabled).
		Return(dbus.Variant{}, errors.New("not available"))

	c := &Config{dsLastoreManager: mgr, UpgradeDeliveryEnabled: true}
	c.handleValueChanged(DSettingsKeyUpgradeDeliveryEnabled)
	assert.True(t, c.UpgradeDeliveryEnabled)
}

func TestHandleValueChangedUnknownKey(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	c := &Config{dsLastoreManager: mgr}
	require.NotPanics(t, func() {
		c.handleValueChanged("unknown-key-that-does-not-exist")
	})
}

func TestHandleValueChangedLastoreDaemonStatusNoChange(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("Value", dbus.Flags(0), DSettingsKeyLastoreDaemonStatus).
		Return(dbus.MakeVariant(int64(0)), nil)

	c := &Config{dsLastoreManager: mgr, lastoreDaemonStatus: 0}
	c.handleValueChanged(DSettingsKeyLastoreDaemonStatus)
	assert.Equal(t, LastoreDaemonStatus(0), c.lastoreDaemonStatus)
}

func TestSetUpgradeStatusAndReasonWithManager(t *testing.T) {
	mgr := &ConfigManager.MockManager{}
	mgr.MockInterfaceManager.On("SetValue", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	cfg := &Config{dsLastoreManager: mgr}
	status := system.UpgradeStatusAndReason{Status: "upgrading", ReasonCode: "001"}
	err := cfg.SetUpgradeStatusAndReason(status)
	require.NoError(t, err)
	assert.Equal(t, status, cfg.UpgradeStatus)
}
