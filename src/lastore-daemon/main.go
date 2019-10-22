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
	"os"
	"path"

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

func Tr(text string) string {
	return text
}

//go:generate dbusutil-gen -type Updater,Job,Manager -output dbusutil.go -import internal/system,pkg.deepin.io/lib/dbus1 updater.go job.go manager.go

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

	utils.UnsetEnv("LC_ALL")
	utils.UnsetEnv("LANGUAGE")
	utils.UnsetEnv("LC_MESSAGES")
	utils.UnsetEnv("LANG")

	gettext.InitI18n()
	gettext.Textdomain("lastore-daemon")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}

	b := apt.New()
	config := NewConfig(path.Join(system.VarLibDir, "config.json"))

	manager := NewManager(service, b, config)
	updater := NewUpdater(service, manager, config)
	manager.updater = updater
	err = service.Export("/com/deepin/lastore", manager, updater)
	if err != nil {
		log.Error("failed to export manager and updater:", err)
		return
	}

	ihObj := inhibit_hint.New("lastore-daemon")
	ihObj.SetIcon("dde-control-center")
	ihObj.SetName(Tr("Control Center"))
	err = ihObj.Export(service)
	if err != nil {
		log.Warn("failed to export inhibit hint:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		log.Error("failed to request name:", err)
		return
	}

	log.Info("Started service at system bus")

	RegisterMonitor(manager.handleUpdateInfosChanged, "update_infos.json")
	manager.handleUpdateInfosChanged()
	service.Wait()
}

func RegisterMonitor(handler func(), paths ...string) {
	dm := system.NewDirMonitor(system.VarLibDir)

	dm.Add(func(fpath string) {
		handler()
	}, paths...)

	err := dm.Start()
	if err != nil {
		log.Warnf("Can't create inotify on %s: %v\n", system.VarLibDir, err)
	}
}
