package main

import (
	"./system/apt"
	"log"
	"os"
	"pkg.deepin.io/lib/dbus"
)

func main() {
	b := apt.NewAPTProxy()
	m := NewManager(b)
	dbus.InstallOnSystem(m)

	os.MkdirAll("/dev/shm/cache/archives", 0755)
	j, err := m.InstallPackages("google-chrome-stable")
	if err != nil {
		panic(err)
	}
	dbus.InstallOnSystem(j)

	err = m.StartJob(j.Id)
	if err != nil {
		log.Fatal("StartFailed:", err)
		return
	}
	if err := dbus.Wait(); err != nil {
		log.Fatal("DBus Error:", err)
	}
}
