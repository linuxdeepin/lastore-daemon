package coremodules

import (
	"fmt"
	"os"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/spf13/cobra"
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
		if DebugVerbose {
			log.SetDebugEnabled()
		}
		if err := RootCoreConfig.LoaderCfgCache(ConfigCfg, &CacheCfg); err != nil {
			log.Errorf("clear loader failed:%v", err)
			CheckRetMsg.SetErrorExtMsg(ecode.CHK_INVALID_INPUT, ecode.CLEAR_UPDATE_CACHE_FAILED, fmt.Sprintf("clear loader failed:%v", err))
			return
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if UpdateUUID == "" && !ClearAll {
			log.Debugf("clear invalid command")
			return
		}
		log.Debugf("clear start uuid:%s all: %v", UpdateUUID, ClearAll)
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
		log.Debugf("return code: %d", CheckRetMsg.Code)
		if CheckRetMsg.Code != 0 {
			CheckRetMsg.RetMsgToJson()
			os.Exit(int(CheckRetMsg.Code))
		}
	},
}

func init() {
	cleanerCmd.Flags().StringVarP(&UpdateUUID, "uuid", "u", "", "clear update uuid")
	cleanerCmd.Flags().BoolVarP(&ClearAll, "all", "", false, "clear all update cache data")
	rootCmd.AddCommand(cleanerCmd)
}
