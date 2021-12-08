/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
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

	b := apt.New()
	config := NewConfig(path.Join(system.VarLibDir, "config.json"))
	allowInstallPackageExecPaths = append(allowInstallPackageExecPaths, config.AllowInstallRemovePkgExecPaths...)
	allowRemovePackageExecPaths = append(allowRemovePackageExecPaths, config.AllowInstallRemovePkgExecPaths...)
	manager := NewManager(service, b, config)
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
	err = serverObject.Export()
	if err != nil {
		logger.Error("failed to export manager and updater:", err)
		return
	}
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
	err = ihObj.Export(service)
	if err != nil {
		logger.Warning("failed to export inhibit hint:", err)
	}

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

	logger.Info("Started service at system bus")
	handleUpdateInfosChanged := func() {
		manager.handleUpdateInfosChanged(false)
	}
	RegisterMonitor(handleUpdateInfosChanged, system.VarLibDir, "update_infos.json")
	RegisterMonitor(updateTokenConfigFile, etcDir, osVersionFileName)
	manager.handleUpdateInfosChanged(false)
	time.AfterFunc(60*time.Second, func() {
		updateTokenConfigFile()
	})
	service.Wait()
}

func RegisterMonitor(handler func(), dir string, paths ...string) {
	dm := system.NewDirMonitor(dir)
	err := dm.Add(func(filePath string) {
		handler()
	}, paths...)
	if err != nil {
		logger.Warningf("Can't add monitor on %s: %v\n", dir, err)
	}
	err = dm.Start()
	if err != nil {
		logger.Warningf("Can't create monitor on %s: %v\n", dir, err)
	}
}

var _tokenUpdateMu sync.Mutex

// 更新 99lastore-token.conf 文件的内容
func updateTokenConfigFile() {
	_tokenUpdateMu.Lock()
	defer _tokenUpdateMu.Unlock()
	systemInfo, err := getSystemInfo()
	tokenPath := path.Join(aptConfDir, tokenConfFileName)
	if err != nil {
		logger.Warning("failed to update 99lastore-token.conf content:", err)
	}
	var tokenSlice []string
	tokenSlice = append(tokenSlice, "a="+systemInfo.SystemName)
	tokenSlice = append(tokenSlice, "b="+systemInfo.ProductType)
	tokenSlice = append(tokenSlice, "c="+systemInfo.EditionName)
	tokenSlice = append(tokenSlice, "v="+systemInfo.Version)
	tokenSlice = append(tokenSlice, "i="+systemInfo.HardwareId)
	tokenSlice = append(tokenSlice, "m="+systemInfo.Processor)
	tokenSlice = append(tokenSlice, "ac="+systemInfo.Arch)
	tokenSlice = append(tokenSlice, "cu="+systemInfo.Custom)
	tokenSlice = append(tokenSlice, "sn="+systemInfo.SN)
	token := strings.Join(tokenSlice, ";")
	token = strings.Replace(token, "\n", "", -1)
	tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	err = ioutil.WriteFile(tokenPath, tokenContent, 0644) // #nosec G306
	if err != nil {
		logger.Warning(err)
	}
}
