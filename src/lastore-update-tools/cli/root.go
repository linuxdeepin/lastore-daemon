// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

var (
	ConfigCfg            string = check.CheckBaseDir + "config.yaml"
	RootCoreConfig       config.CoreConfig
	ThisCacheInfo        *cache.CacheInfo
	UpdateMetaConfigPath string = check.CheckBaseDir + "default.json"
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
		return &system.JobError{
			ErrType:      system.ErrorCheckMetaInfoFile,
			ErrDetail:    fmt.Sprintf("load config failed:%v", err),
			IsCheckError: true,
		}
	}

	if UpdateMetaConfigPath == "" {
		logger.Errorf("update meta config path is empty")
		return &system.JobError{
			ErrType:      system.ErrorCheckMetaInfoFile,
			ErrDetail:    fmt.Sprintf("update meta config path is empty"),
			IsCheckError: true,
		}
	}
	if err := fs.CheckFileExistState(UpdateMetaConfigPath); err != nil {
		logger.Errorf("update meta config path: %v", err)
		return &system.JobError{
			ErrType:      system.ErrorCheckMetaInfoFile,
			ErrDetail:    fmt.Sprintf("update meta config path: %v", err),
			IsCheckError: true,
		}
	}

	logger.Debugf("load update metadata")
	if UpdateMetaConfigPath != "" {
		var loaderUpdateMeta cache.UpdateInfo
		if err := loaderUpdateMeta.LoaderJson(UpdateMetaConfigPath); err != nil {
			logger.Errorf("load meta config failed: %+v", err)
			return &system.JobError{
				ErrType:      system.ErrorCheckMetaInfoFile,
				ErrDetail:    fmt.Sprintf("load meta config failed: %v", err),
				IsCheckError: true,
			}
		}

		newCacheInfo := cache.CacheInfo{}
		newCacheInfo.UUID = loaderUpdateMeta.UUID
		newCacheInfo.UpdateMetaInfo = loaderUpdateMeta
		newCacheInfo.WorkStation = RootCoreConfig.Base + "/" + loaderUpdateMeta.UUID
		ThisCacheInfo = &newCacheInfo
	}

	if SysPkgInfo == nil {
		SysPkgInfo = make(map[string]*cache.AppTinyInfo)
	}
	return nil
}
