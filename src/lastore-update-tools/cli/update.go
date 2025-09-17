// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/update"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

// versionCmd represents the version command

var (
	forceIgnoreCheck  bool
	forceIgnoreError  bool
	cveOfflineInstall bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update system",
	Long:  `Uos Update System Tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		// logger.Debugf("config:%+v", Updatecfg)
		// logger.Debugf("cachecfg:%+v", CacheCfg)

		// cve offline install
		if cveOfflineInstall {
			ThisCacheInfo.Type = "secrity-offline"
			check.AdjustPkgArchWithName(&ThisCacheInfo)
			// check repo and load repo metadata
			CheckRetMsg.PushExtMsg("update/verify cve offline start")
			for _, repoinfo := range ThisCacheInfo.UpdateMetaInfo.RepoBackend {
				if err := repoinfo.LoaderPackageInfo(&ThisCacheInfo); err != nil {
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("load package info failed:%v", err))
					ThisCacheInfo.InternalState.IsCVEOffline = cache.P_Error
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
					logger.Debugf("update/verify package info failed ,pkginfo:%v,%v", pkginfo, err)
					CheckRetMsg.SetErrorExtMsg(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("update/verify package info failed ,pkginfo:%v,%v", pkginfo, err))
					ThisCacheInfo.InternalState.IsCVEOffline = cache.P_Error
					return
				}
				InstalledSizeSum += pkginfo.InstalledSize
				DebSizeSum += pkginfo.DebSize
			}

			CheckRetMsg.PushExtMsg("update/verify cve offline finish")

			if err := update.UpdatePackageInstall(&ThisCacheInfo); err != nil {
				logger.Errorf("update/inst failed : %v", err)
				if err2 := fs.CheckFileExistState(ThisCacheInfo.WorkStation + "/dpkg.log"); err2 == nil {
					CheckRetMsg.LogPath = append(CheckRetMsg.LogPath, ThisCacheInfo.WorkStation+"/dpkg.log")
				}
				CheckRetMsg.SetErrorExtMsg(ecode.CHK_ERROR, ecode.UPDATE_PKG_INSTALL_FAILED, fmt.Sprintf("update/inst failed : %v", err))
				ThisCacheInfo.InternalState.IsCVEOffline = cache.P_Error
				return
			}
			ThisCacheInfo.InternalState.IsCVEOffline = cache.P_OK
			return
		}
		CheckRetMsg.PushExtMsg(fmt.Sprintf("update/verify check start skip:%v", forceIgnoreCheck))
		// check check option
		// updateCheckTag := true
		// if !forceIgnoreCheck {
		// 	func() {
		// 		cachecfgType := reflect.TypeOf(ThisCacheInfo.InternalState)
		// 		cachecfgValue := reflect.ValueOf(ThisCacheInfo.InternalState)

		// 		for i := 0; i < cachecfgType.NumField(); i++ {
		// 			if cachecfgType.Field(i).Tag.Get("cktag") != "" && cachecfgValue.Field(i).Kind() == reflect.Bool {
		// 				if cachecfgValue.Field(i).Bool() == false {
		// 					logger.Errorf("check rules failed ! %+v : %+v", cachecfgType.Field(i).Tag.Get("cktag"), cachecfgValue.Field(i))
		// 					CheckRetMsg.SetErrorAndOutput(ecode.CHK_ERROR,
		// 						ecode.UPDATE_RULES_CHECK_FAILED,
		// 						fmt.Sprintf("%s :false ", cachecfgType.Field(i).Tag.Get("cktag")), false)
		// 					updateCheckTag = false
		// 					break
		// 				}
		// 			}
		// 		}
		// 	}()
		// }

		// if !updateCheckTag {
		// 	return
		// }
		if !forceIgnoreCheck && !ThisCacheInfo.InternalState.IsPreCheck.IsOk() {
			logger.Errorf("precheck failed ! status:%s", ThisCacheInfo.InternalState.IsPreCheck)
			CheckRetMsg.SetErrorAndOutput(ecode.CHK_ERROR,
				ecode.UPDATE_RULES_CHECK_FAILED,
				fmt.Sprintf("precheck :%s ", ThisCacheInfo.InternalState.IsPreCheck), false)
			return
		}

		CheckRetMsg.PushExtMsg("update/verify check finish")

		// if len(ThisCacheInfo.UpdateMetaInfo.PurgeList) > 0 {
		// 	if err := update.UpdatePackagePurge(&ThisCacheInfo); err != nil {
		// 		logger.Errorf("purge failed : %v", err)
		// 		ThisCacheInfo.InternalState.IsPurgeState = "failed"
		// 		if err2 := fs.CheckFileExistState(ThisCacheInfo.WorkStation + "/purge.log"); err2 == nil {
		// 			CheckRetMsg.LogPath = append(CheckRetMsg.LogPath, ThisCacheInfo.WorkStation+"/purge.log")
		// 		}
		// 		CheckRetMsg.SetError(ecode.CHK_ERROR, ecode.UPDATE_PKG_PURGE_FAILED)
		// 		if !forceIgnoreError {
		// 			return
		// 		}
		// 	}

		// }
		// FIXME:(heysion) 需要依据pkglist 来生成安装的软件包列表

		if err := update.UpdatePackageInstall(&ThisCacheInfo); err != nil {
			logger.Errorf("update failed : %v", err)
			if err2 := fs.CheckFileExistState(ThisCacheInfo.WorkStation + "/dpkg.log"); err2 == nil {
				CheckRetMsg.LogPath = append(CheckRetMsg.LogPath, ThisCacheInfo.WorkStation+"/dpkg.log")
			}
			ThisCacheInfo.InternalState.IsPurgeState = cache.P_Error
			ThisCacheInfo.InternalState.IsInstallState = cache.P_Error
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_ERROR, ecode.UPDATE_PKG_INSTALL_FAILED, fmt.Sprintf("update/inst failed : %v", err))
			return

		}
		ThisCacheInfo.InternalState.IsPurgeState = cache.P_OK
		ThisCacheInfo.InternalState.IsInstallState = cache.P_OK

		// update boot config

		// reboot

	},
}
