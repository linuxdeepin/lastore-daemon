// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/spf13/cobra"
)

var (
	IgnoreCheckWarning bool
	IgnoreCheckError   bool
	OnlyCheckCorelist  bool
	CheckWithEmulation bool
	CheckWithSucceed   bool
	CheckWithFailed    bool
	PostCheckStage1    bool
	PostCheckStage2    bool
)

var SysPkgInfo map[string]*cache.AppTinyInfo

// versionCmd represents the version command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check system update",
	Long:  `Check system update with rules that pre/mid/post-check.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debugf("check")
		log.Debugf("config:%s", ConfigCfg)

	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) { //TODO:(DingHao) return code
		rootCmd.PersistentPreRun(cmd, args)
		log.Debugf("check verify update metadata")
		if err := check.CheckVerifyCacheInfo(&ThisCacheInfo); err != nil && IgnoreMetaEmptyCheck == false {
			ThisCacheInfo.InternalState.IsMetaInfoFormatCheck = cache.P_Error
			// var metaInfoExitMsg ecode.RetMsg
			log.Errorf("check/pre-run check meta info failed: %+v", err)
			CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("check/pre-run check meta info failed: %v", err))
		}
		//log.Debugf("aa:%+v", SysPkgInfo)
		ThisCacheInfo.InternalState.IsMetaInfoFormatCheck = cache.P_OK
	},
}

var preCheckCmd = &cobra.Command{
	Use:   "precheck",
	Short: "Pre Check system update",
	Long:  `Pre Check system update with update before.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debugf("precheck start with ignore error:%v warring:%v", IgnoreCheckError, IgnoreCheckWarning)
		CheckRetMsg.PushExtMsg(fmt.Sprintf("precheck start with ignore error:%v warring:%v", IgnoreCheckError, IgnoreCheckWarning))
		//log.Debugf("config:%s", ConfigCfg)

		// 动态检查 DONE:(DingHao)待修改返回值结构体
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_Run
		CheckRetMsg.PushExtMsg("precheck/dynhook start")

		if exitcode, err := check.CheckDynHook(&ThisCacheInfo, cache.PreUpdate); err != nil {
			if !IgnoreCheckError {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage0_Failed
				log.Errorf("precheck/dynhook failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("precheck/dynhook failed: %v", err))
				return
			}
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage0_Failed
			log.Warnf("precheck/dynhook skip error: %v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("precheck/dynhook skip error: %v", err))
		}

		//加载系统软件包信息
		if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil { //DONE:(DingHao)获取系统信息无返回状态码
			ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
			log.Warnf("precheck/syspkginfo load failed: %v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/syspkginfo load failed: %v", err))
			return
		}

		check.AdjustPkgArchWithName(&ThisCacheInfo)
		// check repo and load repo metadata
		for _, repoinfo := range ThisCacheInfo.UpdateMetaInfo.RepoBackend {
			if err := repoinfo.LoaderPackageInfo(&ThisCacheInfo); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/metainfo load failed:%s", err))
				return
			}
		}
		InstalledSizeSum := 0
		DebSizeSum := 0
		for idx, pkginfo := range ThisCacheInfo.UpdateMetaInfo.PkgList {
			if pkginfo.InstalledSize <= 0 {
				if pkginfo.DebSize >= 0 {
					ThisCacheInfo.UpdateMetaInfo.PkgList[idx].InstalledSize = pkginfo.DebSize / 1024
				} else {
					ThisCacheInfo.UpdateMetaInfo.PkgList[idx].InstalledSize = 0
					ThisCacheInfo.UpdateMetaInfo.PkgList[idx].DebSize = 0
				}
				pkginfo.DebSize = ThisCacheInfo.UpdateMetaInfo.PkgList[idx].DebSize
				pkginfo.InstalledSize = ThisCacheInfo.UpdateMetaInfo.PkgList[idx].InstalledSize
			}
			if err := pkginfo.Verify(); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
				log.Debugf("precheck/pkginfo check pkginfo info failed ,pkglist: %v,%v", pkginfo, err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/pkginfo check pkginfo info failed ,pkglist: %v,%v", pkginfo, err))
				return
			}
			if spkginfo, ok := SysPkgInfo[pkginfo.Name]; ok {
				if spkginfo.State == cache.HoldInstalled {
					ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage1_Failed
					log.Warnf("precheck/pkginfo %s with hold system", pkginfo.Name)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/pkginfo %s with hold system", pkginfo.Name))
					return
				}
			}
			InstalledSizeSum += pkginfo.InstalledSize
			DebSizeSum += pkginfo.DebSize
		}

		// log.Debugf("sum size:%d,deb:%d", InstalledSizeSum, DebSizeSum)

		CheckRetMsg.PushExtMsg(fmt.Sprintf("precheck/block start with skip:%v", IgnoreCheckError))
		if IgnoreCheckError {
			//检查apt和dpkg安装状态
			if exitcode, err := check.CheckAPTAndDPKGState(); err != nil {
				ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
				log.Warnf("precheck/tool: check apt/dpkg failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/tool: check apt/dpkg failed:%v", err))
			}

			ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_OK

			//检查是否存在依赖错误
			CheckRetMsg.PushExtMsg("precheck/block check package depends")
			if exitcode, err := check.CheckPkgDependency(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsDependsPreCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
				log.Warnf("precheck/block check package depends failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/block check package depends failed:%v", err))
			}

			ThisCacheInfo.InternalState.IsDependsPreCheck = cache.P_OK

			if len(ThisCacheInfo.UpdateMetaInfo.PurgeList) > 0 {
				if err := check.CheckPurgeList(&ThisCacheInfo, SysPkgInfo); err != nil {
					ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
					log.Warnf("precheck/block check purge list failed: %v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/block check purge list failed: %v", err))
				}
			}

			//检查系统盘剩余可用空间能否满足本次更新TODO:(DingHao)参数换成真实安装包空间大小
			if exitcode, err := check.CheckRootDiskFreeSpace(uint64(InstalledSizeSum)); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
				log.Warnf("precheck/block check root disk free space failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/block check root disk free space failed:%v", err))
			}

		} else {
			//检查apt和dpkg安装状态
			if exitcode, err := check.CheckAPTAndDPKGState(); err != nil {
				ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
				log.Errorf("precheck/tool: check apt/dpkg failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/tool: check apt/dpkg failed:%v", err))
				return
			}
			// ThisCacheInfo.InternalState.IsDpkgCheck = true

			ThisCacheInfo.InternalState.IsDpkgAptPreCheck = cache.P_OK

			//检查是否存在依赖错误
			CheckRetMsg.PushExtMsg("precheck/block check package depends")
			if exitcode, err := check.CheckPkgDependency(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsDependsPreCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
				log.Errorf("precheck/block check package depends failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/block check package depends failed:%v", err))
				return
				// ecode.RenderRetMessage(&preExitMsg, exitcode, ecode.CHK_BLOCK_ERROR, true)
			}

			ThisCacheInfo.InternalState.IsDependsPreCheck = cache.P_OK

			if len(ThisCacheInfo.UpdateMetaInfo.PurgeList) > 0 {
				if err := check.CheckPurgeList(&ThisCacheInfo, SysPkgInfo); err != nil {
					ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
					log.Errorf("precheck/block check purge list failed: %v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("precheck/block check purge list failed: %v", err))
					return
				}
			}

			//检查系统盘剩余可用空间能否满足本次更新TODO:(DingHao)参数换成真实安装包空间大小
			if exitcode, err := check.CheckRootDiskFreeSpace(uint64(InstalledSizeSum)); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage2_Failed
				log.Errorf("precheck/block check root disk free space failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("precheck/block check root disk free space failed:%v", err))
				return
				// ecode.RenderRetMessage(&preExitMsg, exitcode, ecode.CHK_BLOCK_ERROR, true)
			}
		}

		CheckRetMsg.PushExtMsg(fmt.Sprintf("precheck/nonblock start skip:%v", IgnoreCheckWarning))
		if IgnoreCheckWarning {
			if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
				//TODO:(DingHao)获取系统信息无返回状态码
				log.Warnf("precheck/nonblock load system package info failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock load system package info failed:%v", err))
			}
			// //检查pkglist中包的安装状态
			// for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.Pkglist {
			// 	if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, pkginfo, "precheck", "pkglist"); err != nil {
			// 		log.Warnf("precheck: CheckDebListInstallState failed:%v", err)
			// 		CheckRetMsg.SetError(ecode.CHK_NONBLOCK_ERROR, exitcode)
			// 		// ecode.RenderRetMessage(&preExitMsg, exitcode, ecode.CHK_NONBLOCK_ERROR, false)
			// 	}
			// }
			//检查系统核心包安装状态
			for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.BaseLine {
				if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "precheck", "corelist"); err != nil {
					ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
					log.Warnf("precheck/nonblock check core list failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock check baseline failed:%v", err))
				}
			}
			//检查系统可选包安装状态
			if !OnlyCheckCorelist {
				for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.OptionList {
					if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "precheck", "optionlist"); err != nil {
						ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
						log.Warnf("precheck/nonblock check optionlist failed:%v", err)
						CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock check optionlist failed:%v", err))
					}
				}
			}
			//检查DPKG是否为公司版本
			if exitcode, err := check.CheckDPKGVersionSupport(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
				log.Warnf("precheck/nonblock check dpkg version failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock check dpkg version failed:%v", err))
			}

		} else {
			if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil {
				//TODO:(DingHao)获取系统信息无返回状态码
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
				log.Errorf("precheck/nonblock load system package info failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock load system package info failed:%v", err))
				return
			}
			//检查pkglist中包的安装状态
			// for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.Pkglist {
			// 	if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, pkginfo, "precheck", "pkglist"); err != nil {
			// 		log.Errorf("precheck: CheckDebListInstallState failed:%v", err)
			// 		CheckRetMsg.SetError(ecode.CHK_NONBLOCK_ERROR, exitcode)
			// 		return
			// 		// ecode.RenderRetMessage(&preExitMsg, exitcode, ecode.CHK_NONBLOCK_ERROR, true)
			// 	}
			// }
			//检查系统核心包安装状态
			for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.BaseLine {
				if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "precheck", "corelist"); err != nil {
					ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
					log.Errorf("precheck/nonblock check core list failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock check baseline failed:%v", err))
					return
					// ecode.RenderRetMessage(&preExitMsg, exitcode, ecode.CHK_NONBLOCK_ERROR, true)
				}
			}
			//检查系统可选包安装状态
			if !OnlyCheckCorelist {
				for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.OptionList {
					if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "precheck", "optionlist"); err != nil {
						ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
						log.Errorf("precheck/nonblock check optionlist failed:%v", err)
						CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock check optionlist failed:%v", err))
						return
					}
				}
			}
			//检查DPKG是否为公司版本
			if exitcode, err := check.CheckDPKGVersionSupport(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsPreCheck = cache.P_Stage3_Failed
				log.Errorf("precheck/nonblock check dpkg version failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("precheck/nonblock check dpkg version failed:%v", err))
				return
			}

		}
		ThisCacheInfo.InternalState.IsPreCheck = cache.P_OK
		CheckRetMsg.PushExtMsg("precheck finish")
		//return
	},
}

var midCheckCmd = &cobra.Command{
	Use:   "midcheck",
	Short: "Mid Check system update",
	Long:  `Mid Check system update with update after.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debugf("midcheck start with ignore error:%v warring:%v", IgnoreCheckError, IgnoreCheckWarning)
		CheckRetMsg.PushExtMsg(fmt.Sprintf("midcheck start with ignore error:%v warring:%v", IgnoreCheckError, IgnoreCheckWarning))

		ThisCacheInfo.InternalState.IsMidCheck = cache.P_Run

		// 阻塞项检查，判断是否存在IgnoreCheckError
		CheckRetMsg.PushExtMsg(fmt.Sprintf("midcheck/block start with skip:%v", IgnoreCheckError))
		if IgnoreCheckError {
			log.Infof("ignore error when midcheck.")
			//检查apt和dpkg安装状态
			if exitcode, err := check.CheckAPTAndDPKGState(); err != nil {
				ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Warnf("midcheck/block check apt/dpkg failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check apt/dpkg failed:%v", err))
			}

			ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_OK

			//获取当前系统信息
			if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil { //DONE:(DingHao)获取系统信息无返回状态码
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Warnf("midcheck/block load system info failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block load system info failed:%v", err))
			}

			//检查系统核心包安装状态
			for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.SysCoreList {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "midcheck", "corelist"); err != nil {
					log.Warnf("midcheck/block corelist check failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block corelist check failed:%v", err))
				}
			}

			//检查是否存在依赖错误 DONE:(DingHao)实现有问题，需修改
			if exitcode, err := check.CheckPkgDependency(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Warnf("midcheck/block check package depends failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check package depends failed:%v", err))
			}

			ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_OK

			//检查系统盘剩余可用空间是否不小于2M
			if exitcode, err := check.CheckRootDiskFreeSpace(2 * 1024); err != nil {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Warnf("midcheck/block check root disk free space failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check root disk free space failed:%v", err))
			}

			//检查系统核心文件是否存在
			if len(CoreProtectPath) != 0 {
				if exitcode, err := check.CheckCoreFileExist(CoreProtectPath); err != nil { //DONE:(DingHao)待指定系统核心文件参数
					ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
					log.Warnf("midcheck/block check core file exist failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check core file exist failed:%v", err))
				}
			}

			//检查pkglist中包的安装状态
			for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.PkgList {
				if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "midcheck", "pkglist"); err != nil {
					ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
					log.Warnf("midcheck/block check pkglist install state failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check package depends failed:%v", err))
				}
			}

		} else {

			//检查apt和dpkg安装状态
			if exitcode, err := check.CheckAPTAndDPKGState(); err != nil {
				ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Errorf("midcheck/block check apt/dpkg failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check apt/dpkg failed:%v", err))
				return
			}

			ThisCacheInfo.InternalState.IsDpkgAptMidCheck = cache.P_OK

			//获取当前系统信息
			if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil { //TODO:(DingHao)获取系统信息无返回状态码
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Errorf("midcheck/block load system info failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block load system info failed:%v", err))
				return
			}

			//检查系统核心包安装状态
			for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.SysCoreList {
				if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "midcheck", "corelist"); err != nil {
					ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
					log.Errorf("midcheck/block corelist check failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block corelist check failed:%v", err))
					return
				}
			}

			//检查是否存在依赖错误
			if exitcode, err := check.CheckPkgDependency(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_Error
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Errorf("midcheck/block check package depends failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check package depends failed:%v", err))
				return
			}

			ThisCacheInfo.InternalState.IsDependsMidCheck = cache.P_OK

			//检查系统盘剩余可用空间是否不小于2M
			if exitcode, err := check.CheckRootDiskFreeSpace(2 * 1024); err != nil {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
				log.Errorf("midcheck/block: check root disk free space failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check root disk free space failed:%v", err))
				return
			}

			//检查系统核心文件是否存在
			if len(CoreProtectPath) != 0 {
				if exitcode, err := check.CheckCoreFileExist(CoreProtectPath); err != nil { //DONE:(DingHao)待指定系统核心文件参数
					ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
					log.Errorf("midcheck/block check core file exist failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block: check core file exist failed:%v", err))
					return
				}
			}

			//检查pkglist中包的安装状态
			for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.PkgList {
				if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "midcheck", "pkglist"); err != nil {
					ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage0_Failed
					log.Errorf("midcheck/block check pkglist install state failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/block check package depends failed:%v", err))
					return
				}
			}

			//
		}

		// TODO:(DingHao)非阻塞项检查,待理清IgnoreCheckError逻辑
		if IgnoreCheckWarning {
			//检查系统盘剩余可用空间是不小于50M
			if exitcode, err := check.CheckRootDiskFreeSpace(50 * 1024); err != nil {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
				log.Warnf("midcheck/nonblock check root disk free space failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/nonblock check root disk free space failed:%v", err))
			}

			//获取系统当前信息
			if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
				log.Warnf("midcheck/nonblock load system info failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/nonblock load system info failed:%v", err))
			}

			//检查系统可选包安装状态
			if !OnlyCheckCorelist {
				for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.OptionList {
					if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "midcheck", "optionlist"); err != nil {
						ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
						log.Warnf("midcheck/nonblock check optionlist install state failed:%v", err)
						CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/nonblock check optionlist install state failed:%v", err))
					}
				}
			}

		} else {

			//检查系统盘剩余可用空间是不小于50M
			if exitcode, err := check.CheckRootDiskFreeSpace(50 * 1024); err != nil {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
				log.Errorf("midcheck/nonblock check root disk free space failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/nonblock check root disk free space failed:%v", err))
				return
			}

			//获取系统当前信息
			if exitcode, err := check.PreCheckLoadSysPkgInfo(SysPkgInfo); err != nil {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
				log.Errorf("midcheck/nonblock load system info failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/nonblock load system info failed:%v", err))
				return
			}

			//检查系统可选包安装状态
			if !OnlyCheckCorelist {
				for _, pkginfo := range ThisCacheInfo.UpdateMetaInfo.OptionList {
					if exitcode, err := check.CheckDebListInstallState(SysPkgInfo, &pkginfo, "midcheck", "optionlist"); err != nil {
						ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage1_Failed
						log.Errorf("midcheck/nonblock check optionlist install state failed:%v", err)
						CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("midcheck/nonblock check optionlist install state failed:%v", err))
						return
					}
				}
			}

		}
		// 动态检查 DONE:(DingHao)待修改返回值结构体
		if exitcode, err := check.CheckDynHook(&ThisCacheInfo, cache.UpdateCheck); err != nil {
			if !IgnoreCheckError {
				ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage2_Failed
				log.Errorf("midcheck/dynook failed:%v", err)
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("midcheck/dynook failed:%v", err))
				return
			}
			ThisCacheInfo.InternalState.IsMidCheck = cache.P_Stage2_Failed
			log.Warnf("midcheck/dynook failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("midcheck/dynook failed:%v", err))
			return
		}
		ThisCacheInfo.InternalState.IsMidCheck = cache.P_OK
		CheckRetMsg.SetError(0, 0)
	},
}

var postCheckCmd = &cobra.Command{
	Use:   "postcheck",
	Short: "Post Check system update",
	Long:  `Post Check system update with update complete.`,
	Run: func(cmd *cobra.Command, args []string) {

		stage := "stage2"
		if PostCheckStage1 {
			stage = "stage1"
		}

		updatePostCheckStage := func(state cache.PState) {
			switch {
			case PostCheckStage1:
				ThisCacheInfo.InternalState.IsPostCheckStage1 = state
			case PostCheckStage2:
				ThisCacheInfo.InternalState.IsPostCheckStage2 = state
			default:
				ThisCacheInfo.InternalState.IsPostCheckStage2 = state
			}
		}

		if CheckWithFailed {
			CheckWithSucceed = false
		}

		log.Debugf("postcheck check-with-succeed:%v %s start with ignore error:%v warning:%v", CheckWithSucceed, stage, IgnoreCheckError, IgnoreCheckWarning)
		CheckRetMsg.PushExtMsg(fmt.Sprintf("postcheck check-with-succeed:%v %s start with ignore error:%v warning:%v", CheckWithSucceed, stage, IgnoreCheckError, IgnoreCheckWarning))

		updatePostCheckStage(cache.P_Run)

		if CheckWithSucceed {
			//阻塞项检查
			if IgnoreCheckError {
				if exitcode, err := check.CheckImportantProgress(stage); err != nil {
					updatePostCheckStage(cache.P_Stage0_Failed)
					log.Warnf("postcheck/block check important progress failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/block check important progress failed:%v", err))
				}
			} else {
				if exitcode, err := check.CheckImportantProgress(stage); err != nil {
					updatePostCheckStage(cache.P_Stage0_Failed)
					log.Errorf("postcheck/block check important progress failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_BLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/block check important progress failed:%v", err))
					return
				}
			}
			//非阻塞项检查
			if IgnoreCheckWarning {
				if exitcode, err := check.ArchiveLogAndCache(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage1_Failed)
					log.Warnf("postcheck/nonblock archive log and cache failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock archive log and cache failed:%v", err))
				}

				if exitcode, err := check.DeleteUpgradeCacheFile(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage1_Failed)
					log.Warnf("postcheck/nonblock delete upgrade cache file failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock delete upgrade cache file failed:%v", err))
				}

			} else {
				if exitcode, err := check.ArchiveLogAndCache(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage1_Failed)
					log.Errorf("postcheck/nonblock archive log and cache failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock archive log and cache failed:%v", err))
					return
				}

				if exitcode, err := check.DeleteUpgradeCacheFile(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage1_Failed)
					log.Errorf("postcheck/nonblock delete upgrade cache file failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock delete upgrade cache file failed:%v", err))
					return
				}
			}
			// dyn check hook
			//check.CheckDynHook(&ThisCacheInfo, cache.PostCheck) TODO:(DingHao)postcheck暂无动态检查规则
			if stage == "stage2" {
				if exitcode, err := check.CheckDynHook(&ThisCacheInfo, cache.PostCheck); err != nil {
					if !IgnoreCheckError {
						updatePostCheckStage(cache.P_Stage2_Failed)
						log.Errorf("postcheck/dynhook failed:%v", err)
						CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("postcheck/dynhook failed:%v", err))
						return
					}
					updatePostCheckStage(cache.P_Stage2_Failed)
					log.Warnf("postcheck/dynhook failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("postcheck/dynhook failed:%v", err))
				}
			}

			CheckRetMsg.SetError(0, 0)
			updatePostCheckStage(cache.P_OK)
			return
		}

		if CheckWithFailed {
			if IgnoreCheckWarning {
				if exitcode, err := check.ArchiveLogAndCache(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage0_Failed)
					log.Warnf("postcheck/nonblock archive log and cache failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock archive log and cache failed:%v", err))
				}

				if exitcode, err := check.DeleteUpgradeCacheFile(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage0_Failed)
					log.Warnf("postcheck/nonblock delete upgrade cache file failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock delete upgrade cache file failed:%v", err))
				}

			} else {
				if exitcode, err := check.ArchiveLogAndCache(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage0_Failed)
					log.Errorf("postcheck/nonblock archive log and cache failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock archive log and cache failed:%v", err))
					return
				}

				if exitcode, err := check.DeleteUpgradeCacheFile(ThisCacheInfo.UUID); err != nil { //DONE:(DingHao)替换成真正的uuid
					updatePostCheckStage(cache.P_Stage0_Failed)
					log.Errorf("postcheck/nonblock delete upgrade cache file failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_NONBLOCK_ERROR, exitcode, fmt.Sprintf("postcheck/nonblock delete upgrade cache file failed:%v", err))
					return
				}
			}
			// dyn check hook
			//check.CheckDynHook(&ThisCacheInfo, cache.PostCheck) TODO:(DingHao)postcheck暂无动态检查规则
			if stage == "stage2" {
				if exitcode, err := check.CheckDynHook(&ThisCacheInfo, cache.PostCheckFailed); err != nil {
					if !IgnoreCheckError {
						updatePostCheckStage(cache.P_Stage1_Failed)
						log.Errorf("postcheck/dynhook failed:%v", err)
						CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("postcheck/dynhook failed:%v", err))
						return
					}
					updatePostCheckStage(cache.P_Stage1_Failed)
					log.Warnf("postcheck/dynhook failed:%v", err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_DYN_ERROR, exitcode, fmt.Sprintf("postcheck/dynhook failed:%v", err))
					return
				}
			}

			CheckRetMsg.SetError(0, 0)
			updatePostCheckStage(cache.P_OK)
		}

	},
}

func init() {
	checkCmd.PersistentFlags().BoolVarP(&IgnoreCheckWarning, "ignore-warning", "", true, "ignore check warning")
	checkCmd.PersistentFlags().BoolVarP(&IgnoreCheckError, "ignore-error", "", false, "ignore check error")
	checkCmd.AddCommand(preCheckCmd)
	checkCmd.AddCommand(midCheckCmd)
	checkCmd.AddCommand(postCheckCmd)
	rootCmd.AddCommand(checkCmd)

	// preCheckCmd.Flags().BoolVarP(&beforeDownload, "before-download", "", false, "check with download before")
	// preCheckCmd.Flags().BoolVarP(&afterDownload, "after-download", "", false, "check with download after")
	preCheckCmd.Flags().BoolVarP(&OnlyCheckCorelist, "only-check-corelist", "", false, "only check core package list")

	midCheckCmd.Flags().BoolVarP(&CheckWithEmulation, "check-emu", "", false, "check with emulation")
	midCheckCmd.Flags().BoolVarP(&OnlyCheckCorelist, "only-check-corelist", "", false, "only check core package list")

	midCheckCmd.Flags().StringVarP(&CoreProtectPath, "protect-file", "p", "", "system os core proctect file path")

	postCheckCmd.Flags().BoolVarP(&CheckWithSucceed, "check-succeed", "", true, "check with success")
	postCheckCmd.Flags().BoolVarP(&CheckWithFailed, "check-failed", "", false, "check with failed")
	postCheckCmd.Flags().BoolVarP(&PostCheckStage1, "stage1", "", false, "postcheck stage1")
	postCheckCmd.Flags().BoolVarP(&PostCheckStage2, "stage2", "", false, "postcheck stage2")
}
