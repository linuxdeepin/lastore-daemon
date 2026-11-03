// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"

	"github.com/godbus/dbus/v5"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
)

const MinCheckInterval = time.Minute
const ConfigVersion = "0.1"

// LastoreDaemonStatus 由于lastore-daemon会闲时退出,dde-session-shell和dde-control-center需要获取实时状态时需要从dconfig获取,而不是从lastore-daemon获取
type LastoreDaemonStatus uint32

var logger = log.NewLogger("lastore/config")

// DisableConsoleLogging keeps config logs away from stdout for commands that
// need machine-readable output.
func DisableConsoleLogging() {
	logger.RemoveBackendConsole()
}

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
	IntranetUpdate        bool

	// 缓存大小超出限制时的清理时间间隔
	CleanIntervalCacheOverLimit time.Duration
	AppstoreRegion              string
	LastCheckTime               time.Time
	LastCleanTime               time.Time
	LastCheckCacheSizeTime      time.Time
	Repository                  string
	MirrorsUrl                  string
	AutoInstallUpdates          bool
	AutoInstallUpdateType       system.UpdateType

	AllowPostSystemUpgradeMessageVersion []string // 只有数组内的系统版本被允许发送更新完成的数据

	dsLastoreManager   ConfigManager.Manager
	useDSettings       bool
	UpgradeStatus      system.UpgradeStatusAndReason
	IdleDownloadConfig string
	SystemSourceList   []string // 系统更新list文件路径
	SecuritySourceList []string // 安全更新list文件路径
	NonUnknownList     []string // 非未知来源更新list文件
	OtherSourceList    []string // 其他类型更新list文件路径

	DownloadSpeedLimitConfig           string // 下载限速配置
	LocalDownloadSpeedLimitConfig      string
	DeliveryRemoteDownloadGlobalLimit  string
	DeliveryRemoteUploadGlobalLimit    string
	DeliveryRemoteDownloadPeakLimit    string
	DeliveryRemoteUploadPeakLimit      string
	DeliveryRemoteDownloadOffPeakLimit string
	DeliveryRemoteUploadOffPeakLimit   string
	DeliveryLocalDownloadGlobalLimit   string
	DeliveryLocalUploadGlobalLimit     string
	DeliveryLocalDownloadPeakLimit     string
	DeliveryLocalUploadPeakLimit       string
	DeliveryLocalDownloadOffPeakLimit  string
	DeliveryLocalUploadOffPeakLimit    string
	lastoreDaemonStatus                LastoreDaemonStatus
	UpdateStatus                       string
	PlatformUpdate                     bool

	PlatformUrl            string // 更新接口地址
	StartCheckRange        []int  // 开机检查更新区间
	GetHardwareIdByHelper  bool   // machineID是否直接从sync helper获取
	IncludeDiskInfo        bool   // machineID是否包含硬盘信息
	PostUpgradeCron        string // 更新上报间隔
	UpdateTime             string // 定时更新
	PlatformDisabled       DisabledStatus
	EnableVersionCheck     bool
	UpdateProcessUpload    bool
	PlatformRepoComponents string // 更新平台仓库组件
	UpgradeDeliveryEnabled bool   // 传递优化服务是否开机可用

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

	dsettingsChangedCbMap   map[string]func(interface{}, interface{})
	dsettingsChangedCbMapMu sync.Mutex
	dsettingsMu             sync.RWMutex
	// dsettingsReloadMu serializes whole reload/reconnect/invalidate cycles.
	// It is separate from dsettingsMu because a reload may call save() (e.g.
	// recoveryAndApplyOemFlag -> SetSystemRepoType), and dsettingsMu must not
	// be held there: RWMutex is not reentrant.
	dsettingsReloadMu sync.Mutex
	dsettingsSigLoop  *dbusutil.SignalLoop
}

func NewConfig(configPath string) *Config {
	dc := &Config{}
	if err := dc.loadFromDSettings(); err != nil {
		// dconfig 不可用时不致命：useDSettings 保持 false，下面会尝试从
		// 本地 config 文件迁移。
		logger.Warningf("initial dconfig load failed: %v", err)
	}
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
	DSettingsKeyAutoDownloadUpdates                  = "auto-download-updates"
	dSettingsKeyAutoClean                            = "auto-clean"
	dSettingsKeyMirrorSource                         = "mirror-source"
	dSettingsKeyUpdateNotify                         = "update-notify"
	DSettingsKeyCheckInterval                        = "check-interval"
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
	dSettingsKeyAutoInstallUpdates                   = "auto-install-updates"
	dSettingsKeyAutoInstallUpdateType                = "auto-install-update-type"
	dSettingsKeyAllowPostSystemUpgradeMessageVersion = "allow-post-system-upgrade-message-version"
	dSettingsKeyUpgradeStatus                        = "upgrade-status"
	dSettingsKeyIdleDownloadConfig                   = "idle-download-config"
	dSettingsKeySystemSourceList                     = "system-sources"
	dSettingsKeyNonUnknownList                       = "non-unknown-sources"
	DSettingsKeyDownloadSpeedLimit                   = "download-speed-limit"
	dSettingsKeyLocalDownloadSpeedLimit              = "local-download-speed-limit"
	DSettingsKeyLastoreDaemonStatus                  = "lastore-daemon-status"
	dSettingsKeyUpdateStatus                         = "update-status"
	DSettingsKeyPlatformUpdate                       = "platform-update"
	DSettingsKeyPlatformUrl                          = "platform-url"
	DSettingsKeyStartCheckRange                      = "start-check-range"
	DSettingsKeyIncludeDiskInfo                      = "include-disk-info"
	dSettingsKeyPostUpgradeOnCalendar                = "post-upgrade-on-calendar"
	dSettingsKeyUpdateTime                           = "update-time"
	dSettingsKeyPlatformDisabled                     = "platform-disabled"
	dSettingsKeyEnableVersionCheck                   = "enable-version-check"
	dSettingsKeyUpdateProcessUpload                  = "update-process-upload"
	dSettingsKeyEnableCoreList                       = "enable-core-list"
	dSettingsKeyClientPackageName                    = "client-package-name"
	DSettingsKeyUpgradeDeliveryEnabled               = "upgrade-delivery-enabled"
	dSettingsKeySystemCustomSource                   = "system-custom-source"
	dSettingsKeySecurityCustomSource                 = "security-custom-source"
	dSettingsKeySystemRepoType                       = "system-repo-type"
	dSettingsKeySecurityRepoType                     = "security-repo-type"
	dSettingsKeyPlatformRepoComponents               = "platform-repo-components"
	DSettingsKeyIncrementalUpdate                    = "incremental-update"
	DSettingsKeyIntranetUpdate                       = "intranet-update"
	DSettingsKeyGetHardwareIdByHelper                = "hardware-id-from-helper"
	DSettingsKeyDeliveryRemoteDownloadGlobalLimit    = "delivery-remote-download-global-limit"
	DSettingsKeyDeliveryRemoteUploadGlobalLimit      = "delivery-remote-upload-global-limit"
	DSettingsKeyDeliveryRemoteDownloadPeakLimit      = "delivery-remote-download-peak-limit"
	DSettingsKeyDeliveryRemoteUploadPeakLimit        = "delivery-remote-upload-peak-limit"
	DSettingsKeyDeliveryRemoteDownloadOffPeakLimit   = "delivery-remote-download-offpeak-limit"
	DSettingsKeyDeliveryRemoteUploadOffPeakLimit     = "delivery-remote-upload-offpeak-limit"
	dSettingsKeyDeliveryLocalDownloadGlobalLimit     = "delivery-local-download-global-limit"
	dSettingsKeyDeliveryLocalUploadGlobalLimit       = "delivery-local-upload-global-limit"
	dSettingsKeyDeliveryLocalDownloadPeakLimit       = "delivery-local-download-peak-limit"
	dSettingsKeyDeliveryLocalUploadPeakLimit         = "delivery-local-upload-peak-limit"
	dSettingsKeyDeliveryLocalDownloadOffPeakLimit    = "delivery-local-download-offpeak-limit"
	dSettingsKeyDeliveryLocalUploadOffPeakLimit      = "delivery-local-upload-offpeak-limit"
)

const configTimeLayout = "2006-01-02T15:04:05.999999999-07:00"

var (
	onlineCachePath     = "/var/lib/lastore/platform_cache.json"
	classifiedCachePath = "/var/lib/lastore/classified_cache.json"

	// CheckPolicyCacheFile 由 lastore-tools checkpolicy 写入，daemon 在内网下载失败时清理。
	// 与其它 root 临时文件一致放在 root 独占的运行时目录，避免 /tmp 固定路径被抢占。
	CheckPolicyCacheFile = filepath.Join(utils.RuntimeDirPath, "checkpolicy.cache")
)

// connectDSettings establishes the dconfig DBus proxy and signal loop, then
// publishes them under dsettingsMu. It returns the new proxy so callers that
// need to read config values can do so on a local copy without holding the
// lock (the lock must NOT be held across DBus calls, because the reload path
// can re-enter save()).
func (c *Config) connectDSettings() (ConfigManager.Manager, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	ds := ConfigManager.NewConfigManager(sysBus)
	dsPath, err := ds.AcquireManager(0, dSettingsAppID, dSettingsLastoreName, "")
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	mgr, err := ConfigManager.NewManager(sysBus, dsPath)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	sigLoop := dbusutil.NewSignalLoop(sysBus, 10)
	sigLoop.Start()
	mgr.InitSignalExt(sigLoop, true)

	c.dsettingsMu.Lock()
	c.dsLastoreManager = mgr
	c.dsettingsSigLoop = sigLoop
	c.dsettingsMu.Unlock()
	return mgr, nil
}

// teardownDSettingsLocked detaches the current proxy and stops its signal
// loop. Caller must hold dsettingsMu (write) to publish the detach atomically;
// the Stop() call happens after unlocking.
func (c *Config) detachDSettings() *dbusutil.SignalLoop {
	c.dsettingsMu.Lock()
	sigLoop := c.dsettingsSigLoop
	c.dsettingsSigLoop = nil
	c.dsLastoreManager = nil
	c.dsettingsMu.Unlock()
	return sigLoop
}

func (c *Config) loadFromDSettings() error {
	ds, err := c.connectDSettings()
	if err != nil {
		return err
	}
	// 从DSettings获取所有内容，更新config
	v, err := ds.Value(0, dSettingsKeyUseDSettings)
	if err != nil {
		logger.Warning(err)
	} else {
		c.useDSettings = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		c.Version = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyAutoCheckUpdates)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoCheckUpdates = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyDisableUpdateMetadata)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DisableUpdateMetadata = v.Value().(bool)
	}

	v, err = ds.Value(0, DSettingsKeyAutoDownloadUpdates)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoDownloadUpdates = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyAutoClean)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoClean = v.Value().(bool)
	}

	v, err = ds.Value(0, DSettingsKeyIncrementalUpdate)
	if err != nil {
		logger.Warning(err)
	} else {
		c.IncrementalUpdate = v.Value().(bool)
	}

	v, err = ds.Value(0, DSettingsKeyIntranetUpdate)
	if err != nil {
		logger.Warning(err)
	} else {
		c.IntranetUpdate = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyMirrorSource)
	if err != nil {
		logger.Warning(err)
	} else {
		c.MirrorSource = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyUpdateNotify)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateNotify = v.Value().(bool)
	}

	v, err = ds.Value(0, DSettingsKeyCheckInterval)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CheckInterval = time.Duration(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyCleanInterval)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CleanInterval = time.Duration(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyUpdateMode)
	if err != nil {
		logger.Warning(err)
	} else {
		if (c.UpdateMode & system.OnlySecurityUpdate) != 0 {
			c.UpdateMode &= ^system.OnlySecurityUpdate
			c.UpdateMode |= system.SecurityUpdate
		}
		c.UpdateMode = system.UpdateType(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyCleanIntervalCacheOverLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CleanIntervalCacheOverLimit = time.Duration(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyAppstoreRegion)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AppstoreRegion = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyLastCheckTime)
	if err != nil {
		logger.Warning(err)
	} else {
		s := v.Value().(string)
		c.LastCheckTime, err = time.Parse(configTimeLayout, s)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = ds.Value(0, dSettingsKeyLastCleanTime)
	if err != nil {
		logger.Warning(err)
	} else {
		s := v.Value().(string)
		c.LastCleanTime, err = time.Parse(configTimeLayout, s)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = ds.Value(0, dSettingsKeyLastCheckCacheSizeTime)
	if err != nil {
		logger.Warning(err)
	} else {
		s := v.Value().(string)
		c.LastCheckCacheSizeTime, err = time.Parse(configTimeLayout, s)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = ds.Value(0, dSettingsKeyRepository)
	if err != nil {
		logger.Warning(err)
	} else {
		c.Repository = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyMirrorsUrl)
	if err != nil {
		logger.Warning(err)
	} else {
		c.MirrorsUrl = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyAutoInstallUpdates)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoInstallUpdates = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyAutoInstallUpdateType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.AutoInstallUpdateType = system.UpdateType(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyAllowPostSystemUpgradeMessageVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.AllowPostSystemUpgradeMessageVersion = append(c.AllowPostSystemUpgradeMessageVersion, s.Value().(string))
		}
	}

	v, err = ds.Value(0, dSettingsKeyUpgradeStatus)
	if err != nil {
		logger.Warning(err)
	} else {
		statusContent := v.Value().(string)
		err = json.Unmarshal([]byte(statusContent), &c.UpgradeStatus)
		if err != nil {
			logger.Warning(err)
		}
	}

	v, err = ds.Value(0, dSettingsKeyIdleDownloadConfig)
	if err != nil {
		logger.Warning(err)
	} else {
		c.IdleDownloadConfig = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeySystemSourceList)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.SystemSourceList = append(c.SystemSourceList, s.Value().(string))
		}
	}

	v, err = ds.Value(0, dSettingsKeyNonUnknownList)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.NonUnknownList = append(c.NonUnknownList, s.Value().(string))
		}
	}

	v, err = ds.Value(0, DSettingsKeyDownloadSpeedLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DownloadSpeedLimitConfig = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyLocalDownloadSpeedLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.LocalDownloadSpeedLimitConfig = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyDeliveryRemoteDownloadGlobalLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryRemoteDownloadGlobalLimit = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyDeliveryRemoteUploadGlobalLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryRemoteUploadGlobalLimit = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyDeliveryRemoteDownloadPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryRemoteDownloadPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyDeliveryRemoteUploadPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryRemoteUploadPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyDeliveryRemoteDownloadOffPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryRemoteDownloadOffPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyDeliveryRemoteUploadOffPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryRemoteUploadOffPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyDeliveryLocalDownloadGlobalLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryLocalDownloadGlobalLimit = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyDeliveryLocalUploadGlobalLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryLocalUploadGlobalLimit = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyDeliveryLocalDownloadPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryLocalDownloadPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyDeliveryLocalUploadPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryLocalUploadPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyDeliveryLocalDownloadOffPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryLocalDownloadOffPeakLimit = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyDeliveryLocalUploadOffPeakLimit)
	if err != nil {
		logger.Warning(err)
	} else {
		c.DeliveryLocalUploadOffPeakLimit = v.Value().(string)
	}

	c.updateLastoreDaemonStatus(ds)

	v, err = ds.Value(0, dSettingsKeyCheckUpdateMode)
	if err != nil {
		logger.Warning(err)
	} else {
		c.CheckUpdateMode = system.UpdateType(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyUpdateStatus)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateStatus = v.Value().(string)
	}

	v, err = ds.Value(0, DSettingsKeyPlatformUpdate)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PlatformUpdate = v.Value().(bool)
	}

	var url string
	v, err = ds.Value(0, DSettingsKeyPlatformUrl)
	if err != nil {
		logger.Warning(err)
	} else {
		url = v.Value().(string)
	}
	c.PlatformUrl = url

	v, err = ds.Value(0, dSettingsKeyPlatformRepoComponents)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PlatformRepoComponents = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyPostUpgradeOnCalendar)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PostUpgradeCron = v.Value().(string)
	}

	var checkRange []int64
	v, err = ds.Value(0, DSettingsKeyStartCheckRange)
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

	v, err = ds.Value(0, DSettingsKeyIncludeDiskInfo)
	if err != nil {
		logger.Warning(err)
		c.IncludeDiskInfo = true
	} else {
		c.IncludeDiskInfo = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyUpdateTime)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateTime = v.Value().(string)
	}

	v, err = ds.Value(0, dSettingsKeyPlatformDisabled)
	if err != nil {
		logger.Warning(err)
	} else {
		c.PlatformDisabled = DisabledStatus(v.Value().(int64))
	}

	v, err = ds.Value(0, dSettingsKeyEnableVersionCheck)
	if err != nil {
		logger.Warning(err)
	} else {
		c.EnableVersionCheck = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyUpdateProcessUpload)
	if err != nil {
		logger.Warning(err)
	} else {
		c.UpdateProcessUpload = v.Value().(bool)
	}

	v, err = ds.Value(0, dSettingsKeyEnableCoreList)
	if err != nil {
		logger.Warning(err)
	} else {
		c.EnableCoreList = v.Value().(bool)
	}
	v, err = ds.Value(0, dSettingsKeyClientPackageName)
	if err != nil {
		logger.Warning(err)
	} else {
		c.ClientPackageName = v.Value().(string)
	}

	updateUpgradeDeliveryEnabled := func() {
		v, err = ds.Value(0, DSettingsKeyUpgradeDeliveryEnabled)
		if err != nil {
			logger.Warning(err)
			c.UpgradeDeliveryEnabled = false
		} else {
			c.UpgradeDeliveryEnabled = v.Value().(bool)
		}
	}
	updateUpgradeDeliveryEnabled()

	v, err = ds.Value(0, dSettingsKeySystemCustomSource)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.SystemCustomSource = append(c.SystemCustomSource, s.Value().(string))
		}
	}

	v, err = ds.Value(0, dSettingsKeySecurityCustomSource)
	if err != nil {
		logger.Warning(err)
	} else {
		for _, s := range v.Value().([]dbus.Variant) {
			c.SecurityCustomSource = append(c.SecurityCustomSource, s.Value().(string))
		}
	}

	v, err = ds.Value(0, dSettingsKeySystemRepoType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.SystemRepoType = RepoType(v.Value().(string))
	}

	v, err = ds.Value(0, dSettingsKeySecurityRepoType)
	if err != nil {
		logger.Warning(err)
	} else {
		c.SecurityRepoType = RepoType(v.Value().(string))
	}

	v, err = ds.Value(0, DSettingsKeyGetHardwareIdByHelper)
	if err != nil {
		logger.Warning(err)
	} else {
		c.GetHardwareIdByHelper = v.Value().(bool)
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
	// 不再回退读取 /tmp 中的旧缓存：/tmp 全局可写，缓存内容可被本地用户伪造。
	content, err := os.ReadFile(classifiedCachePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
	} else {
		c.ClassifiedUpdatablePackages = make(map[string][]string)
		err = json.Unmarshal(content, &c.ClassifiedUpdatablePackages)
		if err != nil {
			logger.Warning(err)
		}
	}

	content, err = os.ReadFile(onlineCachePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
	} else {
		c.OnlineCache = string(content)
	}
	c.OtherSourceList = append(c.OtherSourceList, "/etc/apt/sources.list.d/driver.list")
	c.SecuritySourceList = append(c.SecuritySourceList, system.SecuritySourceFile)

	_, err = ds.ConnectValueChanged(c.handleValueChanged)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

// InvalidateDSettings clears the stale dconfig proxy and stops its signal loop.
// Called when the dconfig service is detected as stopped (NameOwnerChanged with
// empty newOwner), so that subsequent callers observe a nil proxy and skip
// gracefully instead of operating on a stale connection.
//
// The detach is published under dsettingsMu; Stop() runs outside it because it
// removes the signal channel from the DBus connection and may wait on in-flight
// signal delivery.
func (c *Config) InvalidateDSettings() {
	c.dsettingsReloadMu.Lock()
	defer c.dsettingsReloadMu.Unlock()
	logger.Info("invalidating stale dconfig connection...")

	// 只有已发布的有效连接才需要失效；懒重连刚建立的新连接不能被迟到的
	// stop 信号误清空。
	if sigLoop := c.detachDSettings(); sigLoop != nil {
		sigLoop.Stop()
	}
}

// ReconnectDSettings re-establishes the dconfig connection and reloads every
// config value. It must not hold dsettingsMu across the reload: the reload
// path calls save() (recoveryAndApplyOemFlag -> SetSystemRepoType), and
// sync.RWMutex is not reentrant, so holding the write lock would self-deadlock.
func (c *Config) ReconnectDSettings() {
	c.dsettingsReloadMu.Lock()
	defer c.dsettingsReloadMu.Unlock()
	logger.Info("reconnecting dconfig service...")

	// 释放旧的信号循环资源，避免每次重连累积 goroutine 与 DBus 订阅
	if sigLoop := c.detachDSettings(); sigLoop != nil {
		sigLoop.Stop()
	}

	// 重置 append 型切片，避免重连后条目翻倍
	c.AllowPostSystemUpgradeMessageVersion = c.AllowPostSystemUpgradeMessageVersion[:0]
	c.SystemSourceList = c.SystemSourceList[:0]
	c.NonUnknownList = c.NonUnknownList[:0]
	c.SystemCustomSource = c.SystemCustomSource[:0]
	c.SecurityCustomSource = c.SecurityCustomSource[:0]
	c.OtherSourceList = c.OtherSourceList[:0]
	c.SecuritySourceList = c.SecuritySourceList[:0]

	if err := c.loadFromDSettings(); err != nil {
		logger.Warningf("reconnect dconfig failed: %v", err)
		return
	}
	logger.Info("dconfig service reconnected")
}

// ensureDSettingsConnected performs a lazy reconnect: if the dconfig proxy
// has been invalidated (service was stopped), it re-establishes the DBus
// connection and signal loop without reloading all config values. It also
// re-registers the valueChanged handler, which connectDSettings does not do.
// Used by save() and ResetDSettings() so that a write during the window
// between service restart and the NameOwnerChanged-driven full reload
// still goes through dconfig instead of being dropped.
func (c *Config) ensureDSettingsConnected() {
	// 用 TryLock 而非 Lock：save() 可能从 ReconnectDSettings 的重载路径内部
	// 被调用（recoveryAndApplyOemFlag -> SetSystemRepoType -> save），此时
	// dsettingsReloadMu 已被同 goroutine 持有，阻塞会自死锁。拿不到锁说明
	// 已有重连在进行，直接复用其产物即可。
	if !c.dsettingsReloadMu.TryLock() {
		return
	}
	defer c.dsettingsReloadMu.Unlock()

	c.dsettingsMu.RLock()
	alreadyConnected := c.dsLastoreManager != nil
	c.dsettingsMu.RUnlock()
	if alreadyConnected {
		return
	}
	logger.Info("lazy reconnect: establishing dconfig connection...")

	if sigLoop := c.detachDSettings(); sigLoop != nil {
		sigLoop.Stop()
	}

	ds, err := c.connectDSettings()
	if err != nil {
		logger.Warningf("lazy reconnect failed: %v", err)
		return
	}
	// 重新注册 ValueChanged 信号监听，否则懒重连后的 signal loop
	// 不会处理任何配置变更信号（connectDSettings 不负责注册 handler）
	if _, err := ds.ConnectValueChanged(c.handleValueChanged); err != nil {
		logger.Warningf("lazy reconnect: ConnectValueChanged failed: %v", err)
	}
	logger.Info("lazy reconnect: dconfig connection established")
}

func (c *Config) updateLastoreDaemonStatus(ds ConfigManager.Manager) {
	v, err := ds.Value(0, DSettingsKeyLastoreDaemonStatus)
	if err != nil {
		logger.Warning(err)
	} else {
		c.lastoreDaemonStatus = LastoreDaemonStatus(v.Value().(int64))
	}
}

func (c *Config) handleValueChanged(key string) {
	logger.Infof("config update: key=%s", key)
	c.dsettingsMu.RLock()
	ds := c.dsLastoreManager
	c.dsettingsMu.RUnlock()
	if ds == nil {
		logger.Warning("dsLastoreManager is nil, skip handleValueChanged")
		return
	}
	switch key {
	case DSettingsKeyIntranetUpdate:
		v, err := ds.Value(0, DSettingsKeyIntranetUpdate)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.PlatformUpdate
			newValue := v.Value().(bool)
			c.IntranetUpdate = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyPlatformUpdate:
		v, err := ds.Value(0, DSettingsKeyPlatformUpdate)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.PlatformUpdate
			newValue := v.Value().(bool)
			c.PlatformUpdate = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyPlatformUrl:
		v, err := ds.Value(0, DSettingsKeyPlatformUrl)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.PlatformUrl
			newValue := v.Value().(string)
			c.PlatformUrl = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyGetHardwareIdByHelper:
		v, err := ds.Value(0, DSettingsKeyGetHardwareIdByHelper)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.GetHardwareIdByHelper
			newValue := v.Value().(bool)
			c.GetHardwareIdByHelper = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyIncludeDiskInfo:
		v, err := ds.Value(0, DSettingsKeyIncludeDiskInfo)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.IncludeDiskInfo
			newValue := v.Value().(bool)
			c.IncludeDiskInfo = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyIncrementalUpdate:
		v, err := ds.Value(0, DSettingsKeyIncrementalUpdate)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.IncrementalUpdate
			newValue := v.Value().(bool)
			c.IncrementalUpdate = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyAutoDownloadUpdates:
		v, err := ds.Value(0, DSettingsKeyAutoDownloadUpdates)
		if err != nil {
			logger.Warning(err)
		} else {
			oldValue := c.AutoDownloadUpdates
			newValue := v.Value().(bool)
			c.AutoDownloadUpdates = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyUpgradeDeliveryEnabled:
		v, err := ds.Value(0, DSettingsKeyUpgradeDeliveryEnabled)
		if err != nil {
			logger.Warningf("failed to get value for %s: %v", DSettingsKeyUpgradeDeliveryEnabled, err)
		} else {
			oldValue := c.UpgradeDeliveryEnabled
			newValue := v.Value().(bool)
			c.UpgradeDeliveryEnabled = newValue
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldValue, newValue)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	case DSettingsKeyLastoreDaemonStatus:
		oldStatus := c.lastoreDaemonStatus
		c.updateLastoreDaemonStatus(ds)
		newStatus := c.lastoreDaemonStatus
		if (oldStatus & DisableUpdate) != (newStatus & DisableUpdate) {
			c.dsettingsChangedCbMapMu.Lock()
			cb := c.dsettingsChangedCbMap[key]
			if cb != nil {
				go cb(oldStatus, newStatus)
			}
			c.dsettingsChangedCbMapMu.Unlock()
		}
	}
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
}

func (c *Config) ConnectConfigChanged(key string, cb func(interface{}, interface{})) {
	if c.dsettingsChangedCbMap == nil {
		c.dsettingsChangedCbMap = make(map[string]func(interface{}, interface{}))
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
	return c.save(DSettingsKeyAutoDownloadUpdates, enable)
}

func (c *Config) SetAutoClean(enable bool) error {
	c.AutoClean = enable
	return c.save(dSettingsKeyAutoClean, enable)
}

func (c *Config) SetIncrementalUpdate(enable bool) error {
	c.IncrementalUpdate = enable
	return c.save(DSettingsKeyIncrementalUpdate, enable)
}

const officialCDNMirrorSourceID = "default"

type smartMirrorConfig struct {
	Enable bool
}

func isSmartMirrorEnabled() bool {
	var cfg smartMirrorConfig
	err := system.DecodeJson(filepath.Join(system.VarLibDir, "smartmirror_config.json"), &cfg)
	if err != nil {
		// 文件不存在或读取失败时，智能镜像源默认为开启状态
		return true
	}
	return cfg.Enable
}

var getEditionNameFn = system.GetEditionName
var isSmartMirrorEnabledFn = isSmartMirrorEnabled

func (c *Config) UseIncrementalUpdate() bool {
	if c.IntranetUpdate || c.UpgradeDeliveryEnabled || !c.IncrementalUpdate {
		return false
	}

	edition, err := getEditionNameFn()
	if err != nil {
		logger.Warning("failed to get edition name:", err)
		return true
	}

	if edition == "Community" {
		return !isSmartMirrorEnabledFn() && c.MirrorSource == officialCDNMirrorSourceID
	}

	return true
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
	return c.save(DSettingsKeyCheckInterval, interval)
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

// func (c *Config) SetNeedDownloadSize(size float64) error {
//	c.needDownloadSize = size
//	return c.save(dSettingsKeyNeedDownloadSize, size)
// }

func (c *Config) SetLastCheckTime(lastCheckTime time.Time) error {
	c.LastCheckTime = lastCheckTime
	return c.save(dSettingsKeyLastCheckTime, c.LastCheckTime.Format(configTimeLayout))
}

func (c *Config) SetDownloadSpeedLimitConfig(config string) error {
	c.DownloadSpeedLimitConfig = config
	return c.save(DSettingsKeyDownloadSpeedLimit, config)
}

func (c *Config) SetLocalDownloadSpeedLimitConfig(config string) error {
	c.LocalDownloadSpeedLimitConfig = config
	return c.save(dSettingsKeyLocalDownloadSpeedLimit, config)
}

func (c *Config) SetDeliveryRemoteDownloadGlobalLimit(config string) error {
	c.DeliveryRemoteDownloadGlobalLimit = config
	return c.save(DSettingsKeyDeliveryRemoteDownloadGlobalLimit, config)
}

func (c *Config) SetDeliveryRemoteUploadGlobalLimit(config string) error {
	c.DeliveryRemoteUploadGlobalLimit = config
	return c.save(DSettingsKeyDeliveryRemoteUploadGlobalLimit, config)
}

func (c *Config) SetDeliveryRemoteDownloadPeakLimit(config string) error {
	c.DeliveryRemoteDownloadPeakLimit = config
	return c.save(DSettingsKeyDeliveryRemoteDownloadPeakLimit, config)
}

func (c *Config) SetDeliveryRemoteUploadPeakLimit(config string) error {
	c.DeliveryRemoteUploadPeakLimit = config
	return c.save(DSettingsKeyDeliveryRemoteUploadPeakLimit, config)
}

func (c *Config) SetDeliveryRemoteDownloadOffPeakLimit(config string) error {
	c.DeliveryRemoteDownloadOffPeakLimit = config
	return c.save(DSettingsKeyDeliveryRemoteDownloadOffPeakLimit, config)
}

func (c *Config) SetDeliveryRemoteUploadOffPeakLimit(config string) error {
	c.DeliveryRemoteUploadOffPeakLimit = config
	return c.save(DSettingsKeyDeliveryRemoteUploadOffPeakLimit, config)
}

func (c *Config) SetDeliveryLocalDownloadGlobalLimit(config string) error {
	c.DeliveryLocalDownloadGlobalLimit = config
	return c.save(dSettingsKeyDeliveryLocalDownloadGlobalLimit, config)
}

func (c *Config) SetDeliveryLocalUploadGlobalLimit(config string) error {
	c.DeliveryLocalUploadGlobalLimit = config
	return c.save(dSettingsKeyDeliveryLocalUploadGlobalLimit, config)
}

func (c *Config) SetDeliveryLocalDownloadPeakLimit(config string) error {
	c.DeliveryLocalDownloadPeakLimit = config
	return c.save(dSettingsKeyDeliveryLocalDownloadPeakLimit, config)
}

func (c *Config) SetDeliveryLocalUploadPeakLimit(config string) error {
	c.DeliveryLocalUploadPeakLimit = config
	return c.save(dSettingsKeyDeliveryLocalUploadPeakLimit, config)
}

func (c *Config) SetDeliveryLocalDownloadOffPeakLimit(config string) error {
	c.DeliveryLocalDownloadOffPeakLimit = config
	return c.save(dSettingsKeyDeliveryLocalDownloadOffPeakLimit, config)
}

func (c *Config) SetDeliveryLocalUploadOffPeakLimit(config string) error {
	c.DeliveryLocalUploadOffPeakLimit = config
	return c.save(dSettingsKeyDeliveryLocalUploadOffPeakLimit, config)
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

func (c *Config) SetUpgradeDeliveryEnabled(enable bool) error {
	c.UpgradeDeliveryEnabled = enable
	return c.save(DSettingsKeyUpgradeDeliveryEnabled, enable)
}

func (c *Config) SetSecurityRepoType(typ RepoType) error {
	c.SecurityRepoType = typ
	return c.save(dSettingsKeySecurityRepoType, typ)
}

func (c *Config) SetStartCheckRange(checkRange []int) error {
	c.StartCheckRange = checkRange

	// TODO: remove this after bug of dde-dconfig fixed
	variants := make([]dbus.Variant, 0, len(checkRange))
	for _, item := range checkRange {
		variants = append(variants, dbus.MakeVariant(int64(item)))
	}

	return c.save(DSettingsKeyStartCheckRange, variants)
}

func (c *Config) SetClassifiedUpdatablePackages(pkgMap map[string][]string) error {
	content, err := json.Marshal(pkgMap)
	if err != nil {
		logger.Warning(err)
		return err
	}
	c.ClassifiedUpdatablePackages = pkgMap
	return utils.WriteFileSecurely(classifiedCachePath, content, 0644)
}

func (c *Config) SetOnlineCache(cache string) error {
	c.OnlineCache = cache
	return utils.WriteFileSecurely(onlineCachePath, []byte(cache), 0644)
}

func (c *Config) GetPlatformStatusDisable(status DisabledStatus) bool {
	return c.PlatformDisabled&status == status
}

func (c *Config) save(key string, v interface{}) error {
	c.dsettingsMu.RLock()
	ds := c.dsLastoreManager
	c.dsettingsMu.RUnlock()

	if ds == nil {
		// 懒重连：dconfig 连接已失效（服务重启等），尝试重新建立连接后再写入
		c.ensureDSettingsConnected()
		c.dsettingsMu.RLock()
		ds = c.dsLastoreManager
		c.dsettingsMu.RUnlock()
	}

	if ds != nil {
		return ds.SetValue(0, key, dbus.MakeVariant(v))
	}
	return errors.New("dconfig service unavailable, cannot save config")
}

func (c *Config) ResetDSettings(key string) error {
	c.dsettingsMu.RLock()
	ds := c.dsLastoreManager
	c.dsettingsMu.RUnlock()

	if ds == nil {
		c.ensureDSettingsConnected()
		c.dsettingsMu.RLock()
		ds = c.dsLastoreManager
		c.dsettingsMu.RUnlock()
	}

	if ds != nil {
		return ds.Reset(0, key)
	}
	return errors.New("dconfig service unavailable, cannot reset config")
}
