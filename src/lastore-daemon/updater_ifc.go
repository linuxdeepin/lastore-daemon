// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/polkit"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/lastore-daemon/src/internal/ratelimit"
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
	u.PropsMu.RLock()
	idleDownloadConfigObj := u.idleDownloadConfigObj
	u.PropsMu.RUnlock()
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

// SetIdleDownloadConfig is used to set the idle download config
func (u *Updater) SetIdleDownloadConfig(idleConfig string) *dbus.Error {
	var idleDownloadConfigObj idleDownloadConfig
	err := json.Unmarshal([]byte(idleConfig), &idleDownloadConfigObj)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	// This dbus method may be called concurrently, need to add lock to ensure concurrent data safety
	u.PropsMu.Lock()
	u.idleDownloadConfigObj = idleDownloadConfigObj
	if u.setIdleDownloadConfigTimer == nil {
		u.setIdleDownloadConfigTimer = time.AfterFunc(time.Second, func() {
			u.PropsMu.RLock()
			idleDownloadConfig := u.idleDownloadConfigObj
			u.PropsMu.RUnlock()
			config, err := json.Marshal(idleDownloadConfig)
			if err != nil {
				logger.Warning(err)
				return
			}
			changed := u.setPropIdleDownloadConfig(string(config))
			if changed {
				err = u.config.SetIdleDownloadConfig(string(config))
				if err != nil {
					logger.Warning(err)
					return
				}
				err = u.applyIdleDownloadConfig(idleDownloadConfig, time.Now(), false)
				if err != nil {
					logger.Warning(err)
					return
				}
			}
		})
	} else {
		// Stop the timer before resetting to avoid concurrent scheduling races
		_ = u.setIdleDownloadConfigTimer.Stop()
		u.setIdleDownloadConfigTimer.Reset(time.Second)
		logger.Info("reset idle timer")
	}
	u.PropsMu.Unlock()
	return nil
}

func (u *Updater) SetDownloadSpeedLimit(limitConfig string) *dbus.Error {
	if err := json.Unmarshal([]byte(limitConfig), &u.downloadSpeedLimitConfigObj); err != nil {
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
				logger.Infof("set changed speed limit: %v", limitConfig)
				// When IsOnlineSpeedLimit is false, it means manual speed limit is set.
				// Save to both configs to ensure the value persists after daemon restart.
				if strings.Contains(limitConfig, "IsOnlineSpeedLimit") && !u.downloadSpeedLimitConfigObj.IsOnlineSpeedLimit {
					err := u.config.SetLocalDownloadSpeedLimitConfig(string(config))
					if err != nil {
						logger.Warning(err)
						return
					}
				} else {
					err := u.config.SetDownloadSpeedLimitConfig(string(config))
					if err != nil {
						logger.Warning(err)
						return
					}
				}
				u.manager.ChangePrepareDistUpgradeJobOption()
			}
			logger.Info("update limit config")
		})
	} else {
		u.setDownloadSpeedLimitTimer.Reset(time.Second)
		logger.Info("reset limit timer")
	}
	return nil
}

func (u *Updater) SetP2PUpdateEnable(sender dbus.Sender, enable bool) *dbus.Error {
	err := polkit.CheckAuth(polkitActionChangeUpgradeDelivery, string(sender), nil)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	err = u.config.SetUpgradeDeliveryEnabled(enable)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	u.setPropP2PUpdateEnable(enable)
	return nil
}

func (u *Updater) CleanTransmissionFiles(sender dbus.Sender) *dbus.Error {
	err := polkit.CheckAuth(polkitActionChangeUpgradeDelivery, string(sender), nil)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	object := sysBus.Object("org.deepin.upgradedelivery", "/org/deepin/upgradedelivery")
	err = object.Call("org.deepin.upgradedelivery.Clear", 0).Store()
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (u *Updater) SetDeliveryDownloadSpeedLimit(limitConfig string) *dbus.Error {
	var speedLimitConfig speedLimitConfig
	if err := json.Unmarshal([]byte(limitConfig), &speedLimitConfig); err != nil {
		return dbusutil.ToError(err)
	}

	rateLimit := -1
	if speedLimitConfig.SpeedLimitEnabled {
		limitRate, err := strconv.ParseInt(speedLimitConfig.LimitSpeed, 10, 64)
		if err != nil {
			return dbusutil.ToError(err)
		}
		rateLimit = int(limitRate)
	}

	if err := ratelimit.SetIPFSDownloadRateLimit(rateLimit); err != nil {
		return dbusutil.ToError(err)
	}

	rateInfo := ratelimit.RateInfo{
		LimitType:   ratelimit.RateLimitTypeLocal,
		LimitRate:   int64(rateLimit),
		CurrentRate: int64(rateLimit),
	}
	rateInfoData, err := json.Marshal(rateInfo)
	if err != nil {
		return dbusutil.ToError(err)
	}
	if err := u.config.SetDeliveryLocalDownloadGlobalLimit(string(rateInfoData)); err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}

func (u *Updater) SetDeliveryUploadSpeedLimit(limitConfig string) *dbus.Error {
	var speedLimitConfig speedLimitConfig
	if err := json.Unmarshal([]byte(limitConfig), &speedLimitConfig); err != nil {
		return dbusutil.ToError(err)
	}

	rateLimit := -1
	if speedLimitConfig.SpeedLimitEnabled {
		limitRate, err := strconv.ParseInt(speedLimitConfig.LimitSpeed, 10, 64)
		if err != nil {
			return dbusutil.ToError(err)
		}
		rateLimit = int(limitRate)
	}

	if err := ratelimit.SetIPFSUploadRateLimit(rateLimit); err != nil {
		return dbusutil.ToError(err)
	}

	rateInfo := ratelimit.RateInfo{
		LimitType:   ratelimit.RateLimitTypeLocal,
		LimitRate:   int64(rateLimit),
		CurrentRate: int64(rateLimit),
	}
	rateInfoData, err := json.Marshal(rateInfo)
	if err != nil {
		return dbusutil.ToError(err)
	}
	if err := u.config.SetDeliveryLocalUploadGlobalLimit(string(rateInfoData)); err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}
