// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCheckNil(t *testing.T) {
	err := executeCheck(nil)
	assert.Error(t, err)
}

func TestPreUpdateCheck(t *testing.T) {
	err := PreUpdateCheck()
	assert.NoError(t, err)
}

func TestPostUpdateCheck(t *testing.T) {
	err := PostUpdateCheck()
	assert.NoError(t, err)
}

func TestPreDownloadCheck(t *testing.T) {
	err := PreDownloadCheck()
	assert.NoError(t, err)
}

func TestPostDownloadCheck(t *testing.T) {
	err := PostDownloadCheck()
	assert.NoError(t, err)
}

func TestPreBackupCheck(t *testing.T) {
	err := PreBackupCheck()
	assert.NoError(t, err)
}

func TestPostBackupCheck(t *testing.T) {
	err := PostBackupCheck()
	assert.NoError(t, err)
}

func TestUpdatePostCheckStageStage1(t *testing.T) {
	PostCheckStage1 = true
	defer func() { PostCheckStage1 = false }()

	ThisCacheInfo = &cache.CacheInfo{}
	defer func() { ThisCacheInfo = nil }()

	updatePostCheckStage(cache.P_Run)
	assert.Equal(t, cache.P_Run, ThisCacheInfo.InternalState.IsPostCheckStage1)
}

func TestUpdatePostCheckStageStage2(t *testing.T) {
	PostCheckStage1 = false

	ThisCacheInfo = &cache.CacheInfo{}
	defer func() { ThisCacheInfo = nil }()

	updatePostCheckStage(cache.P_Run)
	assert.Equal(t, cache.P_Run, ThisCacheInfo.InternalState.IsPostCheckStage2)
}

func TestUpgradeChecksEnvError(t *testing.T) {
	old := UpdateMetaConfigPath
	UpdateMetaConfigPath = ""
	defer func() { UpdateMetaConfigPath = old }()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"PreUpgradeCheck", PreUpgradeCheck},
		{"MidUpgradeCheck", MidUpgradeCheck},
		{"PostUpgradeCheck", PostUpgradeCheck},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			require.Error(t, err)
			var jobErr *system.JobError
			require.ErrorAs(t, err, &jobErr)
			assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
		})
	}
}

func TestPreUpgradeCheckInternal(t *testing.T) {
	oldCache, oldSysPkg := ThisCacheInfo, SysPkgInfo
	ThisCacheInfo = &cache.CacheInfo{}
	SysPkgInfo = make(map[string]*cache.AppTinyInfo)
	defer func() { ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg }()

	err := preUpgradeCheck()
	require.NoError(t, err)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsPreCheck)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsDpkgAptPreCheck)
}

func TestMidUpgradeCheckInternal(t *testing.T) {
	oldCache := ThisCacheInfo
	ThisCacheInfo = &cache.CacheInfo{}
	defer func() { ThisCacheInfo = oldCache }()

	err := midUpgradeCheck()
	require.NoError(t, err)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsMidCheck)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsDpkgAptMidCheck)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsDependsMidCheck)
}

func TestPostUpgradeCheckInternal(t *testing.T) {
	oldCache, oldStage1 := ThisCacheInfo, PostCheckStage1
	ThisCacheInfo = &cache.CacheInfo{}
	PostCheckStage1 = false
	defer func() { ThisCacheInfo, PostCheckStage1 = oldCache, oldStage1 }()

	err := postUpgradeCheck()
	// Result depends on display-manager.service state on the host.
	if err != nil {
		var jobErr *system.JobError
		assert.ErrorAs(t, err, &jobErr)
	}
}

func TestPostCheckWithStage(t *testing.T) {
	oldCache, oldStage1 := ThisCacheInfo, PostCheckStage1
	ThisCacheInfo = &cache.CacheInfo{}
	PostCheckStage1 = true
	defer func() { ThisCacheInfo, PostCheckStage1 = oldCache, oldStage1 }()

	err := postCheckWithStage(check.Stage1)
	if err != nil {
		var jobErr *system.JobError
		assert.ErrorAs(t, err, &jobErr)
	}
}

// setupCheckEnv writes a valid config and update meta JSON into a temp dir and
// points the package-level globals at them so initCheckEnv can succeed.
func setupCheckEnv(t *testing.T) {
	t.Helper()
	oldCfg, oldPath := ConfigCfg, UpdateMetaConfigPath
	oldRoot, oldCache, oldSysPkg := RootCoreConfig, ThisCacheInfo, SysPkgInfo
	t.Cleanup(func() {
		ConfigCfg, UpdateMetaConfigPath = oldCfg, oldPath
		RootCoreConfig = oldRoot
		ThisCacheInfo = oldCache
		SysPkgInfo = oldSysPkg
	})

	dir := t.TempDir()
	ConfigCfg = filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(ConfigCfg, []byte("Base: "+dir+"\nDynHookTimeout: 60\n"), 0644))
	UpdateMetaConfigPath = filepath.Join(dir, "metacfg.json")
	require.NoError(t, os.WriteFile(UpdateMetaConfigPath, []byte(`{"UUID":"test-uuid","PkgDebPath":"/tmp/debs"}`), 0644))
}

func TestBeforeCheckVerifyError(t *testing.T) {
	setupCheckEnv(t)
	old := checkVerifyCacheInfoFn
	checkVerifyCacheInfoFn = func(*cache.CacheInfo) error { return errors.New("verify failed") }
	t.Cleanup(func() { checkVerifyCacheInfoFn = old })

	err := beforeCheck()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorCheckMetaInfoFile, jobErr.ErrType)
	assert.Equal(t, cache.P_Error, ThisCacheInfo.InternalState.IsMetaInfoFormatCheck)
}

func TestBeforeCheckSuccess(t *testing.T) {
	setupCheckEnv(t)
	old := checkVerifyCacheInfoFn
	checkVerifyCacheInfoFn = func(*cache.CacheInfo) error { return nil }
	t.Cleanup(func() { checkVerifyCacheInfoFn = old })

	err := beforeCheck()
	require.NoError(t, err)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsMetaInfoFormatCheck)
}

func TestPrePostCheckDynHookError(t *testing.T) {
	old := checkDynHookFn
	checkDynHookFn = func(int8) error { return errors.New("hook failed") }
	t.Cleanup(func() { checkDynHookFn = old })

	cases := []struct {
		name string
		fn   func() error
		want system.JobErrorType
	}{
		{"PreUpdateCheck", PreUpdateCheck, system.ErrorPreUpdateCheckScriptsFailed},
		{"PostUpdateCheck", PostUpdateCheck, system.ErrorPostUpdateCheckScriptsFailed},
		{"PreDownloadCheck", PreDownloadCheck, system.ErrorPreDownloadCheckScriptsFailed},
		{"PostDownloadCheck", PostDownloadCheck, system.ErrorPostDownloadCheckScriptsFailed},
		{"PreBackupCheck", PreBackupCheck, system.ErrorPreBackupCheckScriptsFailed},
		{"PostBackupCheck", PostBackupCheck, system.ErrorPostBackupCheckScriptsFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			var jobErr *system.JobError
			require.ErrorAs(t, err, &jobErr)
			assert.Equal(t, tc.want, jobErr.ErrType)
		})
	}
}

func TestPreUpgradeCheckDynHookError(t *testing.T) {
	oldCache, oldSysPkg := ThisCacheInfo, SysPkgInfo
	old := checkDynHookFn
	checkDynHookFn = func(int8) error { return errors.New("hook failed") }
	t.Cleanup(func() {
		checkDynHookFn = old
		ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
	})

	ThisCacheInfo = &cache.CacheInfo{}
	SysPkgInfo = make(map[string]*cache.AppTinyInfo)

	err := preUpgradeCheck()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorPreCheckScriptsFailed, jobErr.ErrType)
	assert.Equal(t, cache.P_Stage0_Failed, ThisCacheInfo.InternalState.IsPreCheck)
}

func TestPreUpgradeCheckLoadSysPkgError(t *testing.T) {
	oldCache, oldSysPkg := ThisCacheInfo, SysPkgInfo
	oldDyn, oldLoad := checkDynHookFn, loadSysPkgInfoFn
	checkDynHookFn = func(int8) error { return nil }
	loadSysPkgInfoFn = func(map[string]*cache.AppTinyInfo) error { return errors.New("load failed") }
	t.Cleanup(func() {
		checkDynHookFn, loadSysPkgInfoFn = oldDyn, oldLoad
		ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
	})

	ThisCacheInfo = &cache.CacheInfo{}
	SysPkgInfo = make(map[string]*cache.AppTinyInfo)

	err := preUpgradeCheck()
	require.Error(t, err)
	assert.Equal(t, cache.P_Stage1_Failed, ThisCacheInfo.InternalState.IsPreCheck)
}

func TestPreUpgradeCheckRepoLoadError(t *testing.T) {
	oldCache, oldSysPkg := ThisCacheInfo, SysPkgInfo
	oldDyn, oldLoad := checkDynHookFn, loadSysPkgInfoFn
	checkDynHookFn = func(int8) error { return nil }
	loadSysPkgInfoFn = func(map[string]*cache.AppTinyInfo) error { return nil }
	t.Cleanup(func() {
		checkDynHookFn, loadSysPkgInfoFn = oldDyn, oldLoad
		ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
	})

	ThisCacheInfo = &cache.CacheInfo{
		UpdateMetaInfo: cache.UpdateInfo{
			RepoBackend: []cache.RepoInfo{{Name: "test", FilePath: "/nonexistent/repo/file"}},
		},
	}
	SysPkgInfo = make(map[string]*cache.AppTinyInfo)

	err := preUpgradeCheck()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorMetaInfoFile, jobErr.ErrType)
	assert.Equal(t, cache.P_Stage1_Failed, ThisCacheInfo.InternalState.IsPreCheck)
}

func TestPreUpgradeCheckAPTStateError(t *testing.T) {
	oldCache, oldSysPkg := ThisCacheInfo, SysPkgInfo
	oldDyn, oldLoad, oldAPT := checkDynHookFn, loadSysPkgInfoFn, checkAPTAndDPKGStateFn
	checkDynHookFn = func(int8) error { return nil }
	loadSysPkgInfoFn = func(map[string]*cache.AppTinyInfo) error { return nil }
	checkAPTAndDPKGStateFn = func() error { return errors.New("apt state failed") }
	t.Cleanup(func() {
		checkDynHookFn, loadSysPkgInfoFn, checkAPTAndDPKGStateFn = oldDyn, oldLoad, oldAPT
		ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
	})

	ThisCacheInfo = &cache.CacheInfo{}
	SysPkgInfo = make(map[string]*cache.AppTinyInfo)

	err := preUpgradeCheck()
	require.Error(t, err)
	assert.Equal(t, cache.P_Error, ThisCacheInfo.InternalState.IsDpkgAptPreCheck)
	assert.Equal(t, cache.P_Stage2_Failed, ThisCacheInfo.InternalState.IsPreCheck)
}

func TestPreUpgradeCheckNonblockErrors(t *testing.T) {
	oldCache, oldSysPkg := ThisCacheInfo, SysPkgInfo
	oldDyn, oldLoad, oldAPT, oldVer := checkDynHookFn, loadSysPkgInfoFn, checkAPTAndDPKGStateFn, checkDPKGVersionSupportFn
	t.Cleanup(func() {
		checkDynHookFn, loadSysPkgInfoFn, checkAPTAndDPKGStateFn, checkDPKGVersionSupportFn = oldDyn, oldLoad, oldAPT, oldVer
		ThisCacheInfo, SysPkgInfo = oldCache, oldSysPkg
	})

	checkDynHookFn = func(int8) error { return nil }
	loadCalls := 0
	loadSysPkgInfoFn = func(map[string]*cache.AppTinyInfo) error {
		loadCalls++
		if loadCalls == 1 {
			return nil
		}
		return errors.New("nonblock load failed")
	}
	checkAPTAndDPKGStateFn = func() error { return nil }
	checkDPKGVersionSupportFn = func(map[string]*cache.AppTinyInfo) error {
		return errors.New("dpkg version not supported")
	}

	ThisCacheInfo = &cache.CacheInfo{}
	SysPkgInfo = make(map[string]*cache.AppTinyInfo)

	err := preUpgradeCheck()
	require.NoError(t, err)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsPreCheck)
}

func TestMidUpgradeCheckAPTStateError(t *testing.T) {
	oldCache := ThisCacheInfo
	old := checkAPTAndDPKGStateFn
	checkAPTAndDPKGStateFn = func() error { return errors.New("apt failed") }
	t.Cleanup(func() {
		checkAPTAndDPKGStateFn = old
		ThisCacheInfo = oldCache
	})

	ThisCacheInfo = &cache.CacheInfo{}

	err := midUpgradeCheck()
	require.Error(t, err)
	assert.Equal(t, cache.P_Error, ThisCacheInfo.InternalState.IsDpkgAptMidCheck)
	assert.Equal(t, cache.P_Stage0_Failed, ThisCacheInfo.InternalState.IsMidCheck)
}

func TestMidUpgradeCheckPkgDependencyError(t *testing.T) {
	oldCache := ThisCacheInfo
	oldAPT, oldDep := checkAPTAndDPKGStateFn, checkPkgDependencyFn
	checkAPTAndDPKGStateFn = func() error { return nil }
	checkPkgDependencyFn = func() error { return errors.New("depends failed") }
	t.Cleanup(func() {
		checkAPTAndDPKGStateFn, checkPkgDependencyFn = oldAPT, oldDep
		ThisCacheInfo = oldCache
	})

	ThisCacheInfo = &cache.CacheInfo{}

	err := midUpgradeCheck()
	require.Error(t, err)
	assert.Equal(t, cache.P_Error, ThisCacheInfo.InternalState.IsDependsMidCheck)
	assert.Equal(t, cache.P_Stage0_Failed, ThisCacheInfo.InternalState.IsMidCheck)
}

func TestMidUpgradeCheckRootDiskBlockError(t *testing.T) {
	oldCache := ThisCacheInfo
	oldAPT, oldDep, oldDisk := checkAPTAndDPKGStateFn, checkPkgDependencyFn, checkRootDiskFreeSpaceFn
	checkAPTAndDPKGStateFn = func() error { return nil }
	checkPkgDependencyFn = func() error { return nil }
	checkRootDiskFreeSpaceFn = func(uint64) error { return errors.New("disk full") }
	t.Cleanup(func() {
		checkAPTAndDPKGStateFn, checkPkgDependencyFn, checkRootDiskFreeSpaceFn = oldAPT, oldDep, oldDisk
		ThisCacheInfo = oldCache
	})

	ThisCacheInfo = &cache.CacheInfo{}

	err := midUpgradeCheck()
	require.Error(t, err)
	assert.Equal(t, cache.P_Stage0_Failed, ThisCacheInfo.InternalState.IsMidCheck)
}

func TestMidUpgradeCheckNonblockRootDiskError(t *testing.T) {
	oldCache := ThisCacheInfo
	oldAPT, oldDep, oldDisk, oldDyn := checkAPTAndDPKGStateFn, checkPkgDependencyFn, checkRootDiskFreeSpaceFn, checkDynHookFn
	t.Cleanup(func() {
		checkAPTAndDPKGStateFn, checkPkgDependencyFn, checkRootDiskFreeSpaceFn, checkDynHookFn = oldAPT, oldDep, oldDisk, oldDyn
		ThisCacheInfo = oldCache
	})

	checkAPTAndDPKGStateFn = func() error { return nil }
	checkPkgDependencyFn = func() error { return nil }
	diskCalls := 0
	checkRootDiskFreeSpaceFn = func(need uint64) error {
		diskCalls++
		if diskCalls == 1 {
			return nil
		}
		return errors.New("less than 50M free")
	}
	checkDynHookFn = func(int8) error { return nil }

	ThisCacheInfo = &cache.CacheInfo{}

	err := midUpgradeCheck()
	require.NoError(t, err)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsMidCheck)
}

func TestMidUpgradeCheckDynHookError(t *testing.T) {
	oldCache := ThisCacheInfo
	oldAPT, oldDep, oldDisk, oldDyn := checkAPTAndDPKGStateFn, checkPkgDependencyFn, checkRootDiskFreeSpaceFn, checkDynHookFn
	t.Cleanup(func() {
		checkAPTAndDPKGStateFn, checkPkgDependencyFn, checkRootDiskFreeSpaceFn, checkDynHookFn = oldAPT, oldDep, oldDisk, oldDyn
		ThisCacheInfo = oldCache
	})

	checkAPTAndDPKGStateFn = func() error { return nil }
	checkPkgDependencyFn = func() error { return nil }
	checkRootDiskFreeSpaceFn = func(uint64) error { return nil }
	checkDynHookFn = func(int8) error { return errors.New("hook failed") }

	ThisCacheInfo = &cache.CacheInfo{}

	err := midUpgradeCheck()
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorMidCheckScriptsFailed, jobErr.ErrType)
	assert.Equal(t, cache.P_Stage2_Failed, ThisCacheInfo.InternalState.IsMidCheck)
}

func TestPostCheckWithStageServiceError(t *testing.T) {
	oldCache, oldStage1 := ThisCacheInfo, PostCheckStage1
	old := checkImportantServiceFn
	checkImportantServiceFn = func(string) error { return errors.New("service down") }
	t.Cleanup(func() {
		checkImportantServiceFn = old
		ThisCacheInfo, PostCheckStage1 = oldCache, oldStage1
	})

	ThisCacheInfo = &cache.CacheInfo{}
	PostCheckStage1 = true

	err := postCheckWithStage(check.Stage1)
	require.Error(t, err)
	assert.Equal(t, cache.P_Stage0_Failed, ThisCacheInfo.InternalState.IsPostCheckStage1)
}

func TestPostCheckWithStageDynHookError(t *testing.T) {
	oldCache, oldStage1 := ThisCacheInfo, PostCheckStage1
	oldSvc, oldDyn := checkImportantServiceFn, checkDynHookFn
	checkImportantServiceFn = func(string) error { return nil }
	checkDynHookFn = func(int8) error { return errors.New("hook failed") }
	t.Cleanup(func() {
		checkImportantServiceFn, checkDynHookFn = oldSvc, oldDyn
		ThisCacheInfo, PostCheckStage1 = oldCache, oldStage1
	})

	ThisCacheInfo = &cache.CacheInfo{}
	PostCheckStage1 = false

	err := postCheckWithStage(check.Stage2)
	var jobErr *system.JobError
	require.ErrorAs(t, err, &jobErr)
	assert.Equal(t, system.ErrorPostCheckScriptsFailed, jobErr.ErrType)
	assert.Equal(t, cache.P_Stage2_Failed, ThisCacheInfo.InternalState.IsPostCheckStage2)
}

func TestPostCheckWithStageStage1Success(t *testing.T) {
	oldCache, oldStage1 := ThisCacheInfo, PostCheckStage1
	old := checkImportantServiceFn
	checkImportantServiceFn = func(string) error { return nil }
	t.Cleanup(func() {
		checkImportantServiceFn = old
		ThisCacheInfo, PostCheckStage1 = oldCache, oldStage1
	})

	ThisCacheInfo = &cache.CacheInfo{}
	PostCheckStage1 = true

	err := postCheckWithStage(check.Stage1)
	require.NoError(t, err)
	assert.Equal(t, cache.P_OK, ThisCacheInfo.InternalState.IsPostCheckStage1)
}
