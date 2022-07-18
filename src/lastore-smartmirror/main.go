/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
	smartmirror := sysBus.Object("org.deepin.lastore1.Smartmirror", "/org/deepin/lastore1/Smartmirror")
	err = smartmirror.Call("org.deepin.lastore1.Smartmirror.Query", 0, rawURL, officialHost, mirrorHost).Store(&url)
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
