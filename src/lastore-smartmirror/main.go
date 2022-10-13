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

	"internal/utils"

	"github.com/godbus/dbus"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage %s URL OfficialHost MirrorHost\n", os.Args[0])
		os.Exit(-1)
	}

	rawURL := os.Args[1]
	officialHost := os.Args[2]
	mirrorHost := os.Args[3]

	url := ""
	sysBus, err := dbus.SystemBus()
	if err != nil {
		fmt.Print(rawURL)
		return
	}
	smartmirror := sysBus.Object("com.deepin.lastore.Smartmirror", "/com/deepin/lastore/Smartmirror")
	err = smartmirror.Call("com.deepin.lastore.Smartmirror.Query", 0, rawURL, officialHost, mirrorHost).Store(&url)
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
