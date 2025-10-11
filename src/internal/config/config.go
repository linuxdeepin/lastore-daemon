// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/godbus/dbus/v5"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
)

const MinCheckInterval = time.Minute
const ConfigVersion = "0.1"

// LastoreDaemonStatus 由于lastore-daemon会闲时退出,dde-session-shell和dde-control-center需要获取实时状态时需要从dconfig获取,而不是从lastore-daemon获取
type LastoreDaemonStatus uint32

var logger = log.NewLogger("lastore/config")

const (
	CanUpgrade    LastoreDaemonStatus = 1 << 0 // 是否可以进行安装更新操作
	DisableUpdate LastoreDaemonStatus = 1 << 1 // 当前系统是否禁用了更新
	ForceUpdate   LastoreDaemonStatus = 1 << 2 // 关机强制更新
)

type DisabledStatus uint32

const (
	DisabledUnknown         DisabledStatus = 1 << 0 // 禁用重启后的检查项，1063前的版本不兼容需要禁用 // 该配置项不作为配置生效
	DisabledVersion         DisabledStatus = 1 << 1 // 禁用version请求
	DisabledUpdateLog       DisabledStatus = 1 << 2 // 禁用systemupdatelogs请求
	DisabledTargetPkgLists  DisabledStatus = 1 << 3
	DisabledCurrentPkgLists DisabledStatus = 1 << 4
	DisabledPkgCVEs         DisabledStatus = 1 << 5
	DisabledProcess         DisabledStatus = 1 << 6
	DisabledResult          DisabledStatus = 1 << 7
	DisabledRebootCheck     DisabledStatus = 1 << 8 // 禁用重启后的检查项，1063前的版本不兼容需要禁用
)

type Config struct {
	Version               string
	AutoCheckUpdates      bool
	DisableUpdateMetadata bool
	AutoDownloadUpdates   bool
	AutoClean             bool
	MirrorSource          string
	UpdateNotify          bool
	CheckInterval         time.Duration
	CleanInterval         time.Duration
	UpdateMode            system.UpdateType
	CheckUpdateMode       system.UpdateType
	IncrementalUpdate     bool

	// 缓存大小超出限制时的清理时间间隔
	CleanIntervalCacheOverLimit    time.Duration
	AppstoreRegion                 string
	LastCheckTime                  time.Time
	LastCleanTime                  time.Time
	LastCheckCacheSizeTime         time.Time
	Repository                     string
	MirrorsUrl                     string
	AllowInstallRemovePkgExecPaths []string
	AutoInstallUpdates             bool
	AutoInstallUpdateType          system.UpdateType

	AllowPostSystemUpgradeMessageVersion []string // 只有数组内的系统版本被允许发送更新完成的数据

	dsLastoreManager   ConfigManager.Manager
	useDSettings       bool
	UpgradeStatus      system.UpgradeStatusAndReason
	IdleDownloadConfig string
	SystemSourceList   []string // 系统更新list文件路径
	SecuritySourceList []string // 安全更新list文件路径
	NonUnknownList     []string // 非未知来源更新list文件
	OtherSourceList    []string // 其他类型更新list文件路径

	DownloadSpeedLimitConfig string
	lastoreDaemonStatus      LastoreDaemonStatus
	UpdateStatus             string
	PlatformUpdate           bool

	PlatformUrl            string // 更新接口地址
	CheckPolicyCron        string // 策略检查间隔
	StartCheckRange        []int  // 开机检查更新区间
	IncludeDiskInfo        bool   // machineID是否包含硬盘信息
	PostUpgradeCron        string // 更新上报间隔
	UpdateTime             string // 定时更新
	PlatformDisabled       DisabledStatus
	EnableVersionCheck     bool
	PlatformRepoComponents string // 更新平台仓库组件

	ClassifiedUpdatablePackages map[string][]string
	OnlineCache                 string

	EnableCoreList    bool
	ClientPackageName string

	SystemCustomSource      []string      // 系统更新自定义仓库内容
	SecurityCustomSource    []string      // 安全更新自定义仓库内容
	SystemOemSourceConfig   OemRepoConfig // 系统更新OEM仓库配置 来源/etc/deepin/lastore-daemon/oem-repo.conf.d
	SecurityOemSourceConfig OemRepoConfig // 安全更新OEM仓库配置 来源/etc/deepin/lastore-daemon/oem-repo.conf.d
	SystemRepoType          RepoType      // 系统更新仓库类型
	SecurityRepoType        RepoType      // 安全更新仓库类型

	filePath string
	statusMu sync.RWMutex

	dsettingsChangedCbMap   map[string]func(LastoreDaemonStatus, interface{})
	dsettingsChangedCbMapMu sync.Mutex
}

func NewConfig(configPath string) *Config {
	dc := getConfigFromDSettings()
	dc.filePath = configPath
	if !dc.useDSettings { // 从config文件迁移至DSettings
		var c *Config = &Config{
			UpdateMode: system.SystemUpdate | system.SecurityUpdate,
		}
		err := system.DecodeJson(configPath, &c)
		if err != nil {
			logger.Debugf("Can't load config file: %v\n", err)
		} else {
			logger.Info("transfer config.json to DSettings")
			dc.json2DSettings(c)
		}
		_ = dc.SetUseDSettings(true)
	}
	if dc.CheckInterval < MinCheckInterval {
		_ = dc.SetCheckInterval(MinCheckInterval)
	}
	if dc.Version == "" {
		_ = dc.SetVersion(ConfigVersion)
		_ = dc.SetCheckInterval(time.Hour * 24 * 7)
		_ = dc.SetCleanInterval(time.Hour * 24 * 7)
	}

	return dc
}

const (
	dSettingsAppID                                   = "org.deepin.dde.lastore"
	dSettingsLastoreName                             = "org.deepin.dde.lastore"
	dSettingsKeyUseDSettings                         = "use-dsettings"
	dSettingsKeyVersion                              = "version"
	dSettingsKeyAutoCheckUpdates                     = "auto-check-updates"
	dSettingsKeyDisableUpdateMetadata                = "disable-update-metadata"
	dSettingsKeyAutoDownloadUpdates                  = "auto-download-updates"
	dSettingsKeyAutoClean                            = "auto-clean"
	dSettingsKeyMirrorSource                         = "mirror-source"
	dSettingsKeyUpdateNotify                         = "update-notify"
	dSettingsKeyCheckInterval                        = "check-interval"
	dSettingsKeyCleanInterval                        = "clean-interval"
	dSettingsKeyUpdateMode                           = "update-mode"
	dSettingsKeyCheckUpdateMode                      = "check-update-mode"
	dSettingsKeyCleanIntervalCacheOverLimit          = "clean-internal-cache-over-limit"
	dSettingsKeyAppstoreRegion                       = "appstore-region"
	dSettingsKeyLastCheckTime                        = "last-check-time"
	dSettingsKeyLastCleanTime                        = "last-clean-time"
	dSettingsKeyLastCheckCacheSizeTime               = "last-check-cache-size-time"
	dSettingsKeyRepository                           = "repository"
	dSettingsKeyMirrorsUrl                           = "mirrors-url"
	dSettingsKeyAllowInstallRemovePkgExecPaths       = "allow-install-remove-pkg-exec-paths"
	dSettingsKeyAutoInstallUpdates                   = "auto-install-updates"
	dSettingsKeyAutoInstallUpdateType                = "auto-install-update-type"
	dSettingsKeyAllowPostSystemUpgradeMessageVersion = "allow-post-system-upgrade-message-version"
	dSettingsKeyUpgradeStatus                        = "upgrade-status"
	dSettingsKeyIdleDownloadConfig                   = "idle-download-config"
	dSettingsKeySystemSourceList                     = "system-sources"
	dSettingsKeyNonUnknownList                       = "non-unknown-sources"
	dSettingsKeyDownloadSpeedLimit                   = "download-speed-limit"
	DSettingsKeyLastoreDaemonStatus                  = "lastore-daemon-status"
	dSettingsKeyUpdateStatus                         = "update-status"
	dSettingsKeyPlatformUpdate                       = "platform-update"
	dSettingsKeyPlatformUrl                          = "platform-url"
	dSettingsKeyCheckPolicyOnCalendar                = "check-policy-on-calendar"
	dSettingsKeyStartCheckRange                      = "start-check-range"
	dSettingsKeyIncludeDiskInfo                      = "include-disk-info"
	dSettingsKeyPostUpgradeOnCalendar                = "post-upgrade-on-calendar"
	dSettingsKeyUpdateTime                           = "update-time"
	dSettingsKeyPlatformDisabled                     = "platform-disabled"
	dSettingsKeyEnableVersionCheck                   = "enable-version-check"
	dSettingsKeyEnableCoreList                       = "enable-core-list"
	dSettingsKeyClientPackageName                    = "client-package-name"
	dSettingsKeySystemCustomSource                   = "system-custom-source"
	dSettingsKeySecurityCustomSource                 = "security-custom-source"
	dSettingsKeySystemRepoType                       = "system-repo-type"
	dSettingsKeySecurityRepoType                     = "security-repo-type"
	dSettingsKeyPlatformRepoComponents               = "platform-repo-components"
	dSettingsKeyIncrementalUpdate                    = "incremental-update"
)

const configTimeLayout = "2006-01-02T15:04:05.999999999-07:00"

func getConfigFromDSettings() *Config {
	c := &Config{}
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return c
	}
	ds := ConfigManager.NewConfigManager(sysBus)
	dsPath, err := ds.AcquireManager(0, dSettingsAppID, dSettingsLastoreName, "")
	if err != nil {
		logger.Warning(err)
		return c
	}
	c.dsLastoreManager, err = ConfigManager.NewManager(sysBus, dsPath)
	if err != nil {
		logger.Warning(err)
		return c
	}
	systemSigLoop := dbusutil.NewSignalLoop(sysBus, 10)
	systemSigLoop.Start()
	c.dsLastoreManager.InitSignalExt(systemSigLoop, true)
	// 从DSettings获取所有内容，更新config
	v, err := c.dsLastoreManager.Value(0, dSettingsKeyUseDSettings)
	if err != nil {
		logger.Warning(err)
	} else {
		c.useDSettings = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		c.Version = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAutoCheckUpdates)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoCheckUpdates = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyDisableUpdateMetadata)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DisableUpdateMetadata = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAutoDownloadUpdates)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoDownloadUpdates = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAutoClean)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoClean = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyIncrementalUpdate)
	if err != nil {
		logger.Warning(err)
	} else {
		c.IncrementalUpdate = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyMirrorSource)
	if err != nil {
		logger.Warning(err)
	} else {
		c.MirrorSource = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyUpdateNotify)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateNotify = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCheckInterval)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CheckInterval = time.Duration(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCleanInterval)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CleanInterval = time.Duration(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyUpdateMode)
	if err != nil {
		logger.Warning(err)
	} else {
		if (c.UpdateMode & system.OnlySecurityUpdate) != 0 {
			c.UpdateMode &= ^system.OnlySecurityUpdate
			c.UpdateMode |= system.SecurityUpdate
		}
		c.UpdateMode = system.UpdateType(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCleanIntervalCacheOverLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CleanIntervalCacheOverLimit = time.Duration(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAppstoreRegion)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AppstoreRegion = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyLastCheckTime)
	if err != nil {
		logger.Warning(err)
	} else {
		s := v.Value().(string)
		c.LastCheckTime, err = time.Parse(configTimeLayout, s)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyLastCleanTime)
	if err != nil {
		logger.Warning(err)
	} else {
		s := v.Value().(string)
		c.LastCleanTime, err = time.Parse(configTimeLayout, s)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyLastCheckCacheSizeTime)
	if err != nil {
		logger.Warning(err)
	} else {
		s := v.Value().(string)
		c.LastCheckCacheSizeTime, err = time.Parse(configTimeLayout, s)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyRepository)
	if err != nil {
		logger.Warning(err)
	} else {
		c.Repository = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyMirrorsUrl)
	if err != nil {
		logger.Warning(err)
	} else {
		c.MirrorsUrl = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAllowInstallRemovePkgExecPaths)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.AllowInstallRemovePkgExecPaths = append(c.AllowInstallRemovePkgExecPaths, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAutoInstallUpdates)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoInstallUpdates = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAutoInstallUpdateType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoInstallUpdateType = system.UpdateType(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyAllowPostSystemUpgradeMessageVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.AllowPostSystemUpgradeMessageVersion = append(c.AllowPostSystemUpgradeMessageVersion, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyUpgradeStatus)
	if err != nil {
		logger.Warning(err)
	} else {
		statusContent := v.Value().(string)
		err = json.Unmarshal([]byte(statusContent), &c.UpgradeStatus)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyIdleDownloadConfig)
	if err != nil {
		logger.Warning(err)
	} else {
		c.IdleDownloadConfig = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeySystemSourceList)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.SystemSourceList = append(c.SystemSourceList, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyNonUnknownList)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.NonUnknownList = append(c.NonUnknownList, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyDownloadSpeedLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DownloadSpeedLimitConfig = v.Value().(string)
	}

	updateLastoreDaemonStatus := func() {
		v, err = c.dsLastoreManager.Value(0, DSettingsKeyLastoreDaemonStatus)
		if err != nil {
			logger.Warning(err)
		} else {
			c.lastoreDaemonStatus = LastoreDaemonStatus(v.Value().(int64))
		}
	}
	updateLastoreDaemonStatus()
	_, err = c.dsLastoreManager.ConnectValueChanged(func(key string) {
		switch key {
		case DSettingsKeyLastoreDaemonStatus:
			oldStatus := c.lastoreDaemonStatus
			updateLastoreDaemonStatus()
			newStatus := c.lastoreDaemonStatus
			if (oldStatus & DisableUpdate) != (newStatus & DisableUpdate) {
				c.dsettingsChangedCbMapMu.Lock()
				cb := c.dsettingsChangedCbMap[key]
				if cb != nil {
					go cb(DisableUpdate, c.lastoreDaemonStatus)
				}
				c.dsettingsChangedCbMapMu.Unlock()
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCheckUpdateMode)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CheckUpdateMode = system.UpdateType(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyUpdateStatus)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateStatus = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyPlatformUpdate)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PlatformUpdate = v.Value().(bool)
	}

	var url string
	v, err = c.dsLastoreManager.Value(0, dSettingsKeyPlatformUrl)
	if err != nil {
		logger.Warning(err)
	} else {
		url = v.Value().(string)
	}
	c.PlatformUrl = url

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyPlatformRepoComponents)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PlatformRepoComponents = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCheckPolicyOnCalendar)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CheckPolicyCron = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyPostUpgradeOnCalendar)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PostUpgradeCron = v.Value().(string)
	}

	var checkRange []int64
	v, err = c.dsLastoreManager.Value(0, dSettingsKeyStartCheckRange)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			checkRange = append(checkRange, s.Value().(int64))
		}
	}

	if len(checkRange) != 2 {
		c.StartCheckRange = []int{1800, 21600}
	} else {
		if checkRange[0] < checkRange[1] {
			c.StartCheckRange = []int{int(checkRange[0]), int(checkRange[1])}
		} else {
			c.StartCheckRange = []int{int(checkRange[1]), int(checkRange[0])}
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyIncludeDiskInfo)
	if err != nil {
		logger.Warning(err)
		c.IncludeDiskInfo = true
	} else {
		c.IncludeDiskInfo = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyUpdateTime)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateTime = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyPlatformDisabled)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PlatformDisabled = DisabledStatus(v.Value().(int64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyEnableVersionCheck)
	if err != nil {
		logger.Warning(err)
	} else {
		c.EnableVersionCheck = v.Value().(bool)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyEnableCoreList)
	if err != nil {
		logger.Warning(err)
	} else {
		c.EnableCoreList = v.Value().(bool)
	}
	v, err = c.dsLastoreManager.Value(0, dSettingsKeyClientPackageName)
	if err != nil {
		logger.Warning(err)
	} else {
		c.ClientPackageName = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeySystemCustomSource)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.SystemCustomSource = append(c.SystemCustomSource, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeySecurityCustomSource)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.SecurityCustomSource = append(c.SecurityCustomSource, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeySystemRepoType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.SystemRepoType = RepoType(v.Value().(string))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeySecurityRepoType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.SecurityRepoType = RepoType(v.Value().(string))
	}

	err = c.recoveryAndApplyOemFlag(system.SystemUpdate)
	if err != nil {
		logger.Warning(err)
	}

	err = c.recoveryAndApplyOemFlag(system.SecurityUpdate)
	if err != nil {
		logger.Warning(err)
	}

	// classifiedCachePath和onlineCachePath两项数据没有存储在dconfig中，是因为数据量太大，dconfig不支持存储这么长的数据
	content, err := os.ReadFile(classifiedCachePath)
	if err != nil {
		logger.Warning(err)
	} else {
		c.ClassifiedUpdatablePackages = make(map[string][]string)
		err = json.Unmarshal(content, &c.ClassifiedUpdatablePackages)
		if err != nil {
			logger.Warning(err)
		}
	}

	content, err = os.ReadFile(onlineCachePath)
	if err != nil {
		logger.Warning(err)
	} else {
		c.OnlineCache = string(content)
	}
	c.OtherSourceList = append(c.OtherSourceList, "/etc/apt/sources.list.d/driver.list")
	c.SecuritySourceList = append(c.SecuritySourceList, system.SecuritySourceFile)
	return c
}

func (c *Config) json2DSettings(oldConfig *Config) {
	_ = c.UpdateLastCheckTime()
	_ = c.UpdateLastCleanTime()
	_ = c.UpdateLastCheckCacheSizeTime()
	_ = c.SetVersion(oldConfig.Version)
	_ = c.SetAutoCheckUpdates(oldConfig.AutoCheckUpdates)
	_ = c.SetDisableUpdateMetadata(oldConfig.DisableUpdateMetadata)
	_ = c.SetUpdateNotify(oldConfig.UpdateNotify)
	_ = c.SetAutoDownloadUpdates(oldConfig.AutoDownloadUpdates)
	_ = c.SetAutoClean(oldConfig.AutoClean)
	_ = c.SetMirrorSource(oldConfig.MirrorSource)
	_ = c.SetAppstoreRegion(oldConfig.AppstoreRegion)
	_ = c.SetUpdateMode(oldConfig.UpdateMode)
	_ = c.SetCleanIntervalCacheOverLimit(oldConfig.CleanIntervalCacheOverLimit)
	_ = c.SetAutoInstallUpdates(oldConfig.AutoInstallUpdates)
	_ = c.SetAutoInstallUpdateType(oldConfig.AutoInstallUpdateType)
	_ = c.SetAllowPostSystemUpgradeMessageVersion(append(oldConfig.AllowPostSystemUpgradeMessageVersion, c.AllowPostSystemUpgradeMessageVersion...))
	_ = c.SetCheckInterval(oldConfig.CheckInterval)
	_ = c.SetCleanInterval(oldConfig.CleanInterval)
	_ = c.SetRepository(oldConfig.Repository)
	_ = c.SetMirrorsUrl(oldConfig.MirrorsUrl)
	_ = c.SetAllowInstallRemovePkgExecPaths(append(oldConfig.AllowInstallRemovePkgExecPaths, c.AllowInstallRemovePkgExecPaths...))
	return
}

func (c *Config) ConnectConfigChanged(key string, cb func(LastoreDaemonStatus, interface{})) {
	if c.dsettingsChangedCbMap == nil {
		c.dsettingsChangedCbMap = make(map[string]func(LastoreDaemonStatus, interface{}))
	}
	c.dsettingsChangedCbMapMu.Lock()
	c.dsettingsChangedCbMap[key] = cb
	c.dsettingsChangedCbMapMu.Unlock()
}

func (c *Config) UpdateLastCheckTime() error {
	c.LastCheckTime = time.Now()
	return c.save(dSettingsKeyLastCheckTime, c.LastCheckTime.Format(configTimeLayout))
}

func (c *Config) UpdateLastCleanTime() error {
	c.LastCleanTime = time.Now()
	return c.save(dSettingsKeyLastCleanTime, c.LastCleanTime.Format(configTimeLayout))
}

func (c *Config) UpdateLastCheckCacheSizeTime() error {
	c.LastCheckCacheSizeTime = time.Now()
	return c.save(dSettingsKeyLastCheckCacheSizeTime, c.LastCheckCacheSizeTime.Format(configTimeLayout))
}

func (c *Config) SetVersion(version string) error {
	c.Version = version
	return c.save(dSettingsKeyVersion, version)
}

func (c *Config) SetAutoCheckUpdates(enable bool) error {
	c.AutoCheckUpdates = enable
	return c.save(dSettingsKeyAutoCheckUpdates, enable)
}

func (c *Config) SetDisableUpdateMetadata(disable bool) error {
	c.DisableUpdateMetadata = disable
	return c.save(dSettingsKeyDisableUpdateMetadata, disable)
}

func (c *Config) SetUpdateNotify(enable bool) error {
	c.UpdateNotify = enable
	return c.save(dSettingsKeyUpdateNotify, enable)
}

func (c *Config) SetAutoDownloadUpdates(enable bool) error {
	c.AutoDownloadUpdates = enable
	return c.save(dSettingsKeyAutoDownloadUpdates, enable)
}

func (c *Config) SetAutoClean(enable bool) error {
	c.AutoClean = enable
	return c.save(dSettingsKeyAutoClean, enable)
}

func (c *Config) SetIncrementalUpdate(enable bool) error {
	c.IncrementalUpdate = enable
	return c.save(dSettingsKeyIncrementalUpdate, enable)
}

func (c *Config) SetMirrorSource(id string) error {
	c.MirrorSource = id
	return c.save(dSettingsKeyMirrorSource, id)
}

func (c *Config) SetAppstoreRegion(region string) error {
	c.AppstoreRegion = region
	return c.save(dSettingsKeyAppstoreRegion, region)
}

func (c *Config) SetUpdateMode(mode system.UpdateType) error {
	c.UpdateMode = mode
	return c.save(dSettingsKeyUpdateMode, mode)
}

func (c *Config) SetCheckUpdateMode(mode system.UpdateType) error {
	c.CheckUpdateMode = mode
	return c.save(dSettingsKeyCheckUpdateMode, mode)
}

func (c *Config) SetCleanIntervalCacheOverLimit(duration time.Duration) error {
	c.CleanIntervalCacheOverLimit = duration
	return c.save(dSettingsKeyCleanIntervalCacheOverLimit, duration)
}

func (c *Config) SetAutoInstallUpdates(autoInstall bool) error {
	c.AutoInstallUpdates = autoInstall
	return c.save(dSettingsKeyAutoInstallUpdates, autoInstall)
}

func (c *Config) SetAutoInstallUpdateType(updateType system.UpdateType) error {
	c.AutoInstallUpdateType = updateType
	return c.save(dSettingsKeyAutoInstallUpdateType, updateType)
}

func (c *Config) SetAllowPostSystemUpgradeMessageVersion(version []string) error {
	c.AllowPostSystemUpgradeMessageVersion = version
	return c.save(dSettingsKeyAllowPostSystemUpgradeMessageVersion, version)
}

func (c *Config) SetUpgradeStatusAndReason(status system.UpgradeStatusAndReason) error {
	logger.Infof("Update UpgradeStatusAndReason to %+v", status)
	c.UpgradeStatus = status
	v, err := json.Marshal(status)
	if err != nil {
		logger.Warning(err)
	}
	return c.save(dSettingsKeyUpgradeStatus, string(v))
}

func (c *Config) SetUseDSettings(use bool) error {
	c.useDSettings = use
	return c.save(dSettingsKeyUseDSettings, use)
}

func (c *Config) SetIdleDownloadConfig(idleConfig string) error {
	c.IdleDownloadConfig = idleConfig
	return c.save(dSettingsKeyIdleDownloadConfig, idleConfig)
}

func (c *Config) SetCheckInterval(interval time.Duration) error {
	c.CheckInterval = interval
	return c.save(dSettingsKeyCheckInterval, interval)
}

func (c *Config) SetCleanInterval(interval time.Duration) error {
	c.CleanInterval = interval
	return c.save(dSettingsKeyCleanInterval, interval)
}

func (c *Config) SetRepository(repository string) error {
	c.Repository = repository
	return c.save(dSettingsKeyRepository, repository)
}

func (c *Config) SetMirrorsUrl(mirrorsUrl string) error {
	c.MirrorsUrl = mirrorsUrl
	return c.save(dSettingsKeyMirrorsUrl, mirrorsUrl)
}

func (c *Config) SetAllowInstallRemovePkgExecPaths(paths []string) error {
	c.AllowInstallRemovePkgExecPaths = paths
	return c.save(dSettingsKeyAllowInstallRemovePkgExecPaths, paths)
}

// func (c *Config) SetNeedDownloadSize(size float64) error {
//	c.needDownloadSize = size
//	return c.save(dSettingsKeyNeedDownloadSize, size)
// }

func (c *Config) SetDownloadSpeedLimitConfig(config string) error {
	c.DownloadSpeedLimitConfig = config
	return c.save(dSettingsKeyDownloadSpeedLimit, config)
}

func (c *Config) SetLastoreDaemonStatus(status LastoreDaemonStatus) error {
	c.statusMu.Lock()
	c.lastoreDaemonStatus = status
	c.statusMu.Unlock()
	return c.save(DSettingsKeyLastoreDaemonStatus, status)
}

// UpdateLastoreDaemonStatus isSet: true 该位置1; false 该位清零
func (c *Config) UpdateLastoreDaemonStatus(status LastoreDaemonStatus, isSet bool) error {
	c.statusMu.Lock()
	if isSet {
		c.lastoreDaemonStatus |= status
	} else {
		c.lastoreDaemonStatus &= ^status
	}
	c.statusMu.Unlock()
	return c.save(DSettingsKeyLastoreDaemonStatus, c.lastoreDaemonStatus)
}

func (c *Config) GetLastoreDaemonStatus() LastoreDaemonStatus {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.lastoreDaemonStatus
}

func (c *Config) GetLastoreDaemonStatusByBit(key LastoreDaemonStatus) LastoreDaemonStatus {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.lastoreDaemonStatus & key
}

func (c *Config) SetUpdateStatus(status string) error {
	c.UpdateStatus = status
	return c.save(dSettingsKeyUpdateStatus, status)
}

func (c *Config) SetInstallUpdateTime(delayed string) error {
	c.UpdateTime = delayed
	return c.save(dSettingsKeyUpdateTime, c.UpdateTime)
}

func (c *Config) SetSystemCustomSource(sources []string) error {
	c.SystemCustomSource = sources
	return c.save(dSettingsKeySystemCustomSource, sources)
}

func (c *Config) SetSecurityCustomSource(sources []string) error {
	c.SecurityCustomSource = sources
	return c.save(dSettingsKeySecurityCustomSource, sources)
}

func (c *Config) SetSystemRepoType(typ RepoType) error {
	c.SystemRepoType = typ
	return c.save(dSettingsKeySystemRepoType, typ)
}

func (c *Config) SetSecurityRepoType(typ RepoType) error {
	c.SecurityRepoType = typ
	return c.save(dSettingsKeySecurityRepoType, typ)
}

const (
	onlineCachePath     = "/tmp/platform_cache.json"
	classifiedCachePath = "/tmp/classified_cache.json"
)

func (c *Config) SetClassifiedUpdatablePackages(pkgMap map[string][]string) error {
	content, err := json.Marshal(pkgMap)
	if err != nil {
		logger.Warning(err)
		return err
	}
	c.ClassifiedUpdatablePackages = pkgMap
	return os.WriteFile(classifiedCachePath, content, 0644)
}

func (c *Config) SetOnlineCache(cache string) error {
	c.OnlineCache = cache
	return os.WriteFile(onlineCachePath, []byte(cache), 0644)
}

func (c *Config) GetPlatformStatusDisable(status DisabledStatus) bool {
	return c.PlatformDisabled&status == status
}

func (c *Config) save(key string, v interface{}) error {
	if c.dsLastoreManager != nil {
		return c.dsLastoreManager.SetValue(0, key, dbus.MakeVariant(v))
	}
	logger.Info("not exist lastore dsettings")
	return system.EncodeJson(c.filePath, c)
}
