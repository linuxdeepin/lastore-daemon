// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"internal/system"
	"os"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (u *Updater) ApplicationUpdateInfos(lang string) (updateInfos []ApplicationUpdateInfo, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	iInfos := packageIconInfos()
	aInfos := applicationInfos()
	var uInfosMap system.SourceUpgradeInfoMap
	var err error
	repeatCount := 0
	for {
		if repeatCount > 5 {
			break
		}
		uInfosMap, err = u.manager.SystemUpgradeInfo()
		if os.IsNotExist(err) {
			time.Sleep(1 * time.Second)
			repeatCount++
		} else if err != nil {
			updateInfoErr, ok := err.(*system.UpdateInfoError)
			if ok {
				return nil, dbusutil.MakeErrorJSON(u, "UpdateInfoError", updateInfoErr)
			}
			return nil, dbusutil.ToError(err)
		} else {
			break
		}
	}

	for _, uInfos := range uInfosMap {
		for _, uInfo := range uInfos {
			id := uInfo.Package

			aInfo, ok := aInfos[id]
			if !ok {
				continue
			}

			info := ApplicationUpdateInfo{
				Id:             id,
				Name:           aInfo.LocaleName[lang],
				Icon:           iInfos[id],
				CurrentVersion: uInfo.CurrentVersion,
				LastVersion:    uInfo.LastVersion,
			}
			if info.Name == "" {
				info.Name = id
			}
			if info.Icon == "" {
				info.Icon = id
			}
			updateInfos = append(updateInfos, info)
		}
	}
	logger.Info("ApplicationUpdateInfos: ", updateInfos)
	return updateInfos, nil
}

func (u *Updater) GetCheckIntervalAndTime() (interval float64, checkTime string, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	interval = u.config.CheckInterval.Hours()
	checkTime = u.config.LastCheckTime.Format("2006-01-02 15:04:05.999999999 -0700 MST")
	return
}

// ListMirrorSources 返回当前支持的镜像源列表．顺序按优先级降序排
// 其中Name会根据传递进来的lang进行本地化
func (u *Updater) ListMirrorSources(lang string) (mirrorSources []LocaleMirrorSource, busErr *dbus.Error) {
	u.service.DelayAutoQuit()
	return u.listMirrorSources(lang), nil
}

func (u *Updater) RestoreSystemSource() *dbus.Error {
	u.service.DelayAutoQuit()
	err := u.restoreSystemSource()
	if err != nil {
		logger.Warning("failed to restore system source:", err)
		return dbusutil.ToError(err)
	}
	return nil
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

	// save the config to disk
	err := u.config.SetAutoDownloadUpdates(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	u.AutoDownloadUpdates = enable
	_ = u.emitPropChangedAutoDownloadUpdates(enable)
	return nil
}

// SetMirrorSource 设置用于下载软件的镜像源
func (u *Updater) SetMirrorSource(id string) *dbus.Error {
	u.service.DelayAutoQuit()
	err := u.setMirrorSource(id)
	return dbusutil.ToError(err)
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

func (u *Updater) SetIdleDownloadConfig(enable bool, beginTime, endTime string) *dbus.Error {
	changed := u.setPropIdleDownloadConfig(idleDownloadConfig{
		enable, beginTime, endTime,
	})
	if changed {
		err := u.config.SetIdleDownloadConfig(idleDownloadConfig{
			enable, beginTime, endTime,
		})
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		}

	}
	return dbusutil.ToError(u.manager.updateAutoDownloadTimer())
}
