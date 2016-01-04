package main

import (
	"flag"
	log "github.com/cihub/seelog"
	"internal/system"
	"internal/system/apt"
	"os"
	"path"
	"pkg.deepin.io/lib"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/utils"
)

func main() {
	flag.Parse()

	SetSeelogger(DefaultLogLevel, DefaultLogFomrat, DefaultLogOutput)

	setupLog()
	defer log.Flush()

	if !lib.UniqueOnSystem("com.deepin.lastore") {
		log.Info("Can't obtain the com.deepin.lastore")
		return
	}

	utils.UnsetEnv("LC_ALL")
	utils.UnsetEnv("LANGUAGE")
	utils.UnsetEnv("LC_MESSAGES")
	utils.UnsetEnv("LANG")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}

	b := apt.New()
	config := NewConfig(path.Join(system.VarLibDir, "config.json"))

	manager := NewManager(b, config)
	err := dbus.InstallOnSystem(manager)
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
