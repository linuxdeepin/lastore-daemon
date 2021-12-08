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
	"time"

	"internal/utils"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

//go:generate dbusutil-gen em -type SmartMirror
var logger = log.NewLogger("lastore/smartmirror")

func main() {
	runDaemon := flag.Bool("daemon", false, "run as daemon and not exit")
	flag.Parse()

	service, err := dbusutil.NewSystemService()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	logger.Info("Starting lastore-smartmirror-daemon")

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

	_ = utils.UnsetEnv("LC_ALL")
	_ = utils.UnsetEnv("LANGUAGE")
	_ = utils.UnsetEnv("LC_MESSAGES")
	_ = utils.UnsetEnv("LANG")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}

	smartmirror := newSmartMirror(service)
	err = service.Export("/com/deepin/lastore/Smartmirror", smartmirror)
	if err != nil {
		logger.Error("failed to export manager and updater:", err)
		return
	}

	err = service.RequestName(dbusName)
	if err != nil {
		logger.Error("failed to request name:", err)
		return
	}

	logger.Info("Started service at system bus")

	if *runDaemon {
		logger.Info("Run as daemon and not exist")
		service.SetAutoQuitHandler(time.Second*5, func() bool {
			return false
		})
	} else {
		service.SetAutoQuitHandler(time.Second*5, smartmirror.canQuit)
	}
	service.DelayAutoQuit()
	service.Wait()
}
