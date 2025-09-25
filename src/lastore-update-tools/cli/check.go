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
			ErrDetail:    fmt.Sprintf("check function is nil"),
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

// PreCheck do pre-check
func PreCheck() error {
	return executeCheck(preCheck)
}

// MidCheck do mid-check
func MidCheck() error {
	return executeCheck(midCheck)
}

// PostCheck do post-check
func PostCheck() error {
	return executeCheck(postCheck)
}

func preCheck() error {
	logger.Info("precheck start")

	// 动态检查 DONE:(DingHao)待修改返回值结构体
	// 动态hook 脚本检查，阻塞
	ThisCacheInfo.InternalState.IsPreCheck = cache.P_Run

	logger.Info("precheck/dynhook start")

	if err := check.CheckDynHook(ThisCacheInfo, cache.PreUpdate); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage0_Failed
		return &system.JobError{
			ErrType:      system.ErrorPreCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("precheck/dynook failed: %v", err),
			IsCheckError: true,
		}
	}

	logger.Info("precheck/syspkginfo start")
	//加载系统软件包信息
	if err := check.LoadSysPkgInfo(SysPkgInfo); err != nil { //DONE:(DingHao)获取系统信息无返回状态码
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
		logger.Warningf("precheck/syspkginfo load failed: %v", err)
		return err
	}

	// check repo and load repo metadata
	for _, repoInfo := range ThisCacheInfo.UpdateMetaInfo.RepoBackend {
		if err := repoInfo.LoaderPackageInfo(ThisCacheInfo); err != nil {
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
			return &system.JobError{
				ErrType:      system.ErrorMetaInfoFile,
				ErrDetail:    fmt.Sprintf("precheck/repo load failed: %v", err),
				IsCheckError: true,
			}
		}
	}

	logger.Info("precheck/block start")

	//检查apt和dpkg安装状态，阻塞
	if err := check.CheckAPTAndDPKGState(); err != nil {
		ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
		logger.Errorf("precheck/tool: check apt/dpkg failed:%v", err)
		return err
	}

	ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_OK

	logger.Info("precheck/nonblock start")

	if err := check.LoadSysPkgInfo(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
		//TODO:(DingHao)获取系统信息无返回状态码
		logger.Warningf("precheck/nonblock load system package info failed:%v", err)
	}

	//检查DPKG是否为公司版本
	if err := check.CheckDPKGVersionSupport(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
		logger.Warningf("precheck/nonblock check dpkg version failed:%v", err)
	}

	ThisCacheInfo.InternalState.IsPreCheck = cache.P_OK
	logger.Info("precheck finish")
	return nil
}

func midCheck() error {
	logger.Debug("midcheck start")

	ThisCacheInfo.InternalState.IsMidCheck = cache.P_Run

	// 阻塞项检查
	logger.Debug("midcheck/block start")

	//检查apt和dpkg安装状态，阻塞
	if err := check.CheckAPTAndDPKGState(); err != nil {
		ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block check apt/dpkg failed:%v", err)
		return err
	}

	ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_OK

	//检查是否存在依赖错误，阻塞
	if err := check.CheckPkgDependency(); err != nil {
		ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block check package depends failed:%v", err)
		return err
	}

	ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_OK

	//检查系统盘剩余可用空间是否不小于2M，阻塞
	if err := check.CheckRootDiskFreeSpace(2 * 1024); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block: check root disk free space failed:%v", err)
		return err
	}

	// 检查系统盘剩余可用空间是不小于50M, 非阻塞
	if err := check.CheckRootDiskFreeSpace(50 * 1024); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
		logger.Warningf("midcheck/nonblock check root disk free space failed:%v", err)
	}

	// 动态hook脚本检查，阻塞
	if err := check.CheckDynHook(ThisCacheInfo, cache.MidCheck); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage2_Failed
		logger.Errorf("midcheck/dynook failed:%v", err)
		return &system.JobError{
			ErrType:      system.ErrorMidCheckScriptsFailed,
			ErrDetail:    fmt.Sprintf("midcheck/dynook failed:%v", err),
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

func postCheck() error {
	stage := check.Stage2
	if PostCheckStage1 {
		stage = check.Stage1
	}

	logger.Debugf("postcheck %s start", stage)

	updatePostCheckStage(cache.P_Run)

	return postCheckWithStage(stage)
}

func postCheckWithStage(stage string) error {
	//阻塞项检查

	// 检查重要进程是否存在：检查/usr/sbin/lightdm进程是否存在，阻塞
	if err := check.CheckImportantProcess(stage); err != nil {
		updatePostCheckStage(cache.P_Stage0_Failed)
		logger.Errorf("postcheck/block check important progress failed:%v", err)
		return err
	}

	// 动态hook脚本检查，阻塞
	if stage == check.Stage2 {
		if err := check.CheckDynHook(ThisCacheInfo, cache.PostCheck); err != nil {
			updatePostCheckStage(cache.P_Stage2_Failed)
			logger.Errorf("postcheck/dynhook failed:%v", err)
			return &system.JobError{
				ErrType:      system.ErrorPostCheckScriptsFailed,
				ErrDetail:    fmt.Sprintf("postcheck/dynhook failed:%v", err),
				IsCheckError: true,
			}
		}
	}
	updatePostCheckStage(cache.P_OK)
	logger.Debugf("postcheck %s finish", stage)
	return nil
}
