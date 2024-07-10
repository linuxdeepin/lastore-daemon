// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	. "internal/config"
	"internal/updateplatform"
	"io/ioutil"
	"os/exec"
	"path"
	"sync"
	"time"

	"internal/system"

	"github.com/godbus/dbus"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.systemd1"
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

	OfflineInfo string

	P2PUpdateEnable  bool // p2p更新是否开启
	P2PUpdateSupport bool // 是否支持p2p更新
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
		u.P2PUpdateEnable = false
		u.P2PUpdateSupport = false
	} else {
		u.P2PUpdateEnable = false
		u.P2PUpdateSupport = true
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
					u.P2PUpdateEnable = true
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
	return ioutil.WriteFile("/etc/apt/apt.conf.d/99mirrors.conf",
		([]byte)(fmt.Sprintf("Acquire::SmartMirrors::MirrorSource %q;", url)),
		0644) // #nosec G306
}

func (u *Updater) setMirrorSource(id string) error {
	if id == "" || u.MirrorSource == id {
		return nil
	}

	found := false
	for _, m := range u.listMirrorSources("") {
		if m.Id != id {
			continue
		}
		found = true
		if m.Url == "" {
			return system.NotFoundError("empty url")
		}
		if err := SetAPTSmartMirror(m.Url); err != nil {
			logger.Warningf("SetMirrorSource(%q) failed:%v\n", id, err)
			return err
		}
	}
	if !found {
		return system.NotFoundError("invalid mirror source id")
	}
	err := u.config.SetMirrorSource(id)
	if err != nil {
		return err
	}
	u.MirrorSource = u.config.MirrorSource
	_ = u.emitPropChangedMirrorSource(u.MirrorSource)
	return nil
}

type LocaleMirrorSource struct {
	Id   string
	Url  string
	Name string
}

func (u *Updater) listMirrorSources(lang string) []LocaleMirrorSource {
	var raws []system.MirrorSource
	_ = system.DecodeJson(path.Join(system.VarLibDir, "mirrors.json"), &raws)

	makeLocaleMirror := func(lang string, m system.MirrorSource) LocaleMirrorSource {
		ms := LocaleMirrorSource{
			Id:   m.Id,
			Url:  m.Url,
			Name: m.Name,
		}
		if v, ok := m.NameLocale[lang]; ok {
			ms.Name = v
		}
		return ms
	}

	var r []LocaleMirrorSource
	for _, raw := range raws {
		if raw.Weight < 0 {
			continue
		}
		r = append(r, makeLocaleMirror(lang, raw))
	}

	return r
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

func (u *Updater) restoreSystemSource() error {
	// write backup file
	current, err := ioutil.ReadFile(aptSource)
	if err == nil {
		err = ioutil.WriteFile(aptSource+".bak", current, 0644) // #nosec G306
		if err != nil {
			logger.Warning(err)
		}
	} else {
		logger.Warning(err)
	}

	origin, err := ioutil.ReadFile(aptSourceOrigin)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(aptSource, origin, 0644) // #nosec G306
	return err
}

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

func (u *Updater) SetOfflineInfo(res OfflineCheckResult) error {
	content, err := json.Marshal(res)
	if err != nil {
		u.setPropOfflineInfo("")
		logger.Warning(err)
		return err
	}
	u.setPropOfflineInfo(string(content))
	return nil
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

func (u *Updater) setP2PUpdateEnable(enable bool) error {
	if !u.P2PUpdateSupport {
		return fmt.Errorf("unsupport p2p update")
	}
	if u.P2PUpdateEnable == enable {
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
