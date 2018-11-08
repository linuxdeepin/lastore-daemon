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

package utils

import (
	"flag"
	"os"
	"path"

	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

func SetSeelogger(levels string, format string, output string) error {
	os.MkdirAll(path.Dir(output), 0755)

	config := `
<seelog type="sync">
	<outputs formatid="all">
		<filter levels="` + levels + `">
		  <rollingfile type="size" maxsize="1000000" maxrolls="3" filename="` + output + `"/>
		  <console />
		</filter>
	</outputs>
	<formats>
	  <format id="all" format="` + format + `"/>
	</formats>
</seelog>`
	logger, err := log.LoggerFromConfigAsBytes([]byte(config))
	if err != nil {
		return err
	}
	err = log.ReplaceLogger(logger)
	log.Debugf("SetLogger with %q %q %q --> %v\n", levels, format, output, err)
	return err
}

const DefaultLogFormat = "[%Level] [%Date %Time]@%File.%Line %Msg%n"
const DefaultLogLevel = "info,warn,error"
const DefaultLogOutput = "/var/log/lastore/daemon.log"

var baseLogDir = flag.String("log", "/var/log/lastore", "the directory to store logs")

func SetLogger(levels, format, output string) *dbus.Error {
	err := SetSeelogger(levels, format, output)
	return dbusutil.ToError(err)
}
