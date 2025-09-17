// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

var (
	ConfigCfg            string = check.CheckBaseDir + "config.yaml"
	RootCoreConfig       config.CoreConfig
	CacheCfg             cache.CacheConfig
	ThisCacheInfo        cache.CacheInfo
	UpdateMetaConfigPath string = check.CheckBaseDir + "default.json"
	CoreProtectPath      string
	CheckRetMsg          ecode.RetMsg
)

// Error represents an error that occurred during the execution of the program.
type Error struct {
	Code int64
	Ext  int64
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("Code: %d, Ext: %d, Msg: %s", e.Code, e.Ext, e.Msg)
}

func initCheckEnv() error {
	logger.Debugf("initialize check environment")
	logger.Debugf("load config")
	if err := RootCoreConfig.LoaderCfg(ConfigCfg); err != nil {
		logger.Errorf("load config failed:%v", err)
		return &Error{
			Code: ecode.CHK_INVALID_INPUT,
			Ext:  ecode.CHK_METAINFO_FILE_ERROR,
			Msg:  fmt.Sprintf("load config failed:%v", err),
		}
	}
	// cache list load
	logger.Debugf("load cache")
	if err := RootCoreConfig.LoaderCache(&CacheCfg); err != nil {
		logger.Errorf("load cache failed:%v", err)
		return &Error{
			Code: ecode.CHK_INVALID_INPUT,
			Ext:  ecode.CHK_METAINFO_FILE_ERROR,
			Msg:  fmt.Sprintf("load cache failed:%v", err),
		}
	}

	if UpdateMetaConfigPath == "" {
		logger.Errorf("update meta config path is empty")
		return &Error{
			Code: ecode.CHK_INVALID_INPUT,
			Ext:  ecode.CHK_METAINFO_FILE_ERROR,
			Msg:  "update meta config path is empty",
		}
	}
	if err := fs.CheckFileExistState(UpdateMetaConfigPath); err != nil {
		logger.Errorf("update meta config path: %v", err)
		return &Error{
			Code: ecode.CHK_INVALID_INPUT,
			Ext:  ecode.CHK_METAINFO_FILE_ERROR,
			Msg:  fmt.Sprintf("update meta config path: %v", err),
		}
	}

	logger.Debugf("load update metadata")

	if UpdateMetaConfigPath != "" {
		var loaderUpdateMeta cache.UpdateInfo
		if err := loaderUpdateMeta.LoaderJson(UpdateMetaConfigPath); err != nil {
			logger.Errorf("load meta config failed: %+v", err)
			return &Error{
				Code: ecode.CHK_INVALID_INPUT,
				Ext:  ecode.CHK_METAINFO_FILE_ERROR,
				Msg:  fmt.Sprintf("load meta config failed: %v", err),
			}
		}

		if err := loaderUpdateMeta.UpdateInfoFormatVerify(); err != nil {
			logger.Errorf("verify update meta json failed %+v", err)
			return &Error{
				Code: ecode.CHK_INVALID_INPUT,
				Ext:  ecode.CHK_METAINFO_FILE_ERROR,
				Msg:  fmt.Sprintf("verify update meta json failed %v", err),
			}
		}
		if CacheCfg.Cache == nil {
			CacheCfg.Cache = make(map[string]cache.CacheInfo)
		}
		if cacheInfo, ok := CacheCfg.Cache[loaderUpdateMeta.UUID]; ok {
			cacheInfo.UpdateMetaInfo.MergeConfig(loaderUpdateMeta)
			CacheCfg.Cache[loaderUpdateMeta.UUID] = cacheInfo
			ThisCacheInfo = CacheCfg.Cache[loaderUpdateMeta.UUID]
		} else {
			logger.Debugf("add update meta to cache")
			newCacheInfo := cache.CacheInfo{}
			newCacheInfo.UUID = loaderUpdateMeta.UUID
			newCacheInfo.UpdateMetaInfo = loaderUpdateMeta
			newCacheInfo.WorkStation = RootCoreConfig.Base + "/" + loaderUpdateMeta.UUID
			CacheCfg.Cache[loaderUpdateMeta.UUID] = newCacheInfo
			ThisCacheInfo = CacheCfg.Cache[loaderUpdateMeta.UUID]
			logger.Debugf("add cache to cfg with:%v", loaderUpdateMeta.UUID)
		}

		if err := fs.CreateDirMode(ThisCacheInfo.WorkStation, 0755); err != nil {
			logger.Warningf("create uuid %v failed: %v", ThisCacheInfo.UUID, err)
		}
	}
	if SysPkgInfo == nil {
		SysPkgInfo = make(map[string]*cache.AppTinyInfo)
	}
	return nil
}

func afterCheck() {
	// flush config to disk
	logger.Debugf("after check, flush cache")
	if _, ok := CacheCfg.Cache[ThisCacheInfo.UUID]; ok {
		CacheCfg.Cache[ThisCacheInfo.UUID] = ThisCacheInfo
	}
	if err := RootCoreConfig.UpdateCache(&CacheCfg); err != nil {
		logger.Errorf("%+v", err)
		CheckRetMsg.PushExtMsg(fmt.Sprintf("flush cache:%+v", err))
	} else if err := RootCoreConfig.UpdateCfg(ConfigCfg); err != nil {
		CheckRetMsg.PushExtMsg(fmt.Sprintf("flush config:%+v", err))
		logger.Errorf("%+v", err)
	}

	logger.Debugf("return code: %d", CheckRetMsg.Code)
}
