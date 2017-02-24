/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"flag"
	log "github.com/cihub/seelog"
	"os"
	"path"
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

const DefaultLogFomrat = "[%Level] [%Date %Time]@%File.%Line %Msg%n"
const DefaultLogLevel = "info,warn,error"
const DefaultLogOutput = "/var/log/lastore/daemon.log"

var baseLogDir = flag.String("log", "/var/log/lastore", "the directory to store logs")

func (Manager) SetLogger(levels, format, output string) error {
	return SetSeelogger(levels, format, output)
}
