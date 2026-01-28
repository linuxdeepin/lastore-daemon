// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
)

var (
	logger          = log.NewLogger("lastore/update-tools")
	PostCheckStage1 bool
	SysPkgInfo      map[string]*cache.AppTinyInfo
)

func beforeCheck() error {
	err := initCheckEnv()
	if err != nil {
		logger.Debugf("check environment initialization failed: %v", err)
		return err
	}

	logger.Debug("verifying update metadata")
	if err := check.CheckVerifyCacheInfo(ThisCacheInfo); err != nil {
		ThisCacheInfo.InternalState.IsMetaInfoFormatCheck = cache.P_Error
		logger.Errorf("check meta info failed: %+v", err)
		return &system.JobError{
			ErrType:      system.ErrorCheckMetaInfoFile,
			ErrDetail:    fmt.Sprintf("check meta info failed: %v", err),
			IsCheckError: true,
		}
	}
	ThisCacheInfo.InternalState.IsMetaInfoFormatCheck = cache.P_OK
	return nil
}

// executeCheck wraps the common pre/post logic for all check functions
func executeCheck(checkFunc func() error) error {
	if checkFunc == nil {
		logger.Debug("check function is nil")
		return &system.JobError{
			ErrType:      system.ErrorCheckMetaInfoFile,
			ErrDetail:    "check function is nil",
			IsCheckError: true,
		}
	}
	err := beforeCheck()
	if err != nil {
		logger.Debugf("check initialization failed: %v", err)
		return err
	}

	return checkFunc()
}

func PreUpdateCheck() error {
	if err := check.CheckDynHook(cache.PreUpdateCheck); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorPreUpdateCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("pre_update_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

func PostUpdateCheck() error {
	if err := check.CheckDynHook(cache.PostUpdateCheck); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorPostUpdateCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("post_update_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

func PreDownloadCheck() error {
	if err := check.CheckDynHook(cache.PreDownloadCheck); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorPreDownloadCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("pre_download_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

func PostDownloadCheck() error {
	if err := check.CheckDynHook(cache.PostDownloadCheck); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorPostDownloadCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("post_download_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

func PreBackupCheck() error {
	if err := check.CheckDynHook(cache.PreBackupCheck); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorPreBackupCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("pre_backup_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

func PostBackupCheck() error {
	if err := check.CheckDynHook(cache.PostBackupCheck); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorPostBackupCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("post_backup_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

// PreUpgradeCheck do pre-check
func PreUpgradeCheck() error {
	return executeCheck(preUpgradeCheck)
}

// MidCheck do mid-check
func MidUpgradeCheck() error {
	return executeCheck(midUpgradeCheck)
}

// PostCheck do post-check
func PostUpgradeCheck() error {
	return executeCheck(postUpgradeCheck)
}

func preUpgradeCheck() error {
	// 动态检查 DONE:(DingHao)待修改返回值结构体
	// 动态hook 脚本检查，阻塞
	ThisCacheInfo.InternalState.IsPreCheck = cache.P_Run

	logger.Info("pre_upgrade_check/dynhook start")

	if err := check.CheckDynHook(cache.PreUpgradeCheck); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage0_Failed
		return &system.JobError{
			ErrType:      system.ErrorPreCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("pre_upgrade_check/dynook failed: %v", err),
			IsCheckError: true,
		}
	}

	logger.Info("pre_upgrade_check/syspkginfo start")
	//加载系统软件包信息
	if err := check.LoadSysPkgInfo(SysPkgInfo); err != nil { //DONE:(DingHao)获取系统信息无返回状态码
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
		logger.Warningf("pre_upgrade_check/syspkginfo load failed: %v", err)
		return err
	}

	// check repo and load repo metadata
	for _, repoInfo := range ThisCacheInfo.UpdateMetaInfo.RepoBackend {
		if err := repoInfo.LoaderPackageInfo(ThisCacheInfo); err != nil {
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
			return &system.JobError{
				ErrType:      system.ErrorMetaInfoFile,
				ErrDetail:    fmt.Sprintf("pre_upgrade_check/repo load failed: %v", err),
				IsCheckError: true,
			}
		}
	}

	logger.Info("pre_upgrade_check/block start")

	//检查apt和dpkg安装状态，阻塞
	if err := check.CheckAPTAndDPKGState(); err != nil {
		ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
		logger.Errorf("pre_upgrade_check/tool: check apt/dpkg failed:%v", err)
		return err
	}

	ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_OK

	logger.Info("pre_upgrade_check/nonblock start")

	if err := check.LoadSysPkgInfo(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
		//TODO:(DingHao)获取系统信息无返回状态码
		logger.Warningf("pre_upgrade_check/nonblock load system package info failed:%v", err)
	}

	//检查DPKG是否为公司版本
	if err := check.CheckDPKGVersionSupport(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
		logger.Warningf("pre_upgrade_check/nonblock check dpkg version failed:%v", err)
	}

	ThisCacheInfo.InternalState.IsPreCheck = cache.P_OK
	logger.Info("pre_upgrade_check finish")
	return nil
}

func midUpgradeCheck() error {
	logger.Debug("mid_upgrade_check start")

	ThisCacheInfo.InternalState.IsMidCheck = cache.P_Run

	// 阻塞项检查
	logger.Debug("mid_upgrade_check/block start")

	//检查apt和dpkg安装状态，阻塞
	if err := check.CheckAPTAndDPKGState(); err != nil {
		ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("mid_upgrade_check/block check apt/dpkg failed:%v", err)
		return err
	}

	ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_OK

	//检查是否存在依赖错误，阻塞
	if err := check.CheckPkgDependency(); err != nil {
		ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("mid_upgrade_check/block check package depends failed:%v", err)
		return err
	}

	ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_OK

	//检查系统盘剩余可用空间是否不小于2M，阻塞
	if err := check.CheckRootDiskFreeSpace(2 * 1024); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("mid_upgrade_check/block: check root disk free space failed:%v", err)
		return err
	}

	// 检查系统盘剩余可用空间是不小于50M, 非阻塞
	if err := check.CheckRootDiskFreeSpace(50 * 1024); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
		logger.Warningf("mid_upgrade_check/nonblock check root disk free space failed:%v", err)
	}

	// 动态hook脚本检查，阻塞
	if err := check.CheckDynHook(cache.MidUpgradeCheck); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage2_Failed
		logger.Errorf("mid_upgrade_check/dynook failed:%v", err)
		return &system.JobError{
			ErrType:      system.ErrorMidCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("mid_upgrade_check/dynook failed:%v", err),
			IsCheckError: true,
		}
	}
	ThisCacheInfo.InternalState.IsMidCheck = cache.P_OK
	return nil
}

func updatePostCheckStage(state cache.PState) {
	if PostCheckStage1 {
		ThisCacheInfo.InternalState.IsPostCheckStage1 = state
	} else {
		ThisCacheInfo.InternalState.IsPostCheckStage2 = state
	}
}

func postUpgradeCheck() error {
	stage := check.Stage2
	if PostCheckStage1 {
		stage = check.Stage1
	}

	logger.Debugf("post_upgrade_check %s start", stage)

	updatePostCheckStage(cache.P_Run)

	return postCheckWithStage(stage)
}

func postCheckWithStage(stage string) error {
	//阻塞项检查

	// 检查重要服务是否存在：检查display-manager.service服务是否存在，阻塞
	if err := check.CheckImportantService(stage); err != nil {
		updatePostCheckStage(cache.P_Stage0_Failed)
		logger.Errorf("post_upgrade_check/block check important service failed:%v", err)
		return err
	}

	// 动态hook脚本检查，阻塞
	if stage == check.Stage2 {
		if err := check.CheckDynHook(cache.PostUpgradeCheck); err != nil {
			updatePostCheckStage(cache.P_Stage2_Failed)
			logger.Errorf("post_upgrade_check/dynhook failed:%v", err)
			return &system.JobError{
				ErrType:      system.ErrorPostCheckScriptsFailed,
				ErrDetail:    fmt.Sprintf("post_upgrade_check/dynhook failed:%v", err),
				IsCheckError: true,
			}
		}
	}
	updatePostCheckStage(cache.P_OK)
	logger.Debugf("post_upgrade_check %s finish", stage)
	return nil
}
