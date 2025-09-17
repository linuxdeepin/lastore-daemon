// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
)

var (
	UpdateUUID string
	ClearAll   bool
)

// cleaner command
var cleanerCmd = &cobra.Command{
	Use:   "clear",
	Short: "clear the update cache",
	Long:  `Clear the update cache and logs.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := RootCoreConfig.LoaderCfgCache(ConfigCfg, &CacheCfg); err != nil {
			logger.Errorf("clear loader failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_INVALID_INPUT, ecode.CLEAR_UPDATE_CACHE_FAILED, fmt.Sprintf("clear loader failed:%v", err))
			return
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if UpdateUUID == "" && !ClearAll {
			logger.Debugf("clear invalid command")
			return
		}
		logger.Debugf("clear start uuid:%s all: %v", UpdateUUID, ClearAll)
		CheckRetMsg.PushExtMsg(fmt.Sprintf("clear start uuid:%s all: %v", UpdateUUID, ClearAll))

		if ClearAll {
			if len(CacheCfg.Cache) > 0 {
				for UUIDCacheInfo, cacheInfo := range CacheCfg.Cache {
					cacheInfo.ClearUUID(RootCoreConfig.Base, UUIDCacheInfo)
				}
				CacheCfg.Cache = nil
			}
			return
		}
		if UpdateUUID != "" {
			if cacheInfo, ok := CacheCfg.Cache[UpdateUUID]; ok {
				cacheInfo.ClearUUID(RootCoreConfig.Base, UpdateUUID)
				delete(CacheCfg.Cache, UpdateUUID)
				return
			}
			return
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		RootCoreConfig.UpdateCache(&CacheCfg)
		logger.Debugf("return code: %d", CheckRetMsg.Code)
		if CheckRetMsg.Code != 0 {
			CheckRetMsg.RetMsgToJson()
			os.Exit(int(CheckRetMsg.Code))
		}
	},
}
