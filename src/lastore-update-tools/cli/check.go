// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
)

var logger = log.NewLogger("lastore/update-tools")

var (
	CheckWithEmulation bool
	CheckWithSucceed   bool
	PostCheckStage1    bool
)

var SysPkgInfo map[string]*cache.AppTinyInfo

func beforeCheck() error {
	err := initCheckEnv()
	if err != nil {
		logger.Debugf("check environment initialization failed: %v", err)
		return err
	}

	logger.Debug("verifying update metadata")
	if err := check.CheckVerifyCacheInfo(&ThisCacheInfo); err != nil {
		ThisCacheInfo.InternalState.IsMetaInfoFormatCheck = cache.P_Error
		logger.Errorf("check meta info failed: %+v", err)
		return &Error{
			Code: ecode.CHK_INVALID_INPUT,
			Ext:  ecode.CHK_METAINFO_FILE_ERROR,
			Msg:  fmt.Sprintf("check meta info failed: %v", err),
		}
	}
	ThisCacheInfo.InternalState.IsMetaInfoFormatCheck = cache.P_OK
	return nil
}

// executeCheck wraps the common pre/post logic for all check functions
func executeCheck(checkFunc func()) ecode.RetMsg {
	if checkFunc == nil {
		logger.Debug("check function is nil")
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_ERROR, ecode.CHK_PROGRAM_ERROR, "check function is nil")
		return CheckRetMsg
	}

	err := beforeCheck()
	if err != nil {
		logger.Debugf("check initialization failed: %v", err)
		if e, ok := err.(*Error); ok {
			CheckRetMsg.SetErrorExtMsg(e.Code, e.Ext, e.Msg)
			return CheckRetMsg
		} else {
			logger.Debugf("unexpected error during check initialization: %v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_PROGRAM_ERROR, ecode.CHK_PROGRAM_ERROR, fmt.Sprintf("check initialization failed: %v", err))
			return CheckRetMsg
		}
	}

	checkFunc()
	afterCheck()
	return CheckRetMsg
}

// PreCheck do pre-check
func PreCheck() ecode.RetMsg {
	return executeCheck(preCheck)
}

// MidCheck do mid-check
func MidCheck() ecode.RetMsg {
	return executeCheck(midCheck)
}

// PostCheck do post-check
func PostCheck() ecode.RetMsg {
	return executeCheck(postCheck)
}

func preCheck() {
	logger.Debugf("precheck start")
	CheckRetMsg.PushExtMsg("precheck start")

	// 动态检查 DONE:(DingHao)待修改返回值结构体
	// 动态hook 脚本检查，阻塞
	ThisCacheInfo.InternalState.IsPreCheck = cache.P_Run
	CheckRetMsg.PushExtMsg("precheck/dynhook start")

	if extCode, err := check.CheckDynHook(&ThisCacheInfo, cache.PreUpdate); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage0_Failed
		logger.Errorf("precheck/dynhook failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, extCode, fmt.Sprintf("precheck/dynhook failed: %v", err))
		return
	}

	//加载系统软件包信息
	if extCode, err := check.LoadSysPkgInfo(SysPkgInfo); err != nil { //DONE:(DingHao)获取系统信息无返回状态码
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
		logger.Warningf("precheck/syspkginfo load failed: %v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("precheck/syspkginfo load failed: %v", err))
		return
	}

	check.AdjustPkgArchWithName(&ThisCacheInfo)
	// check repo and load repo metadata
	for _, repoInfo := range ThisCacheInfo.UpdateMetaInfo.RepoBackend {
		if err := repoInfo.LoaderPackageInfo(&ThisCacheInfo); err != nil {
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/metainfo load failed:%s", err))
			return
		}
	}
	InstalledSizeSum := 0
	DebSizeSum := 0
	for idx, pkgInfo := range ThisCacheInfo.UpdateMetaInfo.PkgList {
		if pkgInfo.InstalledSize <= 0 {
			if pkgInfo.DebSize >= 0 {
				ThisCacheInfo.UpdateMetaInfo.PkgList[idx].InstalledSize = pkgInfo.DebSize / 1024
			} else {
				ThisCacheInfo.UpdateMetaInfo.PkgList[idx].InstalledSize = 0
				ThisCacheInfo.UpdateMetaInfo.PkgList[idx].DebSize = 0
			}
			pkgInfo.DebSize = ThisCacheInfo.UpdateMetaInfo.PkgList[idx].DebSize
			pkgInfo.InstalledSize = ThisCacheInfo.UpdateMetaInfo.PkgList[idx].InstalledSize
		}
		if err := pkgInfo.Verify(); err != nil {
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
			logger.Debugf("precheck/pkginfo check pkginfo info failed ,pkglist: %v,%v", pkgInfo, err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/pkginfo check pkginfo info failed ,pkglist: %v,%v", pkgInfo, err))
			return
		}
		if sPkgInfo, ok := SysPkgInfo[pkgInfo.Name]; ok {
			if sPkgInfo.State == cache.HoldInstalled {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
				logger.Warningf("precheck/pkginfo %s with hold system", pkgInfo.Name)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/pkginfo %s with hold system", pkgInfo.Name))
				return
			}
		}
		InstalledSizeSum += pkgInfo.InstalledSize
		DebSizeSum += pkgInfo.DebSize
	}

	// logger.Debugf("sum size:%d,deb:%d", InstalledSizeSum, DebSizeSum)

	CheckRetMsg.PushExtMsg("precheck/block start")
	//检查apt和dpkg安装状态，阻塞
	if exit, err := check.CheckAPTAndDPKGState(); err != nil {
		ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
		logger.Errorf("precheck/tool: check apt/dpkg failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exit, fmt.Sprintf("precheck/tool: check apt/dpkg failed:%v", err))
		return
	}

	ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_OK

	//检查核心包列表中软件包是否存在依赖错误，阻塞
	CheckRetMsg.PushExtMsg("precheck/block check package depends")
	if extCode, err := check.CheckPkgDependency(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsDependsPreCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
		logger.Errorf("precheck/block check package depends failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("precheck/block check package depends failed:%v", err))
		return
	}

	ThisCacheInfo.InternalState.IsDependsPreCheck = cache.P_OK

	// 检查purge list中的包是否存在，阻塞
	if len(ThisCacheInfo.UpdateMetaInfo.PurgeList) > 0 {
		if err := check.CheckPurgeList(&ThisCacheInfo, SysPkgInfo); err != nil {
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
			logger.Errorf("precheck/block check purge list failed: %v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/block check purge list failed: %v", err))
			return
		}
	}

	//检查系统盘剩余可用空间能否满足本次更新，阻塞
	// TODO:(DingHao)参数换成真实安装包空间大小
	if extCode, err := check.CheckRootDiskFreeSpace(uint64(InstalledSizeSum)); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
		logger.Errorf("precheck/block check root disk free space failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("precheck/block check root disk free space failed:%v", err))
		return
	}

	CheckRetMsg.PushExtMsg("precheck/nonblock start")
	if extCode, err := check.LoadSysPkgInfo(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
		//TODO:(DingHao)获取系统信息无返回状态码
		logger.Warningf("precheck/nonblock load system package info failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("precheck/nonblock load system package info failed:%v", err))
	}

	//检查系统可选包安装状态
	for _, pkgInfo := range ThisCacheInfo.UpdateMetaInfo.OptionList {
		if extCode, err := check.CheckDebListInstallState(SysPkgInfo, &pkgInfo, "precheck", "optionlist"); err != nil {
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
			logger.Warningf("precheck/nonblock check optionlist failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("precheck/nonblock check optionlist failed:%v", err))
		}
	}

	//检查DPKG是否为公司版本
	if extCode, err := check.CheckDPKGVersionSupport(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
		logger.Warningf("precheck/nonblock check dpkg version failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("precheck/nonblock check dpkg version failed:%v", err))
	}

	ThisCacheInfo.InternalState.IsPreCheck = cache.P_OK
	CheckRetMsg.PushExtMsg("precheck finish")
	CheckRetMsg.SetError(0, 0)
}

func midCheck() {
	logger.Debug("midcheck start")
	CheckRetMsg.PushExtMsg("midcheck start")

	ThisCacheInfo.InternalState.IsMidCheck = cache.P_Run

	// 阻塞项检查
	CheckRetMsg.PushExtMsg("midcheck/block start")

	//检查apt和dpkg安装状态，阻塞
	if extCode, err := check.CheckAPTAndDPKGState(); err != nil {
		ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block check apt/dpkg failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block check apt/dpkg failed:%v", err))
		return
	}

	ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_OK

	//获取当前系统信息，阻塞
	if extCode, err := check.LoadSysPkgInfo(SysPkgInfo); err != nil { //TODO:(DingHao)获取系统信息无返回状态码
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block load system info failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block load system info failed:%v", err))
		return
	}

	//检查系统核心包安装状态，阻塞
	for _, pkgInfo := range ThisCacheInfo.UpdateMetaInfo.SysCoreList {
		if extCode, err := check.CheckDebListInstallState(SysPkgInfo, &pkgInfo, "midcheck", "corelist"); err != nil {
			ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
			logger.Errorf("midcheck/block corelist check failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block corelist check failed:%v", err))
			return
		}
	}

	//检查是否存在依赖错误，阻塞
	if extCode, err := check.CheckPkgDependency(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_Error
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block check package depends failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block check package depends failed:%v", err))
		return
	}

	ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_OK

	//检查系统盘剩余可用空间是否不小于2M，阻塞
	if extCode, err := check.CheckRootDiskFreeSpace(2 * 1024); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
		logger.Errorf("midcheck/block: check root disk free space failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block check root disk free space failed:%v", err))
		return
	}

	//检查系统核心文件是否存在，阻塞
	if len(CoreProtectPath) != 0 {
		if extCode, err := check.CheckCoreFileExist(CoreProtectPath); err != nil { //DONE:(DingHao)待指定系统核心文件参数
			ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
			logger.Errorf("midcheck/block check core file exist failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block: check core file exist failed:%v", err))
			return
		}
	}

	//检查pkglist中包的安装状态，阻塞
	for _, pkgInfo := range ThisCacheInfo.UpdateMetaInfo.PkgList {
		if extCode, err := check.CheckDebListInstallState(SysPkgInfo, &pkgInfo, "midcheck", "pkglist"); err != nil {
			ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
			logger.Errorf("midcheck/block check pkglist install state failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("midcheck/block check package depends failed:%v", err))
			return
		}
	}

	// 非阻塞项检查
	// 检查系统盘剩余可用空间是不小于50M
	if extCode, err := check.CheckRootDiskFreeSpace(50 * 1024); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
		logger.Warningf("midcheck/nonblock check root disk free space failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("midcheck/nonblock check root disk free space failed:%v", err))
	}

	//获取系统当前信息
	if extCode, err := check.LoadSysPkgInfo(SysPkgInfo); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
		logger.Warningf("midcheck/nonblock load system info failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("midcheck/nonblock load system info failed:%v", err))
	}

	//检查系统可选包安装状态
	for _, pkgInfo := range ThisCacheInfo.UpdateMetaInfo.OptionList {
		if extCode, err := check.CheckDebListInstallState(SysPkgInfo, &pkgInfo, "midcheck", "optionlist"); err != nil {
			ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
			logger.Warningf("midcheck/nonblock check optionlist install state failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("midcheck/nonblock check optionlist install state failed:%v", err))
		}
	}

	// 动态hook脚本检查，阻塞
	if extCode, err := check.CheckDynHook(&ThisCacheInfo, cache.UpdateCheck); err != nil {
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage2_Failed
		logger.Errorf("midcheck/dynook failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, extCode, fmt.Sprintf("midcheck/dynook failed:%v", err))
		return
	}
	ThisCacheInfo.InternalState.IsMidCheck = cache.P_OK
	CheckRetMsg.SetError(0, 0)
}

func updatePostCheckStage(state cache.PState) {
	if PostCheckStage1 {
		ThisCacheInfo.InternalState.IsPostCheckStage1 = state
	} else {
		ThisCacheInfo.InternalState.IsPostCheckStage2 = state
	}
}

func postCheck() {
	stage := check.Stage2
	if PostCheckStage1 {
		stage = check.Stage1
	}

	logger.Debugf("postcheck check-with-succeed:%v %s start", CheckWithSucceed, stage)
	CheckRetMsg.PushExtMsg(fmt.Sprintf("postcheck check-with-succeed:%v %s start", CheckWithSucceed, stage))

	updatePostCheckStage(cache.P_Run)

	if CheckWithSucceed {
		postCheckWithSucceed(stage)
	} else {
		postCheckWithFailed(stage)
	}
}

func postCheckWithSucceed(stage string) {
	//阻塞项检查

	// 检查重要进程是否存在：检查/usr/sbin/lightdm进程是否存在，阻塞
	if extCode, err := check.CheckImportantProgress(stage); err != nil {
		updatePostCheckStage(cache.P_Stage0_Failed)
		logger.Errorf("postcheck/block check important progress failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, extCode, fmt.Sprintf("postcheck/block check important progress failed:%v", err))
		return
	}

	//非阻塞项检查
	if extCode, err := check.ArchiveLogAndCache(ThisCacheInfo.UUID); err != nil {
		updatePostCheckStage(cache.P_Stage1_Failed)
		logger.Warningf("postcheck/nonblock archive log and cache failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("postcheck/nonblock archive log and cache failed:%v", err))
	}

	if extCode, err := check.DeleteUpgradeCacheFile(ThisCacheInfo.UUID); err != nil {
		updatePostCheckStage(cache.P_Stage1_Failed)
		logger.Warningf("postcheck/nonblock delete upgrade cache file failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("postcheck/nonblock delete upgrade cache file failed:%v", err))
	}

	// 动态hook脚本检查，阻塞
	if stage == check.Stage2 {
		if extCode, err := check.CheckDynHook(&ThisCacheInfo, cache.PostCheck); err != nil {
			updatePostCheckStage(cache.P_Stage2_Failed)
			logger.Errorf("postcheck/dynhook failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, extCode, fmt.Sprintf("postcheck/dynhook failed:%v", err))
			return
		}
	}

	CheckRetMsg.SetError(0, 0)
	updatePostCheckStage(cache.P_OK)
}

func postCheckWithFailed(stage string) {
	if extCode, err := check.ArchiveLogAndCache(ThisCacheInfo.UUID); err != nil {
		updatePostCheckStage(cache.P_Stage0_Failed)
		logger.Warningf("postcheck/nonblock archive log and cache failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("postcheck/nonblock archive log and cache failed:%v", err))
	}

	if extCode, err := check.DeleteUpgradeCacheFile(ThisCacheInfo.UUID); err != nil {
		updatePostCheckStage(cache.P_Stage0_Failed)
		logger.Warningf("postcheck/nonblock delete upgrade cache file failed:%v", err)
		CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, extCode, fmt.Sprintf("postcheck/nonblock delete upgrade cache file failed:%v", err))
	}

	// 动态hook脚本检查，阻塞
	if stage == check.Stage2 {
		if extCode, err := check.CheckDynHook(&ThisCacheInfo, cache.PostCheckFailed); err != nil {
			updatePostCheckStage(cache.P_Stage2_Failed)
			logger.Errorf("postcheck/dynhook failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, extCode, fmt.Sprintf("postcheck/dynhook failed:%v", err))
			return
		}
	}

	CheckRetMsg.SetError(0, 0)
	updatePostCheckStage(cache.P_OK)
}
