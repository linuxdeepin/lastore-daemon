package login1

import "pkg.deepin.io/lib/dbus"

var __conn *dbus.Conn = nil

func getBus() *dbus.Conn {
	if __conn == nil {
		var err error
		__conn, err = dbus.SystemBus()
		if err != nil {
			panic(err)
		}
	}
	return __conn
}
