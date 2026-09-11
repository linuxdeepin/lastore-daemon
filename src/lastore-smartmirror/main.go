// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

/*
This tools implement SmartMirrorDetector.

Run with `detector url OfficialSite MirrorSite`
The result will be the right url.
*/
package main

import (
	"fmt"
	"os"

	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"

	"github.com/godbus/dbus/v5"
)

// Injectable seams so tests can exercise the early-exit branches of main
// without a real smartmirror daemon or terminating the test process.
var (
	osExit      = os.Exit
	systemBusFn = dbus.SystemBus
)

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage %s URL OfficialHost MirrorHost\n", os.Args[0])
		osExit(-1)
	}

	rawURL := os.Args[1]
	officialHost := os.Args[2]
	mirrorHost := os.Args[3]

	url := ""
	sysBus, err := systemBusFn()
	if err != nil {
		fmt.Print(rawURL)
		return
	}
	smartmirror := sysBus.Object("org.deepin.dde.Lastore1.Smartmirror", "/org/deepin/dde/Lastore1/Smartmirror")
	err = smartmirror.Call("org.deepin.dde.Lastore1.Smartmirror.Query", 0, rawURL, officialHost, mirrorHost).Store(&url)
	if err != nil {
		fmt.Print(rawURL)
		return
	}

	if utils.ValidURL(url) {
		fmt.Print(url)
		return
	}

	fmt.Print(rawURL)
}
