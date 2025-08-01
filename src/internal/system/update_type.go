// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/go-lib/strv"
)

type UpdateType uint64

// 用于设置UpdateMode属性,最大支持64位
const (
	SystemUpdate       UpdateType = 1 << 0 // 系统仓库更新，检查该仓库，显示更新内容
	AppStoreUpdate     UpdateType = 1 << 1 // 应用仓库更新，检查该仓库，不显示更新内容
	SecurityUpdate     UpdateType = 1 << 2 // 安全仓库更新，检查该仓库，显示更新内容
	UnknownUpdate      UpdateType = 1 << 3 // 未知来源仓库更新:排除 系统、商店、安全、驱动、hwe仓库的其他仓库，不检查该仓库，不显示更新内容
	OnlySecurityUpdate UpdateType = 1 << 4 // 仅安全仓库更新，已经废弃，用于处理历史版本升级后的兼容问题
	OtherSystemUpdate  UpdateType = 1 << 5 // 其他来源系统的仓库更新:对应dconfig non-unknown-sources 字段去掉商店和安全仓库,通常为驱动、hwe仓库(hwe仓库应该从平台获取)，检查该仓库，不显示更新内容

	AllCheckUpdate   = SystemUpdate | AppStoreUpdate | SecurityUpdate | UnknownUpdate | OtherSystemUpdate | AppendUpdate // 所有需要检查的仓库 TODO 该字段变动，需要检查所有使用者
	AllInstallUpdate = SystemUpdate | SecurityUpdate | UnknownUpdate                                                     // 所有控制中心需要显示的仓库

	AppendUpdate UpdateType = 1 << 7 // 追加仓库/etc/deepin/lastore-daemon/sources.list.d/ 用于打印管理追加离线包.检查该仓库,不显示更新内容
)

func (m UpdateType) JobType() string {
	switch m {
	case SystemUpdate:
		return SystemUpgradeJobType
	case AppStoreUpdate:
		return AppStoreUpgradeJobType
	case SecurityUpdate, OnlySecurityUpdate:
		return SecurityUpgradeJobType
	case UnknownUpdate:
		return UnknownUpgradeJobType
	case OtherSystemUpdate:
		return OtherUpgradeJobType
	case AppendUpdate:
		return AppendUpgradeJobTye
	default:
		return ""
	}
}

func UpdateTypeBitToArray(mode UpdateType) []UpdateType {
	var res []UpdateType
	for _, typ := range AllUpdateType() {
		if typ&mode == typ {
			res = append(res, typ)
		}
	}
	return res
}

func AllUpdateType() []UpdateType {
	return []UpdateType{
		SystemUpdate,
		AppStoreUpdate,
		SecurityUpdate,
		UnknownUpdate,
		OtherSystemUpdate,
		AppendUpdate,
	}
}

// AllCheckUpdateType 对应 system.AllCheckUpdate
func AllCheckUpdateType() []UpdateType {
	return []UpdateType{
		SystemUpdate,
		AppStoreUpdate,
		SecurityUpdate,
		UnknownUpdate,
		OtherSystemUpdate,
		AppendUpdate,
	}
}

// AllInstallUpdateType 对应 system.AllInstallUpdate
func AllInstallUpdateType() []UpdateType {
	return []UpdateType{
		SystemUpdate,
		SecurityUpdate,
		UnknownUpdate,
	}
}

const (
	OriginSourceFile = "/etc/apt/sources.list"
	OriginSourceDir  = "/etc/apt/sources.list.d"

	AppStoreList       = "appstore.list"
	UnstableSourceList = "deepin-unstable-source.list"
	HweSourceList      = "hwe.list"
	DriverList         = "driver.list"
	SecurityList       = "security.list"

	AppStoreSourceFile = "/etc/apt/sources.list.d/" + AppStoreList
	UnstableSourceFile = "/etc/apt/sources.list.d/" + UnstableSourceList
	HweSourceFile      = "/etc/apt/sources.list.d/" + HweSourceList
	SecuritySourceFile = "/etc/apt/sources.list.d/" + SecurityList

	SoftLinkSystemSourceDir = "/var/lib/lastore/SystemSource.d" // 系统更新仓库
	SecuritySourceDir       = "/var/lib/lastore/SecuritySource.d"
	PlatFormSourceFile      = "/var/lib/lastore/platform.list"            // 从更新平台获取的仓库,为系统更新仓库,在message_report.go 中的 获取升级版本信息genUpdatePolicyByToken后即可 更新
	UnknownSourceDir        = "/var/lib/lastore/unknownSource.d"          // 未知来源更新的源个数不定,需要创建软链接放在同一目录内
	OtherSystemSourceDir    = "/var/lib/lastore/otherSystemSource.d"      // 其他需要检查的系统仓库
	AppendSourceDir         = "/etc/deepin/lastore-daemon/sources.list.d" // 追加仓库的路径
)

var SystemUpdateSource = SoftLinkSystemSourceDir

func SetSystemUpdate(platform bool) {
	if platform {
		SystemUpdateSource = PlatFormSourceFile
	} else {
		SystemUpdateSource = SoftLinkSystemSourceDir
	}
}

// GetCategorySourceMap 缺省更新类型与对应仓库的map
func GetCategorySourceMap() map[UpdateType]string {
	return map[UpdateType]string{
		SystemUpdate:      SystemUpdateSource,
		AppStoreUpdate:    AppStoreSourceFile,
		SecurityUpdate:    SecuritySourceDir,
		UnknownUpdate:     UnknownSourceDir,
		OtherSystemUpdate: OtherSystemSourceDir,
		AppendUpdate:      AppendSourceDir,
	}
}

const (
	LastoreSourcesPath = "/var/lib/lastore/sources.list"   // 历史版本遗留,已废弃
	CustomSourceDir    = "/var/lib/lastore/sources.list.d" // 历史版本遗留,已废弃
)

// UpdateSystemDefaultSourceDir systemSourceList需要list文件的绝对路径；更新系统仓库文件夹,如果从更新平台获取系统仓库,那么不需要调用这里
func UpdateSystemDefaultSourceDir(sourceList []string) error {
	err := os.RemoveAll(SoftLinkSystemSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	// #nosec G301
	err = os.MkdirAll(SoftLinkSystemSourceDir, 0755)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if len(sourceList) == 0 {
		sourceList = []string{UnstableSourceFile, OriginSourceFile, HweSourceFile}
	}
	// 创建对应的软链接
	for _, filePath := range sourceList {
		linkPath := filepath.Join(SoftLinkSystemSourceDir, filepath.Base(filePath))
		err = os.Symlink(filePath, linkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", filePath, err)
		}
	}
	return nil
}

func UpdateSecurityDefaultSourceDir(sourceList []string) error {
	err := os.RemoveAll(SecuritySourceDir)
	if err != nil {
		logger.Warning(err)
	}
	// #nosec G301
	err = os.MkdirAll(SecuritySourceDir, 0755)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if len(sourceList) == 0 {
		sourceList = []string{SecuritySourceFile}
	}
	// 创建对应的软链接
	for _, filePath := range sourceList {
		linkPath := filepath.Join(SecuritySourceDir, filepath.Base(filePath))
		err = os.Symlink(filePath, linkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", filePath, err)
		}
	}
	return nil
}

func UpdateSourceDirUseUrl(updateType UpdateType, repoUrl []string, fileName string, annotation string) error {
	var sourceDir string
	switch updateType {
	case SystemUpdate:
		sourceDir = SoftLinkSystemSourceDir
	case SecurityUpdate:
		sourceDir = SecuritySourceDir
	}
	err := os.RemoveAll(sourceDir)
	if err != nil {
		logger.Warning(err)
	}
	// #nosec G301
	err = os.MkdirAll(sourceDir, 0755)
	if err != nil {
		logger.Warning(err)
		return err
	}
	var content string
	content = fmt.Sprintf("## %v \n%v", annotation, strings.Join(repoUrl, "\n"))
	return os.WriteFile(filepath.Join(sourceDir, fileName), []byte(content), 0644)
}

// UpdateUnknownSourceDir 更新未知来源仓库文件夹
func UpdateUnknownSourceDir(nonUnknownList strv.Strv) error {
	// 移除旧版本内容
	err := os.RemoveAll(CustomSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	err = os.RemoveAll(LastoreSourcesPath)
	if err != nil {
		logger.Warning(err)
	}
	// 移除旧数据
	err = os.RemoveAll(UnknownSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	// #nosec G301
	err = os.MkdirAll(UnknownSourceDir, 0755)
	if err != nil {
		logger.Warning(err)
		return err
	}

	var unknownSourceFilePaths []string
	sourceDirFileInfos, err := os.ReadDir(OriginSourceDir)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if len(nonUnknownList) == 0 {
		nonUnknownList = strv.Strv{
			AppStoreList,
			SecurityList,
			DriverList,
			UnstableSourceList,
			HweSourceList,
		}
	}
	for _, fileInfo := range sourceDirFileInfos {
		name := fileInfo.Name()
		if strings.HasSuffix(name, ".list") {
			if !nonUnknownList.Contains(name) {
				unknownSourceFilePaths = append(unknownSourceFilePaths, filepath.Join(OriginSourceDir, name))
			}
		}
	}

	// 创建对应的软链接
	for _, filePath := range unknownSourceFilePaths {
		linkPath := filepath.Join(UnknownSourceDir, filepath.Base(filePath))
		err = os.Symlink(filePath, linkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", filePath, err)
		}
	}
	return nil
}

// UpdateOtherSystemSourceDir otherSourceList 需要list文件的绝对路径
func UpdateOtherSystemSourceDir(otherSourceList []string) error {
	// 移除旧数据
	err := os.RemoveAll(OtherSystemSourceDir)
	if err != nil {
		logger.Warning(err)
	}
	if len(otherSourceList) == 0 {
		logger.Info("not exist other Source need check update")
		return nil
	}
	// #nosec G301
	err = os.MkdirAll(OtherSystemSourceDir, 0755)
	if err != nil {
		logger.Warning(err)
		return err
	}
	// 创建对应的软链接
	for _, filePath := range otherSourceList {
		linkPath := filepath.Join(OtherSystemSourceDir, filepath.Base(filePath))
		err = os.Symlink(filePath, linkPath)
		if err != nil {
			return fmt.Errorf("create symlink for %q failed: %v", filePath, err)
		}
	}
	return nil
}

// CustomSourceWrapper 根据updateType组合source文件,doRealAction完成实际操作,unref用于释放资源
func CustomSourceWrapper(updateType UpdateType, doRealAction func(path string, unref func()) error) error {
	var sourcePathList []string
	for _, t := range AllCheckUpdateType() {
		category := updateType & t
		if category != 0 {
			sourcePath := GetCategorySourceMap()[t]
			sourcePathList = append(sourcePathList, sourcePath)
		}
	}
	// 由于103x版本兼容，检查更新时需要检查商店仓库
	// if updateType&AppStoreUpdate != 0 {
	// 	updateType &= ^AppStoreUpdate
	// }
	switch len(sourcePathList) {
	case 0:
		return fmt.Errorf("failed to match %v source", updateType)
	case 1:
		// 如果只有一个仓库，证明是单项的更新，可以直接使用默认的文件夹
		if doRealAction != nil {
			return doRealAction(GetCategorySourceMap()[updateType], nil)
		}
		return errors.New("doRealAction is nil")
	default:
		if doRealAction != nil {
			// 仓库组合的情况，需要重新组合文件
			var beforeDoRealErr error
			var sourceDir string
			// #nosec G301
			sourceDir, beforeDoRealErr = os.MkdirTemp("/tmp", "*Source.d")
			if beforeDoRealErr != nil {
				logger.Warning(beforeDoRealErr)
				return beforeDoRealErr
			}
			unref := func() {
				err := os.RemoveAll(sourceDir)
				if err != nil {
					logger.Warning(err)
				}
			}
			defer func() {
				if beforeDoRealErr != nil {
					unref()
				}
			}()
			var allSourceFilePaths []string
			for _, path := range sourcePathList {
				var fileInfo os.FileInfo
				fileInfo, beforeDoRealErr = os.Stat(path)
				if beforeDoRealErr != nil {
					continue
				}
				if fileInfo.IsDir() {
					var allSourceDirFileInfos []os.DirEntry
					allSourceDirFileInfos, beforeDoRealErr = os.ReadDir(path)
					if beforeDoRealErr != nil {
						continue
					}
					for _, fileInfo := range allSourceDirFileInfos {
						name := fileInfo.Name()
						if strings.HasSuffix(name, ".list") {
							allSourceFilePaths = append(allSourceFilePaths, filepath.Join(path, name))
						}
					}
				} else {
					allSourceFilePaths = append(allSourceFilePaths, path)
				}
			}

			// 创建对应的软链接
			for _, filePath := range allSourceFilePaths {
				linkPath := filepath.Join(sourceDir, filepath.Base(filePath))
				beforeDoRealErr = os.Symlink(filePath, linkPath)
				if beforeDoRealErr != nil {
					return fmt.Errorf("create symlink for %q failed: %v", filePath, beforeDoRealErr)
				}
			}
			return doRealAction(sourceDir, unref)
		}
		return errors.New("doRealAction is nil")
	}
}
