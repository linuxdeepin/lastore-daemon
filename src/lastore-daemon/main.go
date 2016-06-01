/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

import (
	"flag"
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"internal/system/apt"
	"os"
	"path"
	"pkg.deepin.io/lib"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/utils"
)

func main() {
	flag.Parse()

	err := SetSeelogger(DefaultLogLevel, DefaultLogFomrat, DefaultLogOutput)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	log.Info("Starting lastore-daemon")
	defer log.Flush()

	if !lib.UniqueOnSystem("com.deepin.lastore") {
		log.Infof("Can't obtain the com.deepin.lastore\n")
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

	manager := NewManager(b, config)
	err = dbus.InstallOnSystem(manager)
	if err != nil {
		log.Error("Install manager on system bus :", err)
		return
	}
	log.Info("Started service at system bus")

	updater := NewUpdater(b, config)
	err = dbus.InstallOnSystem(updater)
	if err != nil {
		log.Error("Start failed:", err)
		return
	}

	dbus.DealWithUnhandledMessage()
	if err := dbus.Wait(); err != nil {
		log.Warn("DBus Error:", err)
	}
}
