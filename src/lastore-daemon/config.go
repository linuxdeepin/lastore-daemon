// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"

	"internal/system"

	"github.com/godbus/dbus"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
)

const MinCheckInterval = time.Minute
const ConfigVersion = "0.1"

// 由于lastore-daemon会闲时退出,dde-session-shell和dde-control-center需要获取实时状态时需要从dconfig获取,而不是从lastore-daemon获取
type lastoreDaemonStatus uint32

const (
	canUpgrade    lastoreDaemonStatus = 1 << 0 // 是否可以进行安装更新操作
	disableUpdate lastoreDaemonStatus = 1 << 1 // 当前系统是否禁用了更新
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

	dsLastoreManager         ConfigManager.Manager
	useDSettings             bool
	upgradeStatus            system.UpgradeStatusAndReason
	idleDownloadConfig       string
	systemSourceList         []string
	nonUnknownList           []string
	needDownloadSize         float64
	downloadSpeedLimitConfig string
	lastoreDaemonStatus      lastoreDaemonStatus

	filePath string
	statusMu sync.RWMutex

	dsettingsChangedCbMap   map[string]func(lastoreDaemonStatus, interface{})
	dsettingsChangedCbMapMu sync.Mutex
}

func NewConfig(fpath string) *Config {
	dc := getConfigFromDSettings()
	dc.filePath = fpath
	if !dc.useDSettings { // 从config文件迁移至DSettings
		var c *Config
		err := system.DecodeJson(fpath, &c)
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
	if dc.Repository == "" || dc.MirrorSource == "" {
		info := system.DetectDefaultRepoInfo(system.RepoInfos)
		_ = dc.SetRepository(info.Name)
		_ = dc.SetMirrorSource("default") // info.Mirror
	}
	if dc.Version == "" {
		_ = dc.SetVersion(ConfigVersion)
		_ = dc.SetCheckInterval(time.Hour * 24 * 7)
		_ = dc.SetCleanInterval(time.Hour * 24 * 7)
	}

	return dc
}

const (
	dSettingsAppID                                   = "org.deepin.lastore"
	dSettingsLastoreName                             = "org.deepin.lastore"
	dSettingsKeyUseDSettings                         = "use-dsettings"
	dSettingsKeyVersion                              = "version"
	dSettingsKeyAutoCheckUpdates                     = "auto-check-updates"
	dSettingsKeyDisableUpdateMetadata                = "disable-update-metadata"
	dSettingsKeyAutoDownloadUpdates                  = "auto-download-updates"
	dSettingsKeyAutoClean                            = "auto-clean"
	dSettingsKeyMirrorSource                         = "mirror-source"
	dSettingsKeyUpdateNotify                         = "update-notify"
	dSettingsKeyCheckInterval                        = "check-internal"
	dSettingsKeyCleanInterval                        = "clean-internal"
	dSettingsKeyUpdateMode                           = "update-mode"
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
	dSettingsKeyLastoreDaemonStatus                  = "lastore-daemon-status"
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
		c.CheckInterval = time.Duration(v.Value().(float64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCleanInterval)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CleanInterval = time.Duration(v.Value().(float64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyUpdateMode)
	if err != nil {
		logger.Warning(err)
	} else {
		if (c.UpdateMode & system.OnlySecurityUpdate) != 0 {
			c.UpdateMode &= ^system.OnlySecurityUpdate
			c.UpdateMode |= system.SecurityUpdate
		}
		c.UpdateMode = system.UpdateType(v.Value().(float64))
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyCleanIntervalCacheOverLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CleanIntervalCacheOverLimit = time.Duration(v.Value().(float64))
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
		c.AutoInstallUpdateType = system.UpdateType(v.Value().(float64))
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
		err = json.Unmarshal([]byte(statusContent), &c.upgradeStatus)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyIdleDownloadConfig)
	if err != nil {
		logger.Warning(err)
	} else {
		c.idleDownloadConfig = v.Value().(string)
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeySystemSourceList)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.systemSourceList = append(c.systemSourceList, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyNonUnknownList)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.nonUnknownList = append(c.nonUnknownList, s.Value().(string))
		}
	}

	v, err = c.dsLastoreManager.Value(0, dSettingsKeyDownloadSpeedLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.downloadSpeedLimitConfig = v.Value().(string)
	}

	updateLastoreDaemonStatus := func() {
		v, err = c.dsLastoreManager.Value(0, dSettingsKeyLastoreDaemonStatus)
		if err != nil {
			logger.Warning(err)
		} else {
			c.lastoreDaemonStatus = lastoreDaemonStatus(v.Value().(float64))
		}
	}
	updateLastoreDaemonStatus()
	_, err = c.dsLastoreManager.ConnectValueChanged(func(key string) {
		switch key {
		case dSettingsKeyLastoreDaemonStatus:
			oldStatus := c.lastoreDaemonStatus
			updateLastoreDaemonStatus()
			newStatus := c.lastoreDaemonStatus
			if (oldStatus & disableUpdate) != (newStatus & disableUpdate) {
				c.dsettingsChangedCbMapMu.Lock()
				cb := c.dsettingsChangedCbMap[key]
				if cb != nil {
					go cb(disableUpdate, c.lastoreDaemonStatus)
				}
				c.dsettingsChangedCbMapMu.Unlock()
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
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
	if (oldConfig.UpdateMode & system.OnlySecurityUpdate) != 0 {
		oldConfig.UpdateMode &= ^system.OnlySecurityUpdate
		oldConfig.UpdateMode |= system.SecurityUpdate
	}
	_ = c.SetUpdateMode(oldConfig.UpdateMode)
	_ = c.SetCleanIntervalCacheOverLimit(oldConfig.CleanIntervalCacheOverLimit)
	_ = c.SetAutoInstallUpdates(oldConfig.AutoInstallUpdates)
	_ = c.SetAutoInstallUpdateType(oldConfig.AutoInstallUpdateType)
	_ = c.SetAllowPostSystemUpgradeMessageVersion(oldConfig.AllowPostSystemUpgradeMessageVersion)
	_ = c.SetCheckInterval(oldConfig.CheckInterval)
	_ = c.SetCleanInterval(oldConfig.CleanInterval)
	_ = c.SetRepository(oldConfig.Repository)
	_ = c.SetMirrorsUrl(oldConfig.MirrorsUrl)
	_ = c.SetAllowInstallRemovePkgExecPaths(oldConfig.AllowInstallRemovePkgExecPaths)
	return
}

func (c *Config) connectConfigChanged(key string, cb func(lastoreDaemonStatus, interface{})) {
	if c.dsettingsChangedCbMap == nil {
		c.dsettingsChangedCbMap = make(map[string]func(lastoreDaemonStatus, interface{}))
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
	c.upgradeStatus = status
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
	c.idleDownloadConfig = idleConfig
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
	c.downloadSpeedLimitConfig = config
	return c.save(dSettingsKeyDownloadSpeedLimit, config)
}

func (c *Config) SetLastoreDaemonStatus(status lastoreDaemonStatus) error {
	c.statusMu.Lock()
	c.lastoreDaemonStatus = status
	c.statusMu.Unlock()
	return c.save(dSettingsKeyLastoreDaemonStatus, status)
}

// UpdateLastoreDaemonStatus isSet: true 该位置1; false 该位清零
func (c *Config) UpdateLastoreDaemonStatus(status lastoreDaemonStatus, isSet bool) error {
	c.statusMu.Lock()
	if isSet {
		c.lastoreDaemonStatus |= status
	} else {
		c.lastoreDaemonStatus &= ^status
	}
	c.statusMu.Unlock()
	return c.save(dSettingsKeyLastoreDaemonStatus, c.lastoreDaemonStatus)
}

func (c *Config) getLastoreDaemonStatus() lastoreDaemonStatus {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.lastoreDaemonStatus
}

func (c *Config) save(key string, v interface{}) error {
	if c.dsLastoreManager != nil {
		return c.dsLastoreManager.SetValue(0, key, dbus.MakeVariant(v))
	}
	logger.Info("not exist lastore dsettings")
	return system.EncodeJson(c.filePath, c)
}
