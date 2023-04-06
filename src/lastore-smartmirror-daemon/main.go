// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"

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

	dbusName := "org.deepin.dde.Lastore1.Smartmirror"
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
	err = service.Export("/org/deepin/dde/Lastore1/Smartmirror", smartmirror)
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
