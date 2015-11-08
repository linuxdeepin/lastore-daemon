package main

import "pkg.deepin.io/lib/dbus"
import "syscall"
import "fmt"

import "dbus/org/freedesktop/login1"
import "dbus/com/deepin/lastore"

import "pkg.deepin.io/lib/gettext"

func PreventShutdownOnSystemChanging() error {
	m, err := lastore.NewManager("com.deepin.lastore", "/com/deepin/lastore")
	if err != nil {
		return err
	}
	handleSystemChaning := func() func() {
		var inhibitFd dbus.UnixFD = -1
		return func() {
			onChanging := m.SystemOnChanging.Get()

			if onChanging && inhibitFd == -1 {
				inhibitFd, err = Inhibitor("shutdown", gettext.Tr("Deepin Store"),
					gettext.Tr("System is updating, please wait and do not unplug your machine."))
				fmt.Println("Prevent shutdown...:", inhibitFd, err)
			}
			if !onChanging && inhibitFd != -1 {
				err = syscall.Close(int(inhibitFd))
				fmt.Println("Enable shutdown...", inhibitFd, err)
				inhibitFd = -1
			}
		}
	}()
	handleSystemChaning()
	m.SystemOnChanging.ConnectChanged(handleSystemChaning)

	return nil
}

func Inhibitor(what, who, why string) (dbus.UnixFD, error) {
	m, err := login1.NewManager("org.freedesktop.login1", "/org/freedesktop/login1")
	if err != nil {
		return -1, err
	}
	defer login1.DestroyManager(m)
	return m.Inhibit(what, who, why, "block")
}

func main() {
	gettext.InitI18n()
	gettext.Textdomain("lastore-daemon")

	err := PreventShutdownOnSystemChanging()
	if err != nil {
		fmt.Println("Failed Handle OnSystemChaning:", err)
	}

	if err = dbus.Wait(); err != nil {
	}
}
