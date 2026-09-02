// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	tmpfile, err := os.CreateTemp(t.TempDir(), "config-*.json")
	require.NoError(t, err)
	tmpfile.Close()
	cfg := &Config{filePath: tmpfile.Name()}
	return cfg
}

func TestSetVersion(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetVersion("2.0")
	require.NoError(t, err)
	assert.Equal(t, "2.0", cfg.Version)
}

func TestSetDisableUpdateMetadata(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDisableUpdateMetadata(true)
	require.NoError(t, err)
	assert.True(t, cfg.DisableUpdateMetadata)
}

func TestSetIncrementalUpdate(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetIncrementalUpdate(true)
	require.NoError(t, err)
	assert.True(t, cfg.IncrementalUpdate)
}

func TestSetCheckUpdateMode(t *testing.T) {
	cfg := newTestConfig(t)
	mode := system.SystemUpdate | system.SecurityUpdate
	err := cfg.SetCheckUpdateMode(mode)
	require.NoError(t, err)
	assert.Equal(t, mode, cfg.CheckUpdateMode)
}

func TestSetCleanIntervalCacheOverLimit(t *testing.T) {
	cfg := newTestConfig(t)
	d := time.Hour
	err := cfg.SetCleanIntervalCacheOverLimit(d)
	require.NoError(t, err)
	assert.Equal(t, d, cfg.CleanIntervalCacheOverLimit)
}

func TestSetAutoInstallUpdates(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetAutoInstallUpdates(true)
	require.NoError(t, err)
	assert.True(t, cfg.AutoInstallUpdates)
}

func TestSetAutoInstallUpdateType(t *testing.T) {
	cfg := newTestConfig(t)
	typ := system.SystemUpdate
	err := cfg.SetAutoInstallUpdateType(typ)
	require.NoError(t, err)
	assert.Equal(t, typ, cfg.AutoInstallUpdateType)
}

func TestSetAllowPostSystemUpgradeMessageVersion(t *testing.T) {
	cfg := newTestConfig(t)
	v := []string{"25.0", "25.1"}
	err := cfg.SetAllowPostSystemUpgradeMessageVersion(v)
	require.NoError(t, err)
	assert.Equal(t, v, cfg.AllowPostSystemUpgradeMessageVersion)
}

func TestSetUpgradeStatusAndReason(t *testing.T) {
	cfg := newTestConfig(t)
	status := system.UpgradeStatusAndReason{Status: "idle", ReasonCode: ""}
	err := cfg.SetUpgradeStatusAndReason(status)
	require.NoError(t, err)
	assert.Equal(t, status, cfg.UpgradeStatus)
}

func TestSetUseDSettings(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetUseDSettings(true)
	require.NoError(t, err)
	assert.True(t, cfg.useDSettings)
}

func TestSetIdleDownloadConfig(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetIdleDownloadConfig(`{"idle":true}`)
	require.NoError(t, err)
	assert.Equal(t, `{"idle":true}`, cfg.IdleDownloadConfig)
}

func TestSetCheckInterval(t *testing.T) {
	cfg := newTestConfig(t)
	d := 2 * time.Hour
	err := cfg.SetCheckInterval(d)
	require.NoError(t, err)
	assert.Equal(t, d, cfg.CheckInterval)
}

func TestSetCleanInterval(t *testing.T) {
	cfg := newTestConfig(t)
	d := 3 * time.Hour
	err := cfg.SetCleanInterval(d)
	require.NoError(t, err)
	assert.Equal(t, d, cfg.CleanInterval)
}

func TestSetRepository(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetRepository("desktop")
	require.NoError(t, err)
	assert.Equal(t, "desktop", cfg.Repository)
}

func TestSetMirrorsUrl(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetMirrorsUrl("https://mirrors.example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://mirrors.example.com", cfg.MirrorsUrl)
}

func TestSetLastCheckTime(t *testing.T) {
	cfg := newTestConfig(t)
	now := time.Now()
	err := cfg.SetLastCheckTime(now)
	require.NoError(t, err)
	assert.Equal(t, now.Format(configTimeLayout), cfg.LastCheckTime.Format(configTimeLayout))
}

func TestSetDownloadSpeedLimitConfig(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDownloadSpeedLimitConfig(`{"limit":1000}`)
	require.NoError(t, err)
	assert.Equal(t, `{"limit":1000}`, cfg.DownloadSpeedLimitConfig)
}

func TestSetLocalDownloadSpeedLimitConfig(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetLocalDownloadSpeedLimitConfig(`{"limit":500}`)
	require.NoError(t, err)
	assert.Equal(t, `{"limit":500}`, cfg.LocalDownloadSpeedLimitConfig)
}

func TestSetDeliveryRemoteDownloadGlobalLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryRemoteDownloadGlobalLimit("10000")
	require.NoError(t, err)
	assert.Equal(t, "10000", cfg.DeliveryRemoteDownloadGlobalLimit)
}

func TestSetDeliveryRemoteUploadGlobalLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryRemoteUploadGlobalLimit("5000")
	require.NoError(t, err)
	assert.Equal(t, "5000", cfg.DeliveryRemoteUploadGlobalLimit)
}

func TestSetDeliveryRemoteDownloadPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryRemoteDownloadPeakLimit("20000")
	require.NoError(t, err)
	assert.Equal(t, "20000", cfg.DeliveryRemoteDownloadPeakLimit)
}

func TestSetDeliveryRemoteUploadPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryRemoteUploadPeakLimit("10000")
	require.NoError(t, err)
	assert.Equal(t, "10000", cfg.DeliveryRemoteUploadPeakLimit)
}

func TestSetDeliveryRemoteDownloadOffPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryRemoteDownloadOffPeakLimit("30000")
	require.NoError(t, err)
	assert.Equal(t, "30000", cfg.DeliveryRemoteDownloadOffPeakLimit)
}

func TestSetDeliveryRemoteUploadOffPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryRemoteUploadOffPeakLimit("15000")
	require.NoError(t, err)
	assert.Equal(t, "15000", cfg.DeliveryRemoteUploadOffPeakLimit)
}

func TestSetDeliveryLocalDownloadGlobalLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryLocalDownloadGlobalLimit("8000")
	require.NoError(t, err)
	assert.Equal(t, "8000", cfg.DeliveryLocalDownloadGlobalLimit)
}

func TestSetDeliveryLocalUploadGlobalLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryLocalUploadGlobalLimit("4000")
	require.NoError(t, err)
	assert.Equal(t, "4000", cfg.DeliveryLocalUploadGlobalLimit)
}

func TestSetDeliveryLocalDownloadPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryLocalDownloadPeakLimit("12000")
	require.NoError(t, err)
	assert.Equal(t, "12000", cfg.DeliveryLocalDownloadPeakLimit)
}

func TestSetDeliveryLocalUploadPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryLocalUploadPeakLimit("6000")
	require.NoError(t, err)
	assert.Equal(t, "6000", cfg.DeliveryLocalUploadPeakLimit)
}

func TestSetDeliveryLocalDownloadOffPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryLocalDownloadOffPeakLimit("16000")
	require.NoError(t, err)
	assert.Equal(t, "16000", cfg.DeliveryLocalDownloadOffPeakLimit)
}

func TestSetDeliveryLocalUploadOffPeakLimit(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetDeliveryLocalUploadOffPeakLimit("8000")
	require.NoError(t, err)
	assert.Equal(t, "8000", cfg.DeliveryLocalUploadOffPeakLimit)
}

func TestSetLastoreDaemonStatus(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetLastoreDaemonStatus(CanUpgrade | ForceUpdate)
	require.NoError(t, err)
	assert.Equal(t, CanUpgrade|ForceUpdate, cfg.GetLastoreDaemonStatus())
}

func TestUpdateLastoreDaemonStatus(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.UpdateLastoreDaemonStatus(CanUpgrade, true)
	require.NoError(t, err)
	assert.Equal(t, CanUpgrade, cfg.GetLastoreDaemonStatus())

	err = cfg.UpdateLastoreDaemonStatus(ForceUpdate, true)
	require.NoError(t, err)
	assert.Equal(t, CanUpgrade|ForceUpdate, cfg.GetLastoreDaemonStatus())

	err = cfg.UpdateLastoreDaemonStatus(CanUpgrade, false)
	require.NoError(t, err)
	assert.Equal(t, ForceUpdate, cfg.GetLastoreDaemonStatus())
}

func TestGetLastoreDaemonStatusByBit(t *testing.T) {
	cfg := newTestConfig(t)
	_ = cfg.SetLastoreDaemonStatus(CanUpgrade | DisableUpdate)
	assert.Equal(t, CanUpgrade, cfg.GetLastoreDaemonStatusByBit(CanUpgrade))
	assert.Equal(t, DisableUpdate, cfg.GetLastoreDaemonStatusByBit(DisableUpdate))
	assert.Equal(t, LastoreDaemonStatus(0), cfg.GetLastoreDaemonStatusByBit(ForceUpdate))
}

func TestSetUpdateStatus(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetUpdateStatus("updating")
	require.NoError(t, err)
	assert.Equal(t, "updating", cfg.UpdateStatus)
}

func TestSetInstallUpdateTime(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetInstallUpdateTime("02:00")
	require.NoError(t, err)
	assert.Equal(t, "02:00", cfg.UpdateTime)
}

func TestSetSystemCustomSource(t *testing.T) {
	cfg := newTestConfig(t)
	src := []string{"deb http://example.com stable main"}
	err := cfg.SetSystemCustomSource(src)
	require.NoError(t, err)
	assert.Equal(t, src, cfg.SystemCustomSource)
}

func TestSetSecurityCustomSource(t *testing.T) {
	cfg := newTestConfig(t)
	src := []string{"deb http://security.example.com stable main"}
	err := cfg.SetSecurityCustomSource(src)
	require.NoError(t, err)
	assert.Equal(t, src, cfg.SecurityCustomSource)
}

func TestSetSystemRepoType(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetSystemRepoType(CustomRepo)
	require.NoError(t, err)
	assert.Equal(t, CustomRepo, cfg.SystemRepoType)
}

func TestSetSecurityRepoType(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetSecurityRepoType(OemDefaultRepo)
	require.NoError(t, err)
	assert.Equal(t, OemDefaultRepo, cfg.SecurityRepoType)
}

func TestSetUpgradeDeliveryEnabled(t *testing.T) {
	cfg := newTestConfig(t)
	err := cfg.SetUpgradeDeliveryEnabled(true)
	require.NoError(t, err)
	assert.True(t, cfg.UpgradeDeliveryEnabled)
}

func TestSetStartCheckRange(t *testing.T) {
	cfg := newTestConfig(t)
	r := []int{0, 300}
	err := cfg.SetStartCheckRange(r)
	require.NoError(t, err)
	assert.Equal(t, r, cfg.StartCheckRange)
}

func TestConnectConfigChanged(t *testing.T) {
	cfg := newTestConfig(t)
	called := false
	cfg.ConnectConfigChanged("testKey", func(old, new interface{}) {
		called = true
	})
	assert.NotNil(t, cfg.dsettingsChangedCbMap["testKey"])
	cb := cfg.dsettingsChangedCbMap["testKey"]
	cb(nil, nil)
	assert.True(t, called)
}

func TestDisableConsoleLogging(t *testing.T) {
	DisableConsoleLogging()
}

func TestJson2DSettings(t *testing.T) {
	cfg := newTestConfig(t)
	old := &Config{
		Version:           "1.0",
		AutoCheckUpdates:  true,
		MirrorSource:      "test",
		AppstoreRegion:    "CN",
		Repository:        "desktop",
		MirrorsUrl:        "https://mirrors.test.com",
		UpdateMode:        system.SystemUpdate,
		CheckInterval:     time.Hour,
		CleanInterval:     time.Hour * 2,
		AutoInstallUpdates: true,
	}
	cfg.json2DSettings(old)
	assert.Equal(t, "1.0", cfg.Version)
	assert.True(t, cfg.AutoCheckUpdates)
	assert.Equal(t, "test", cfg.MirrorSource)
	assert.Equal(t, "CN", cfg.AppstoreRegion)
	assert.Equal(t, "desktop", cfg.Repository)
	assert.Equal(t, "https://mirrors.test.com", cfg.MirrorsUrl)
	assert.Equal(t, system.SystemUpdate, cfg.UpdateMode)
	assert.True(t, cfg.AutoInstallUpdates)
}
