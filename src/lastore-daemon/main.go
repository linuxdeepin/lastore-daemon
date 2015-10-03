package main

import (
	"fmt"
	"internal/system/apt"
	"log"
	"os"
	"pkg.deepin.io/lib"
	"pkg.deepin.io/lib/dbus"
)

func main() {
	os.Setenv("PATH", "/usr/bin/:/bin:/sbin")
	if !lib.UniqueOnSystem("org.deepin.lastore") {
		fmt.Println("Can't obtain the org.deepin.lastore")
		return
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	b := apt.New()
	m := NewManager(b)

	err := dbus.InstallOnSystem(m)
	if err != nil {
		fmt.Println("Start failed:", err)
		return
	}
	fmt.Println("Started service at system bus")

	dbus.DealWithUnhandledMessage()

	if err := dbus.Wait(); err != nil {
		fmt.Println("DBus Error:", err)
	}
}
