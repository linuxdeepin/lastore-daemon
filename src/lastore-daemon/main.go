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
	la_utils "internal/utils"

	log "github.com/cihub/seelog"
	"pkg.deepin.io/dde/api/inhibit_hint"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/utils"
)

const (
	dbusServiceName = "com.deepin.lastore"
)
const (
	etcDir            = "/etc"
	osVersionFileName = "os-version"
	aptConfDir        = "/etc/apt/apt.conf.d"
	tokenConfFileName = "99lastore-token.conf"
)

func Tr(text string) string {
	return text
}

//go:generate dbusutil-gen -type Updater,Job,Manager -output dbusutil.go -import internal/system,github.com/godbus/dbus updater.go job.go manager.go
//go:generate dbusutil-gen em -type Manager,Updater

func main() {
	flag.Parse()

	err := la_utils.SetSeelogger(la_utils.DefaultLogLevel, la_utils.DefaultLogFormat, la_utils.DefaultLogOutput)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	service, err := dbusutil.NewSystemService()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	log.Info("Starting lastore-daemon")
	defer log.Flush()

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
		_ = log.Error("failed to new server manager and updater object:", err)
		return
	}

	err = serverObject.SetWriteCallback(manager, "UpdateMode", manager.updateModeWriteCallback)
	if err != nil {
		_ = log.Error("failed to set write cb for property UpdateMode:", err)
	}
	err = serverObject.Export()
	if err != nil {
		_ = log.Error("failed to export manager and updater:", err)
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
		_ = log.Warn("failed to export inhibit hint:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		_ = log.Error("failed to request name:", err)
		return
	}

	// Force notify changed at the first time
	manager.PropsMu.RLock()
	err = manager.emitPropChangedJobList(manager.JobList)
	if err != nil {
		_ = log.Warn(err)
	}
	err = manager.emitPropChangedUpgradableApps(manager.UpgradableApps)
	if err != nil {
		_ = log.Warn(err)
	}
	manager.PropsMu.RUnlock()

	log.Info("Started service at system bus")
	RegisterMonitor(manager.handleUpdateInfosChanged, system.VarLibDir, "update_infos.json")
	RegisterMonitor(updateTokenConfigFile, etcDir, osVersionFileName)
	manager.handleUpdateInfosChanged()
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
		_ = log.Warnf("Can't add monitor on %s: %v\n", dir, err)
	}
	err = dm.Start()
	if err != nil {
		_ = log.Warnf("Can't create monitor on %s: %v\n", dir, err)
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
		_ = log.Warn("failed to update 99lastore-token.conf content:", err)
		return
	}
	var tokenSlice []string
	tokenSlice = append(tokenSlice, "a="+systemInfo.SystemName)
	tokenSlice = append(tokenSlice, "b="+systemInfo.ProductType)
	tokenSlice = append(tokenSlice, "c="+systemInfo.EditionName)
	tokenSlice = append(tokenSlice, "v="+systemInfo.Version)
	tokenSlice = append(tokenSlice, "i="+systemInfo.HardwareId)
	tokenSlice = append(tokenSlice, "m="+systemInfo.Processor)
	token := strings.Join(tokenSlice, ";")
	tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	err = ioutil.WriteFile(tokenPath, tokenContent, 0644)
	if err != nil {
		_ = log.Warn(err)
	}
}
