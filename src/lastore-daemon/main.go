package main

import (
	"internal/system/apt"
	"log"
	"os"
	"pkg.deepin.io/lib/dbus"
)

func main() {
	os.MkdirAll("/dev/shm/cache/archives", 0755)
	b := apt.NewAPTProxy()
	m := NewManager(b)

	err := dbus.InstallOnSystem(m)
	if err != nil {
		log.Fatal("StartFailed:", err)
		return
	}
	if err := dbus.Wait(); err != nil {
		log.Fatal("DBus Error:", err)
	}
}
