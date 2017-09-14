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

package main

import "pkg.deepin.io/lib/gettext"
import "pkg.deepin.io/lib/dbus"
import log "github.com/cihub/seelog"
import "os"
import "path"

func main() {
	setupLog()

	gettext.InitI18n()
	gettext.Textdomain("lastore-daemon")

	NewLastore()
	dbus.DealWithUnhandledMessage()
	if err := dbus.Wait(); err != nil {
	}
}

func setupLog() {
	logDirectory := path.Join(os.Getenv("HOME"), ".cache", "lastore-daemon")
	os.MkdirAll(logDirectory, 0755)

	config := `
<seelog type="sync">
	<outputs formatid="all">
		<filter levels="info,debug,warn,error,trace">
		  <file path="` + logDirectory + `/session.log"/>
		  <console />
		</filter>
	</outputs>

	<formats>
	  <format id="all" format="[%Level] [%Date %Time]@%File.%Line %Msg%n"/>
	</formats>
</seelog>`

	logger, err := log.LoggerFromConfigAsBytes([]byte(config))
	if err != nil {
		panic(err)
	}
	log.ReplaceLogger(logger)
}
