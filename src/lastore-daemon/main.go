// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"time"

	"internal/system"
	"internal/system/apt"
	"internal/utils"

	"github.com/linuxdeepin/dde-api/inhibit_hint"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/log"
)

const (
	dbusServiceName = "com.deepin.lastore"
)
const (
	etcDir               = "/etc"
	osVersionFileName    = "os-version"
	aptConfDir           = "/etc/apt/apt.conf.d"
	tokenConfFileName    = "99lastore-token.conf" // #nosec G101
	securityConfFileName = "99security.conf"
)

func Tr(text string) string {
	return text
}

var logger = log.NewLogger("lastore/lastore-daemon")

//go:generate dbusutil-gen -type Updater,Job,Manager -output dbusutil.go -import internal/system,github.com/godbus/dbus updater.go job.go manager.go
//go:generate dbusutil-gen em -type Manager,Updater

func main() {
	flag.Parse()
	service, err := dbusutil.NewSystemService()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	logger.Info("Starting lastore-daemon")

	hasOwner, err := service.NameHasOwner(dbusServiceName)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if hasOwner {
		fmt.Println("another lastore-daemon running")
		return
	}

	_ = utils.UnsetEnv("LC_ALL")
	_ = utils.UnsetEnv("LANGUAGE")
	_ = utils.UnsetEnv("LC_MESSAGES")
	_ = utils.UnsetEnv("LANG")

	gettext.InitI18n()
	gettext.Textdomain("lastore-daemon")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		_ = os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}

	config := NewConfig(path.Join(system.VarLibDir, "config.json"))
	aptImpl := apt.New(config.systemSourceList, config.nonUnknownList)
	allowInstallPackageExecPaths = append(allowInstallPackageExecPaths, config.AllowInstallRemovePkgExecPaths...)
	allowRemovePackageExecPaths = append(allowRemovePackageExecPaths, config.AllowInstallRemovePkgExecPaths...)
	manager := NewManager(service, aptImpl, config)
	updater := NewUpdater(service, manager, config)
	manager.updater = updater
	serverObject, err := service.NewServerObject("/com/deepin/lastore", manager, updater)
	if err != nil {
		logger.Error("failed to new server manager and updater object:", err)
		return
	}

	err = serverObject.SetWriteCallback(updater, "AutoInstallUpdates", updater.autoInstallUpdatesWriteCallback)
	if err != nil {
		logger.Error("failed to set write cb for property AutoInstallUpdates:", err)
	}
	err = serverObject.SetWriteCallback(updater, "AutoInstallUpdateType", updater.autoInstallUpdatesSuitesWriteCallback)
	if err != nil {
		logger.Error("failed to set write cb for property AutoInstallUpdateType:", err)
	}
	err = serverObject.SetWriteCallback(manager, "UpdateMode", manager.updateModeWriteCallback)
	if err != nil {
		logger.Error("failed to set write cb for property UpdateMode:", err)
	}
	err = serverObject.ConnectChanged(manager, "UpdateMode", manager.afterUpdateModeChanged)
	if err != nil {
		logger.Error("failed to connect changed for property UpdateMode:", err)
	}
	manager.handleUpdateInfosChanged()
	manager.loadLastoreCache()       // object导出前将job处理完成,否则控制中心继续任务时,StartJob会出现job未导出的情况
	go manager.jobManager.Dispatch() // 导入job缓存之后，再执行job的dispatch，防止暂停任务创建时自动开始
	err = serverObject.Export()
	if err != nil {
		logger.Error("failed to export manager and updater:", err)
		return
	}
	initLastoreInhibitHint(service)
	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Error("failed to request name:", err)
		return
	}

	// Force notify changed at the first time
	manager.PropsMu.RLock()
	err = manager.emitPropChangedJobList(manager.JobList)
	if err != nil {
		logger.Warning(err)
	}
	err = manager.emitPropChangedUpgradableApps(manager.UpgradableApps)
	if err != nil {
		logger.Warning(err)
	}
	manager.PropsMu.RUnlock()
	manager.startSystemdUnit()
	logger.Info("Started service at system bus")
	autoQuitTime := 60 * time.Second
	if logger.GetLogLevel() == log.LevelDebug {
		autoQuitTime = 6000 * time.Second
	}
	service.SetAutoQuitHandler(autoQuitTime, manager.canAutoQuit)
	service.Wait()
	manager.saveLastoreCache()
}

func initLastoreInhibitHint(service *dbusutil.Service) {
	ihObj := inhibit_hint.New("lastore-daemon")
	ihObj.SetIconFunc(func(why string) string {
		switch why {
		case "Updating the system, please do not shut down or reboot now.":
			return "preferences-system"
		case "Tasks are running...":
			return "deepin-app-store"
		default:
			return "preferences-system" // TODO
		}
	})
	ihObj.SetNameFunc(func(why string) string {
		switch why {
		case "Updating the system, please do not shut down or reboot now.":
			return Tr("Control Center")
		case "Tasks are running...":
			return Tr("App Store")
		default:
			return Tr("Control Center") // TODO
		}
	})
	err := ihObj.Export(service)
	if err != nil {
		logger.Warning("failed to export inhibit hint:", err)
	}
}
