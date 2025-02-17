// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	utils2 "github.com/linuxdeepin/go-lib/utils"
)

type RepoType string

const (
	OSDefaultRepo  RepoType = "UOS_DEFAULT"
	OemDefaultRepo RepoType = "OEM_DEFAULT"
	CustomRepo     RepoType = "CUSTOM"
)

func (r RepoType) String() string {
	switch r {
	case OSDefaultRepo:
		return "OSDefaultRepo"
	case OemDefaultRepo:
		return "OemDefaultRepo"
	case CustomRepo:
		return "CustomRepo"
	default:
		return ""
	}
}

func (r RepoType) IsValid() bool {
	return len(r.String()) > 0
}

const (
	OemRepoDirPath                      = "/etc/deepin/lastore-daemon/oem-repo.conf.d/"
	UseOemSystemRepoFlagFile            = ".use_oem_system_source_flag"
	UseOemSecurityRepoFlagFile          = ".use_oem_security_source_flag"
	BackupOemSystemSourceListFilePath   = "/etc/apt/sources.list.bak_oem"
	BackupOemSecuritySourceListFilePath = " /etc/apt/sources.list.d/security.list.bak_oem"
)

type OemRepoConfig struct {
	UpdateType     system.UpdateType
	RepoShowNameZh string `json:"RepoShowName_zh"`
	RepoShowNameEn string `json:"RepoShowName_en"`
	RepoUrl        []string
	hasSet         bool
}

// GetOemRepoInfo 获取系统和安全仓库OEM配置
func GetOemRepoInfo(dir string) (*OemRepoConfig, *OemRepoConfig) {
	systemOemRepoInfo := &OemRepoConfig{
		UpdateType: system.SystemUpdate,
	}
	securityOemRepoInfo := &OemRepoConfig{
		UpdateType: system.SecurityUpdate,
	}
	infos, err := os.ReadDir(dir)
	if err != nil {
		logger.Warningf("failed to read %v,error is %v", dir, err)
		return nil, nil
	}
	for _, info := range infos {
		if info.IsDir() || !strings.HasSuffix(info.Name(), "json") {
			logger.Infof("skip file %v", info.Name())
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, info.Name()))
		if err != nil {
			logger.Warning(err)
			continue
		}
		var tmpInfo *OemRepoConfig
		err = json.Unmarshal(content, &tmpInfo)
		if err != nil {
			logger.Warning(err)
			continue
		}
		var targetRepoInfo *OemRepoConfig
		switch tmpInfo.UpdateType {
		case system.SystemUpdate:
			targetRepoInfo = systemOemRepoInfo
		case system.SecurityUpdate:
			targetRepoInfo = securityOemRepoInfo
		default:
			logger.Warningf("invalid update type: %v", tmpInfo.UpdateType)
			continue
		}

		if targetRepoInfo.hasSet {
			targetRepoInfo.RepoShowNameZh = ""
			targetRepoInfo.RepoShowNameEn = ""
			targetRepoInfo.RepoUrl = nil
		}
		targetRepoInfo.RepoShowNameZh = tmpInfo.RepoShowNameZh
		targetRepoInfo.RepoShowNameEn = tmpInfo.RepoShowNameEn
		targetRepoInfo.RepoUrl = tmpInfo.RepoUrl
		targetRepoInfo.hasSet = true

	}

	return systemOemRepoInfo, securityOemRepoInfo
}

// 从/etc/deepin/lastore-daemon/oem-repo.conf.d/读取标记，恢复source.list以及修改更新仓库兼容历史配置
func (c *Config) recoveryAndApplyOemFlag(typ system.UpdateType) error {
	var flagFilePath string
	var backupListPath string
	var recoveryListPath string
	var applyFunc func() error
	switch typ {
	case system.SystemUpdate:
		flagFilePath = filepath.Join(OemRepoDirPath, UseOemSystemRepoFlagFile)
		backupListPath = BackupOemSystemSourceListFilePath
		recoveryListPath = system.OriginSourceFile
		applyFunc = func() error {
			return c.SetSystemRepoType(OemDefaultRepo)
		}
	case system.SecurityUpdate:
		flagFilePath = filepath.Join(OemRepoDirPath, UseOemSecurityRepoFlagFile)
		backupListPath = BackupOemSecuritySourceListFilePath
		recoveryListPath = system.SecuritySourceFile
		applyFunc = func() error {
			return c.SetSecurityRepoType(OemDefaultRepo)
		}
	default:
		return fmt.Errorf("invalid oem update type: %v", typ)
	}
	if !utils2.IsFileExist(flagFilePath) {
		logger.Infof("flag: %v not exist, don't need to apply oem setting", flagFilePath)
		return nil
	}

	if !utils2.IsFileExist(backupListPath) {
		logger.Infof("backup file :%v not exist, don't need to recovery source list", backupListPath)
	} else {
		err := utils2.MoveFile(backupListPath, recoveryListPath)
		if err != nil {
			logger.Warning(err)
		}
	}
	err := applyFunc()
	if err != nil {
		logger.Warning(err)
		return err
	}
	return os.RemoveAll(flagFilePath)
}

func (c *Config) reloadOemRepoConfig() {
	systemOemSourceConfig, securityOemSourceConfig := GetOemRepoInfo(OemRepoDirPath)
	if systemOemSourceConfig != nil {
		c.SystemOemSourceConfig = *systemOemSourceConfig
	}
	if securityOemSourceConfig != nil {
		c.SecurityOemSourceConfig = *securityOemSourceConfig
	}
}

// ReloadSourcesDir 更新系统、安全仓库list文件，/var/lib/lastore/SystemSource.d和/var/lib/lastore/SecuritySource.d
func (c *Config) ReloadSourcesDir() {
	c.reloadOemRepoConfig()
	v, err := c.dsLastoreManager.Value(0, dSettingsKeySystemRepoType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.SystemRepoType = RepoType(v.Value().(string))
	}
	logger.Info("reload sources using config:", c.SystemRepoType)
	v, err = c.dsLastoreManager.Value(0, dSettingsKeySecurityRepoType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.SecurityRepoType = RepoType(v.Value().(string))
	}
	logger.Info("reload security using config:", c.SecurityRepoType)
	switch c.SystemRepoType {
	case OSDefaultRepo:
		err = system.UpdateSystemDefaultSourceDir(c.SystemSourceList)
	case OemDefaultRepo:
		err = system.UpdateSourceDirUseUrl(system.SystemUpdate, c.SystemOemSourceConfig.RepoUrl, "system-oem-sources.list", "Oem system repo config, Generated by lastore-daemon")
	case CustomRepo:
		err = system.UpdateSourceDirUseUrl(system.SystemUpdate, c.SystemCustomSource, "system-custom-sources.list", "Custom system repo config, Generated by lastore-daemon")
	}
	if err != nil {
		logger.Warning("update system source failed:", err)
	}

	switch c.SecurityRepoType {
	case OSDefaultRepo:
		err = system.UpdateSecurityDefaultSourceDir(c.SecuritySourceList)
	case OemDefaultRepo:
		err = system.UpdateSourceDirUseUrl(system.SecurityUpdate, c.SecurityOemSourceConfig.RepoUrl, "security-oem-sources.list", "Oem security repo config, Generated by lastore-daemon")
	case CustomRepo:
		err = system.UpdateSourceDirUseUrl(system.SecurityUpdate, c.SecurityCustomSource, "security-custom-sources.list", "Custom security repo config, Generated by lastore-daemon")
	}

	if err != nil {
		logger.Warning("update security source failed:", err)
	}

}
