// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"
	"os"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
	"github.com/spf13/cobra"
)

var (
	ConfigCfg            string
	DebugVerbose         bool
	IgnoreMetaEmptyCheck bool
	RootCoreConfig       config.CoreConfig
	CacheCfg             cache.CacheConfig
	ThisCacheInfo        cache.CacheInfo
	UpdateMetaConfigPath string
	CoreProtectPath      string
	CheckRetMsg          ecode.RetMsg
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lastore-update-tools",
	Short: "lastore system update tools",
	Long: `lastore system update tools:

deepin system update tools is a linux system upgrade tool that depends on the dpkg
package manager.`,
	// log config
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if DebugVerbose {
			log.SetDebugEnabled()
		}
		log.Debugf("load config")
		if err := RootCoreConfig.LoaderCfg(ConfigCfg); err != nil {
			log.Errorf("load config failed:%v", err)
			CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("load config failed:%v", err))
			return
		}
		// cache list load
		log.Debugf("load cache")
		if err := RootCoreConfig.LoaderCache(&CacheCfg); err != nil {
			log.Errorf("load cache failed:%v", err)
			CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("load cache failed:%v", err))
		}

		if UpdateMetaConfigPath == "" {
			log.Errorf("update meta config path is empty")
			CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, "update meta config path is empty")
		}
		if err := fs.CheckFileExistState(UpdateMetaConfigPath); err != nil {
			log.Errorf("update meta config path: %v", err)
			CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("update meta config path: %v", err))
		}

		log.Debugf("load update metadata")

		if UpdateMetaConfigPath != "" {
			var loaderUpdateMeta cache.UpdateInfo
			if err := loaderUpdateMeta.LoaderJson(UpdateMetaConfigPath); err != nil {
				log.Errorf("load meta config failed: %+v", err)
				CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("load meta config failed: %v", err))
				return
			}

			if err := loaderUpdateMeta.UpdateInfoFormatVerify(); err != nil && IgnoreMetaEmptyCheck == false {
				log.Errorf("verify update meta json failed %+v", err)
				CheckRetMsg.ExitOutput(ecode.CHK_INVALID_INPUT, ecode.CHK_METAINFO_FILE_ERROR, fmt.Sprintf("verify update meta json failed %v", err))
				return
			}
			if CacheCfg.Cache == nil {
				CacheCfg.Cache = make(map[string]cache.CacheInfo)
			}
			if cacheInfo, ok := CacheCfg.Cache[loaderUpdateMeta.UUID]; ok {
				cacheInfo.UpdateMetaInfo.MergeConfig(loaderUpdateMeta)
				CacheCfg.Cache[loaderUpdateMeta.UUID] = cacheInfo
				ThisCacheInfo = CacheCfg.Cache[loaderUpdateMeta.UUID]
				// log.Debugf("%v",xx)
			} else {
				log.Debugf("add update meta to cache")
				newCacheInfo := cache.CacheInfo{}
				newCacheInfo.UUID = loaderUpdateMeta.UUID
				newCacheInfo.UpdateMetaInfo = loaderUpdateMeta
				newCacheInfo.WorkStation = RootCoreConfig.Base + "/" + loaderUpdateMeta.UUID
				CacheCfg.Cache[loaderUpdateMeta.UUID] = newCacheInfo
				ThisCacheInfo = CacheCfg.Cache[loaderUpdateMeta.UUID]
				log.Debugf("add cache to cfg with:%v", loaderUpdateMeta.UUID)
			}

			if err := fs.CreateDirMode(ThisCacheInfo.WorkStation, 0755); err != nil {
				log.Warnf("create uuid %v failed: %v", ThisCacheInfo.UUID, err)
			}
		}
		if SysPkgInfo == nil {
			SysPkgInfo = make(map[string]*cache.AppTinyInfo)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// flush config to disk
		log.Debugf("flush cache")
		if _, ok := CacheCfg.Cache[ThisCacheInfo.UUID]; ok {
			CacheCfg.Cache[ThisCacheInfo.UUID] = ThisCacheInfo
		}
		if err := RootCoreConfig.UpdateCache(&CacheCfg); err != nil {
			log.Errorf("%+v", err)
			CheckRetMsg.PushExtMsg(fmt.Sprintf("flush cache:%+v", err))
		} else if err := RootCoreConfig.UpdateCfg(ConfigCfg); err != nil {
			CheckRetMsg.PushExtMsg(fmt.Sprintf("flush config:%+v", err))
			log.Errorf("%+v", err)
		}

		log.Debugf("return code: %d", CheckRetMsg.Code)
		if CheckRetMsg.Code != 0 {
			CheckRetMsg.RetMsgToJson()
			os.Exit(int(CheckRetMsg.Code))
		}

	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfigCfg, "config", "c", check.CheckBaseDir+"config.yaml", "config file")

	rootCmd.PersistentFlags().StringVarP(&UpdateMetaConfigPath, "meta-cfg", "m", check.CheckBaseDir+"default.json", "update meta info with update platform")

	//rootCmd.PersistentFlags().StringVarP(&DataCfgPath, "data", "d", "", "data file")
	rootCmd.PersistentFlags().BoolVarP(&DebugVerbose, "debug", "d", false, "debug mode")
	rootCmd.PersistentFlags().BoolVarP(&IgnoreMetaEmptyCheck, "ignore-meta-empty-check", "", false, "ignore meta empty check")

	cobra.OnInitialize()

}
