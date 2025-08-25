// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	. "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"

	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	p2pService = "uos-p2p.service"
)

type ApplicationUpdateInfo struct {
	Id             string
	Name           string
	Icon           string
	CurrentVersion string
	LastVersion    string
}
type idleDownloadConfig struct {
	IdleDownloadEnabled bool
	BeginTime           string
	EndTime             string
}

type downloadSpeedLimitConfig struct {
	DownloadSpeedLimitEnabled bool
	LimitSpeed                string
}

type Updater struct {
	manager             *Manager
	service             *dbusutil.Service
	PropsMu             sync.RWMutex
	AutoCheckUpdates    bool
	AutoDownloadUpdates bool
	UpdateNotify        bool
	MirrorSource        string
	systemdManager      systemd1.Manager

	config *Config
	// dbusutil-gen: equal=nil
	UpdatableApps []string
	// dbusutil-gen: equal=nil
	UpdatablePackages []string
	// dbusutil-gen: equal=nil
	ClassifiedUpdatablePackages map[string][]string

	AutoInstallUpdates    bool              `prop:"access:rw"`
	AutoInstallUpdateType system.UpdateType `prop:"access:rw"`

	IdleDownloadConfig          string
	idleDownloadConfigObj       idleDownloadConfig
	DownloadSpeedLimitConfig    string
	downloadSpeedLimitConfigObj downloadSpeedLimitConfig

	setDownloadSpeedLimitTimer *time.Timer
	setIdleDownloadConfigTimer *time.Timer

	UpdateTarget string

	p2PUpdateEnable  bool // p2p更新是否开启
	p2PUpdateSupport bool // 是否支持p2p更新
}

func NewUpdater(service *dbusutil.Service, m *Manager, config *Config) *Updater {
	u := &Updater{
		manager:                     m,
		service:                     service,
		config:                      config,
		AutoCheckUpdates:            config.AutoCheckUpdates,
		AutoDownloadUpdates:         config.AutoDownloadUpdates,
		MirrorSource:                config.MirrorSource,
		UpdateNotify:                config.UpdateNotify,
		AutoInstallUpdates:          config.AutoInstallUpdates,
		AutoInstallUpdateType:       config.AutoInstallUpdateType,
		IdleDownloadConfig:          config.IdleDownloadConfig,
		DownloadSpeedLimitConfig:    config.DownloadSpeedLimitConfig,
		ClassifiedUpdatablePackages: config.ClassifiedUpdatablePackages,
		systemdManager:              systemd1.NewManager(service.Conn()),
	}
	err := json.Unmarshal([]byte(u.IdleDownloadConfig), &u.idleDownloadConfigObj)
	if err != nil {
		logger.Warning(err)
	}
	err = json.Unmarshal([]byte(u.DownloadSpeedLimitConfig), &u.downloadSpeedLimitConfigObj)
	if err != nil {
		logger.Warning(err)
	}
	state, err := u.systemdManager.GetUnitFileState(0, p2pService)
	if err != nil {
		logger.Warning("get p2p service state err:", err)
		u.p2PUpdateEnable = false
		u.p2PUpdateSupport = false
	} else {
		u.p2PUpdateEnable = false
		u.p2PUpdateSupport = true
		if state == "enabled" {
			unit, err := u.getP2PUnit()
			if err != nil {
				logger.Warning("get p2p unit err:", err)
			} else {
				value, err := unit.Unit().ActiveState().Get(0)
				if err != nil {
					logger.Warning("get p2p SubState err:", err)
				}
				if value == "active" {
					u.p2PUpdateEnable = true
				}
			}
		}
	}
	return u
}

func startUpdateMetadataInfoService() {
	logger.Info("start update metadata info service")
	err := exec.Command("systemctl", "start", "lastore-update-metadata-info.service").Run()
	if err != nil {
		logger.Warningf("AutoCheck Update failed: %v", err)
	}
}

func SetAPTSmartMirror(url string) error {
	return os.WriteFile("/etc/apt/apt.conf.d/99mirrors.conf",
		([]byte)(fmt.Sprintf("Acquire::SmartMirrors::MirrorSource %q;", url)),
		0644) // #nosec G306
}

type LocaleMirrorSource struct {
	Id   string
	Url  string
	Name string
}

// 设置更新时间的接口
func (u *Updater) SetInstallUpdateTime(sender dbus.Sender, timeStr string) *dbus.Error {
	logger.Info("SetInstallUpdateTime", timeStr)

	if len(timeStr) == 0 {
		u.config.SetInstallUpdateTime(updateplatform.KeyNow)
	} else if timeStr == updateplatform.KeyNow || timeStr == updateplatform.KeyShutdown {
		u.config.SetInstallUpdateTime(timeStr)
	} else {
		updateTime, err := time.Parse(updateplatform.KeyLayout, timeStr)
		if err != nil {
			logger.Warning(err)
			updateTime, err = time.Parse(time.RFC3339, timeStr)
			if err != nil {
				logger.Warning(err)
				return dbusutil.ToError(err)
			}
		}
		u.config.SetInstallUpdateTime(updateTime.Format(time.RFC3339))
	}

	_, err := u.manager.updateSource(sender) // 自动检查更新按照控制中心更新配置进行检查
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}

const (
	aptSource       = "/etc/apt/sources.list"
	aptSourceOrigin = aptSource + ".origin"
)

func (u *Updater) setClassifiedUpdatablePackages(infosMap map[string][]string) {
	u.PropsMu.Lock()
	defer u.PropsMu.Unlock()
	_ = u.config.SetClassifiedUpdatablePackages(infosMap)
	u.setPropClassifiedUpdatablePackages(infosMap)
}

func (u *Updater) autoInstallUpdatesWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	return dbusutil.ToError(u.config.SetAutoInstallUpdates(pw.Value.(bool)))
}

func (u *Updater) autoInstallUpdatesSuitesWriteCallback(pw *dbusutil.PropertyWrite) *dbus.Error {
	return dbusutil.ToError(u.config.SetAutoInstallUpdateType(system.UpdateType(pw.Value.(uint64))))
}

func (u *Updater) getIdleDownloadEnabled() bool {
	u.PropsMu.RLock()
	defer u.PropsMu.RUnlock()
	return u.idleDownloadConfigObj.IdleDownloadEnabled
}

// initIdleDownloadConfig initializes the idle download configuration
func (u *Updater) initIdleDownloadConfig() error {
	u.PropsMu.RLock()
	idleDownloadConfig := u.idleDownloadConfigObj
	u.PropsMu.RUnlock()

	err := u.applyIdleDownloadConfigImmediately(idleDownloadConfig, time.Now())
	if err != nil {
		return fmt.Errorf("failed to apply idle download config immediately: %w", err)
	}
	return nil
}

// writeTimerFile writes a systemd timer file to the specified path.
// It returns true if the timer file is changed, false otherwise.
func writeTimerFile(desc, hourMinute, unit string) (bool, error) {
	// Validate and parse the time format (HH:MM)
	_, err := time.Parse(autoDownloadTimeLayout, hourMinute)
	if err != nil {
		return false, fmt.Errorf("failed to parse time %q: %w",
			hourMinute, err)
	}

	// Validate unit parameter
	if unit == "" {
		return false, errors.New("unit is empty")
	}

	filePath := filepath.Join("/etc/systemd/system", unit)

	// Get current file hash to check if content has changed
	currentHash, err := getFileSha256(filePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warningf("Failed to get file sha256 of %s: %v", filePath, err)
		}
	}

	unit = strings.TrimSpace(unit)
	serviceUnit := strings.TrimSuffix(unit, ".timer") + ".service"

	// Define systemd timer unit template
	template := `[Unit]
Description=%s

[Timer]
OnCalendar=*-*-* %s:00
Unit=%s

[Install]
WantedBy=timers.target
`
	data := fmt.Sprintf(template, desc, hourMinute, serviceUnit)
	newHash := getContentSha256(data)

	// Check if content has changed to avoid unnecessary writes
	if currentHash == newHash {
		return false, nil
	}

	// Write the timer file to disk
	err = os.WriteFile(filePath, []byte(data), 0644)
	if err != nil {
		return false, fmt.Errorf("failed to write timer file: %w", err)
	}
	return true, nil
}

// applyIdleDownloadConfigImmediately applies the idle download configuration immediately based on current time.
// This function makes the configuration take effect immediately by checking if the current time is within
// the download time period and starting or aborting auto download accordingly.
func (u *Updater) applyIdleDownloadConfigImmediately(idleDownloadConfig idleDownloadConfig, now time.Time) error {
	if !idleDownloadConfig.IdleDownloadEnabled {
		// If auto download is disabled, abort the download
		u.manager.handleAbortAutoDownload()
		return nil
	}
	// Parse auto download time range
	tr, err := parseAutoDownloadRange(idleDownloadConfig, now)
	if err != nil {
		return fmt.Errorf("failed to parse auto download range: %w", err)
	}
	logger.Debug("Idle download time range: ", tr)
	if tr.Contains(now) {
		// Current time is within download period, start download
		u.manager.handleAutoDownload()
	} else {
		// Current time is outside download period, abort download
		u.manager.handleAbortAutoDownload()
	}
	return nil
}

const (
	autoDownloadTimer      = "lastore-auto-download.timer"
	abortAutoDownloadTimer = "lastore-abort-auto-download.timer"
)

// applyIdleDownloadConfig applies the idle download configuration by creating and managing systemd timer units.
// It creates timer files for auto download and abort auto download, then enables or disables them based on the configuration.
func (u *Updater) applyIdleDownloadConfig(idleDownloadConfig idleDownloadConfig, now time.Time, isStartup bool) error {
	if !isStartup {
		err := u.applyIdleDownloadConfigImmediately(idleDownloadConfig, now)
		if err != nil {
			return fmt.Errorf("failed to apply idle download config immediately: %w", err)
		}
	}

	needReload := false
	defer func() {
		// Reload systemd daemon
		if needReload {
			logger.Debug("Reload systemd daemon")
			if err := u.systemdManager.Reload(0); err != nil {
				logger.Warningf("Failed to reload systemd daemon: %v", err)
			}
		}
	}()

	// Setup timer files
	changed, err := writeTimerFile("Auto download every day", idleDownloadConfig.BeginTime, autoDownloadTimer)
	if err != nil {
		return fmt.Errorf("failed to write timer file %s: %w", autoDownloadTimer, err)
	}
	if changed {
		needReload = true
		logger.Debug("Auto download timer file changed")
	}

	changed, err = writeTimerFile("Abort auto download every day", idleDownloadConfig.EndTime, abortAutoDownloadTimer)
	if err != nil {
		return fmt.Errorf("failed to write timer file %s: %w", abortAutoDownloadTimer, err)
	}
	if changed {
		needReload = true
		logger.Debug("Abort auto download timer file changed")
	}

	// Enable or disable timer units
	units := []string{autoDownloadTimer, abortAutoDownloadTimer}
	if idleDownloadConfig.IdleDownloadEnabled {
		changed, err := u.enableAndStartTimerUnits(units)
		if err != nil {
			return fmt.Errorf("failed to enable and start timer units: %w", err)
		}
		if changed {
			needReload = true
			logger.Debug("Enable auto download timer units changed")
		}
	} else {
		changed, err := u.disableAndStopTimerUnits(units)
		if err != nil {
			return fmt.Errorf("failed to disable and stop timer units: %w", err)
		}
		if changed {
			needReload = true
			logger.Debug("Disable auto download timer units changed")
		}
	}

	return nil
}

// enableAndStartTimerUnits enables and starts the timer units for idle download
func (u *Updater) enableAndStartTimerUnits(units []string) (bool, error) {

	// enable timer units
	_, changes, err := u.systemdManager.EnableUnitFiles(0, units, false, true)
	if err != nil {
		return false, fmt.Errorf("enable idle download timer units err: %w", err)
	}
	// start timer units
	for _, unit := range units {
		_, err = u.systemdManager.StartUnit(0, unit, "replace")
		if err != nil {
			return false, fmt.Errorf("failed to start unit %s: %w", unit, err)
		}
	}

	return len(changes) > 0, nil
}

// disableAndStopTimerUnits disables and stops the timer units for idle download
func (u *Updater) disableAndStopTimerUnits(units []string) (bool, error) {
	// disable timer units
	changes, err := u.systemdManager.DisableUnitFiles(0, units, false)
	if err != nil {
		return false, fmt.Errorf("disable idle download timer units err: %w", err)
	}
	// stop timer units
	for _, unit := range units {
		_, err = u.systemdManager.StopUnit(0, unit, "replace")
		if err != nil {
			return false, fmt.Errorf("failed to stop unit %s: %w", unit, err)
		}
	}

	return len(changes) > 0, nil
}

func (u *Updater) getUpdatablePackagesByType(updateType system.UpdateType) []string {
	u.PropsMu.RLock()
	defer u.PropsMu.RUnlock()
	var updatableApps []string
	for _, t := range system.AllInstallUpdateType() {
		if updateType&t != 0 {
			packages := u.ClassifiedUpdatablePackages[t.JobType()]
			if len(packages) > 0 {
				updatableApps = append(updatableApps, packages...)
			}
		}
	}
	return updatableApps
}

func (u *Updater) GetLimitConfig() (bool, string) {
	return u.downloadSpeedLimitConfigObj.DownloadSpeedLimitEnabled, u.downloadSpeedLimitConfigObj.LimitSpeed
}

func (u *Updater) getP2PUnit() (systemd1.Unit, error) {
	p2pPath, err := u.systemdManager.GetUnit(0, p2pService)
	if err != nil {
		return nil, err
	}
	unit, err := systemd1.NewUnit(u.service.Conn(), p2pPath)
	if err != nil {
		return nil, err
	}
	return unit, nil
}

func (u *Updater) dealSetP2PUpdateEnable(enable bool) error {
	if !u.p2PUpdateSupport {
		return fmt.Errorf("unsupport p2p update")
	}
	if u.p2PUpdateEnable == enable {
		return nil
	}
	files := []string{p2pService}
	if enable {
		_, _, err := u.systemdManager.EnableUnitFiles(0, files, false, true)
		if err != nil {
			return fmt.Errorf("enable p2p UnitFile err:%v", err)
		}
		_, err = u.systemdManager.StartUnit(0, p2pService, "replace")
		if err != nil {
			return fmt.Errorf("p2p StartUnit err:%v", err)
		}
	} else {
		_, err := u.systemdManager.DisableUnitFiles(0, files, false)
		if err != nil {
			return fmt.Errorf("disable p2p UnitFile err:%v", err)
		}
		_, err = u.systemdManager.StopUnit(0, p2pService, "replace")
		if err != nil {
			return fmt.Errorf("p2p StopUnit err:%v", err)
		}
	}
	u.setPropP2PUpdateEnable(enable)
	return nil
}
