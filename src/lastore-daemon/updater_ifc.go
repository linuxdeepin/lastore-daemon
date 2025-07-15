// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (u *Updater) GetCheckIntervalAndTime() (interval float64, checkTime string, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	interval = u.config.CheckInterval.Hours()
	checkTime = u.config.LastCheckTime.Format("2006-01-02 15:04:05.999999999 -0700 MST")
	return
}

func (u *Updater) SetAutoCheckUpdates(enable bool) *dbus.Error {
	u.service.DelayAutoQuit()
	if u.AutoCheckUpdates == enable {
		return nil
	}

	// save the config to disk
	err := u.config.SetAutoCheckUpdates(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	u.AutoCheckUpdates = enable
	_ = u.emitPropChangedAutoCheckUpdates(enable)
	return nil
}

func (u *Updater) SetAutoDownloadUpdates(enable bool) *dbus.Error {
	u.service.DelayAutoQuit()
	if u.AutoDownloadUpdates == enable {
		return nil
	}
	u.PropsMu.Lock()
	idleDownloadConfigObj := u.idleDownloadConfigObj
	u.PropsMu.Unlock()
	if !enable && idleDownloadConfigObj.IdleDownloadEnabled == true {
		idleDownloadConfigObj.IdleDownloadEnabled = false
		idleDownloadByte, err := json.Marshal(idleDownloadConfigObj)
		if err != nil {
			logger.Warning(err)
		} else {
			err = u.SetIdleDownloadConfig(string(idleDownloadByte))
			if err != nil {
				logger.Warning(err)
			}
		}
	}
	// save the config to disk
	err := u.config.SetAutoDownloadUpdates(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	u.AutoDownloadUpdates = enable
	_ = u.emitPropChangedAutoDownloadUpdates(enable)
	return nil
}

func (u *Updater) SetUpdateNotify(enable bool) *dbus.Error {
	u.service.DelayAutoQuit()
	if u.UpdateNotify == enable {
		return nil
	}
	err := u.config.SetUpdateNotify(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}
	u.UpdateNotify = enable

	_ = u.emitPropChangedUpdateNotify(enable)

	return nil
}

func (u *Updater) SetIdleDownloadConfig(idleConfig string) *dbus.Error {
	err := json.Unmarshal([]byte(idleConfig), &u.idleDownloadConfigObj)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	if u.setIdleDownloadConfigTimer == nil {
		u.setIdleDownloadConfigTimer = time.AfterFunc(time.Second, func() {
			config, err := json.Marshal(u.idleDownloadConfigObj)
			if err != nil {
				logger.Warning(err)
				return
			}
			changed := u.setPropIdleDownloadConfig(string(config))
			if changed {
				u.manager.resetIdleDownload = true
				err = u.config.SetIdleDownloadConfig(string(config))
				if err != nil {
					logger.Warning(err)
					return
				}
				err = u.manager.updateAutoDownloadTimer()
				if err != nil {
					logger.Warning(err)
					return
				}
			}
		})
	} else {
		u.setIdleDownloadConfigTimer.Reset(time.Second)
		logger.Info("reset idle timer")
	}
	return nil
}

func (u *Updater) SetDownloadSpeedLimit(limitConfig string) *dbus.Error {
	err := json.Unmarshal([]byte(limitConfig), &u.downloadSpeedLimitConfigObj)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if u.setDownloadSpeedLimitTimer == nil {
		u.setDownloadSpeedLimitTimer = time.AfterFunc(time.Second, func() {
			config, err := json.Marshal(u.downloadSpeedLimitConfigObj)
			if err != nil {
				logger.Warning(err)
				return
			}
			changed := u.setPropDownloadSpeedLimitConfig(string(config))
			if changed {
				logger.Info("speed limit: ", u.downloadSpeedLimitConfigObj)
				err := u.config.SetDownloadSpeedLimitConfig(string(config))
				if err != nil {
					logger.Warning(err)
					return
				}
				u.manager.ChangePrepareDistUpgradeJobOption()
			}
			logger.Info("update limit config")
			return
		})
	} else {
		u.setDownloadSpeedLimitTimer.Reset(time.Second)
		logger.Info("reset limit timer")
	}
	return nil
}

func (u *Updater) SetP2PUpdateEnable(enable bool) *dbus.Error {
	err := u.dealSetP2PUpdateEnable(enable)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}
