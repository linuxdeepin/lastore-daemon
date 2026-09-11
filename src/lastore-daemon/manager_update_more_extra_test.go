// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareUpdateSource(t *testing.T) {
	prepareUpdateSource()
}

func TestBeforeUpdateSourceEnvCheck(t *testing.T) {
	m := &Manager{updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false)}
	_, _ = m.beforeUpdateSourceEnvCheck()
}

func TestUpdateSourceImmutableAutoRecovery(t *testing.T) {
	job, err := (&Manager{ImmutableAutoRecovery: true}).updateSource(ifcTestSender)
	assert.Error(t, err)
	assert.Nil(t, job)
}

func TestRefreshThrottlingFromPlatformNoNetwork(t *testing.T) {
	m := &Manager{
		config:         newTestConfig(t),
		updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false),
	}
	assert.Error(t, m.refreshThrottlingFromPlatform())
}

func TestGenerateUpdateInfo(t *testing.T) {
	m := &Manager{updater: &Updater{service: newTestService(), config: newTestConfig(t)}}
	_ = m.generateUpdateInfo()
}

func TestGetSecurityUpgradablePackagesMap(t *testing.T) {
	_, _, _ = getSecurityUpgradablePackagesMap(nil)
}

func TestGetUnknownUpgradablePackagesMap(t *testing.T) {
	_, _, _ = getUnknownUpgradablePackagesMap(nil)
}

func TestGetSystemUpgradablePackageList(t *testing.T) {
	_, _ = getSystemUpgradablePackageList(nil)
}

func TestGetSecurityUpgradablePackageList(t *testing.T) {
	_, _ = getSecurityUpgradablePackageList(nil)
}

func TestGetUnknownUpgradablePackageList(t *testing.T) {
	_, _ = getUnknownUpgradablePackageList(nil)
}

func TestLoadPkgStatusVersion(t *testing.T) {
	result, err := loadPkgStatusVersion()
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestListDistUpgradePackages(t *testing.T) {
	_, _ = listDistUpgradePackages(system.SystemUpdate)
}

func TestGetCoreListDisabled(t *testing.T) {
	m := &Manager{config: newTestConfig(t)}
	assert.Nil(t, m.getCoreList(false))
}

func TestRefreshUpdateInfosAsync(t *testing.T) {
	newFullManager(t).refreshUpdateInfos(false)
}

func TestUpdateUpdatablePropEmpty(t *testing.T) {
	m := &Manager{updater: &Updater{service: newTestService(), config: newTestConfig(t)}}
	m.updateUpdatableProp(map[string][]string{})
}

func TestEnsureUpdateSourceOnceAlreadyDone(t *testing.T) {
	(&Manager{updateSourceOnce: true}).ensureUpdateSourceOnce()
}

func TestHandleUpdateSourceFailed(t *testing.T) {
	handleUpdateSourceFailed(&Job{})
}

func TestPrepareUpdateSourceRedirected(t *testing.T) {
	partial := filepath.Join(t.TempDir(), "partial")
	require.NoError(t, os.MkdirAll(partial, 0755))
	stale := filepath.Join(partial, "stale.list")
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0644))

	orig := prepareUpdateSourcePartialPaths
	prepareUpdateSourcePartialPaths = []string{partial, filepath.Join(t.TempDir(), "missing")}
	t.Cleanup(func() { prepareUpdateSourcePartialPaths = orig })

	prepareUpdateSource()

	_, err := os.Stat(stale)
	assert.True(t, os.IsNotExist(err), "stale partial file should be removed")
}

func TestGetCoreListOfflineEnabled(t *testing.T) {
	orig := coreListVarPath
	coreListVarPath = filepath.Join(t.TempDir(), "corelist")
	t.Cleanup(func() { coreListVarPath = orig })

	data, _ := json.Marshal(PackageList{PkgList: []Package{{PkgName: "pkg-a"}, {PkgName: "pkg-b"}}})
	require.NoError(t, os.WriteFile(coreListVarPath, data, 0644))

	m := &Manager{config: newTestConfig(t)}
	m.config.EnableCoreList = true
	assert.Equal(t, []string{"pkg-a", "pkg-b"}, m.getCoreList(false))
}

func TestGetCoreListOnlineBranch(t *testing.T) {
	m := &Manager{config: newTestConfig(t)}
	m.config.EnableCoreList = true
	_ = m.getCoreList(true)
}

func TestRefreshThrottlingFromPlatformValidJSON(t *testing.T) {
	m := &Manager{
		config:         newTestConfig(t),
		updatePlatform: updateplatform.NewUpdatePlatformManager(newTestConfig(t), false),
	}
	m.config.LocalDownloadSpeedLimitConfig = `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"1024","IsOnlineSpeedLimit":false}`
	assert.Error(t, m.refreshThrottlingFromPlatform())
}
