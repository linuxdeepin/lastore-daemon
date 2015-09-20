package main

import (
	"internal/system/apt"
	"log"
	"os"
	"pkg.deepin.io/lib"
	"pkg.deepin.io/lib/dbus"
)

func main() {
	os.Setenv("PATH", "/usr/bin/:/bin:/sbin")
	if !lib.UniqueOnSystem("org.deepin.lastore") {
		return
	}

	os.MkdirAll("/dev/shm/cache/archives", 0755)
	b := apt.NewAPTProxy()
	m := NewManager(b)

	err := dbus.InstallOnSystem(m)
	if err != nil {
		log.Fatal("StartFailed:", err)
		return
	}
	dbus.DealWithUnhandledMessage()

	if err := dbus.Wait(); err != nil {
		log.Fatal("DBus Error:", err)
	}
}
