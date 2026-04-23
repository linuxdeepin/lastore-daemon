// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"strconv"
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
	var limitConfigObj downloadSpeedLimitConfig
	if err := json.Unmarshal([]byte(limitConfig), &limitConfigObj); err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if u.downloadSpeedLimitConfigObj.IsOnlineSpeedLimit {
		// 在线限速优先级更高，如果通过dbus设置了一个本地限速，则不应该把在线限速的配置覆盖掉
		return nil
	}
	// dbus设置数据本地限速，强制将IsOnlineSpeedLimit设置为false
	limitConfigObj.IsOnlineSpeedLimit = false
	if err := u.setDownloadSpeedLimit(limitConfigObj); err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (u *Updater) setDownloadSpeedLimit(limitConfigObj downloadSpeedLimitConfig) error {
	u.PropsMu.Lock()

	logger.Infof("set download limit %+v --> %+v", u.downloadSpeedLimitConfigObj, limitConfigObj)
	u.downloadSpeedLimitConfigObj = limitConfigObj
	config, err := json.Marshal(limitConfigObj)
	if err != nil {
		logger.Warning(err)
		u.PropsMu.Unlock()
		return err
	}
	changed := u.setPropDownloadSpeedLimitConfig(string(config))
	if !changed {
		u.PropsMu.Unlock()
		return nil
	}

	if u.setDownloadSpeedLimitTimer != nil {
		u.setDownloadSpeedLimitTimer.Stop()
		logger.Info("reset limit timer")
	}
	configStr := string(config)
	u.setDownloadSpeedLimitTimer = time.AfterFunc(time.Second, func() {
		u.PropsMu.Lock()
		if !u.downloadSpeedLimitConfigObj.IsOnlineSpeedLimit {
			logger.Info("update local speed limit config")
			if err := u.config.SetLocalDownloadSpeedLimitConfig(configStr); err != nil {
				logger.Warning(err)
			}
		} else {
			logger.Info("update online speed limit config")
			if err := u.config.SetDownloadSpeedLimitConfig(configStr); err != nil {
				logger.Warning(err)
			}
		}
		u.manager.ChangePrepareDistUpgradeJobOption()
		u.PropsMu.Unlock()
	})

	u.PropsMu.Unlock()
	return nil
}

func (u *Updater) SetP2PUpdateEnable(sender dbus.Sender, enable bool) *dbus.Error {
	var action string
	if enable {
		action = polkitActionEnableUpgradeDelivery
	} else {
		action = polkitActionDisableUpgradeDelivery
	}
	err := polkit.CheckAuth(action, string(sender), nil)
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

	var rateInfo ratelimit.RateInfo
	if speedLimitConfig.SpeedLimitEnabled {
		limitRate, err := strconv.ParseInt(speedLimitConfig.LimitSpeed, 10, 64)
		if err != nil {
			return dbusutil.ToError(err)
		}
		rateLimit := int(limitRate)
		if rateLimit*1024 < ratelimit.MinRateLimit || rateLimit*1024 > ratelimit.MaxRateLimit {
			rateLimit = int(ratelimit.DefaultRateLimit) / 1024
		}
		if err := ratelimit.SetIPFSDownloadRateLimit(rateLimit); err != nil {
			return dbusutil.ToError(err)
		}
		rateInfo = ratelimit.RateInfo{
			LimitType:   ratelimit.RateLimitTypeLocal,
			LimitRate:   int64(rateLimit) * 1024,
			CurrentRate: int64(rateLimit) * 1024,
		}
	} else {
		if err := ratelimit.SetIPFSDownloadRateLimit(-1); err != nil {
			return dbusutil.ToError(err)
		}
		rateInfo = ratelimit.RateInfo{
			LimitType:   ratelimit.RateLimitTypeNo,
			LimitRate:   ratelimit.DefaultRateLimit,
			CurrentRate: ratelimit.DefaultRateLimit,
		}
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

	var rateInfo ratelimit.RateInfo
	if speedLimitConfig.SpeedLimitEnabled {
		limitRate, err := strconv.ParseInt(speedLimitConfig.LimitSpeed, 10, 64)
		if err != nil {
			return dbusutil.ToError(err)
		}
		rateLimit := int(limitRate)
		if rateLimit*1024 < ratelimit.MinRateLimit || rateLimit*1024 > ratelimit.MaxRateLimit {
			rateLimit = int(ratelimit.DefaultRateLimit) / 1024
		}
		if err := ratelimit.SetIPFSUploadRateLimit(rateLimit); err != nil {
			return dbusutil.ToError(err)
		}
		rateInfo = ratelimit.RateInfo{
			LimitType:   ratelimit.RateLimitTypeLocal,
			LimitRate:   int64(rateLimit) * 1024,
			CurrentRate: int64(rateLimit) * 1024,
		}
	} else {
		if err := ratelimit.SetIPFSUploadRateLimit(-1); err != nil {
			return dbusutil.ToError(err)
		}
		rateInfo = ratelimit.RateInfo{
			LimitType:   ratelimit.RateLimitTypeNo,
			LimitRate:   ratelimit.DefaultRateLimit,
			CurrentRate: ratelimit.DefaultRateLimit,
		}
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
