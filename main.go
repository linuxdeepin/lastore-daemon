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
	j := m.InstallPackages([]string{"google-chrome-stable"})
	dbus.InstallOnSystem(j)
	m.StartJob(j.Id)

	if err := dbus.Wait(); err != nil {
		log.Fatal("DBus Error:", err)
	}
}
