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
	la_utils "internal/utils"
	"os"

	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/utils"
)

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

	log.Info("Starting lastore-smartmirror-daemon")
	defer log.Flush()

	dbusName := "com.deepin.lastore.Smartmirror"
	hasOwner, err := service.NameHasOwner(dbusName)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if hasOwner {
		fmt.Println("another lastore-smartmirror-daemon running")
		return
	}

	utils.UnsetEnv("LC_ALL")
	utils.UnsetEnv("LANGUAGE")
	utils.UnsetEnv("LC_MESSAGES")
	utils.UnsetEnv("LANG")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}

	smartmirror := NewSmartMirror(service)
	err = service.Export("/com/deepin/lastore/Smartmirror", smartmirror)
	if err != nil {
		log.Error("failed to export manager and updater:", err)
		return
	}

	err = service.RequestName(dbusName)
	if err != nil {
		log.Error("failed to request name:", err)
		return
	}

	log.Info("Started service at system bus")

	service.Wait()
}
