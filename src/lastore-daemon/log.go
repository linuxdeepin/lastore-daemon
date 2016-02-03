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
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"os"
	"path"
	"time"
)

func SetSeelogger(levels string, format string, output string) error {
	config := `
<seelog type="sync">
	<outputs formatid="all">
		<filter levels="` + levels + `">
		  <file path="` + output + `"/>
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
const DefaultLogOutput = "/var/log/lastore/last/daemon.log"

var baseLogDir = flag.String("log", "/var/log/lastore", "the directory to store logs")

func setupLog() {
	var logDir = path.Join(*baseLogDir, time.Now().Format("2006-1-02 15:04:05"))

	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		panic(fmt.Sprintf("Can't create base Dir %v", err))
	}
	lastDir := path.Join(*baseLogDir, "last")
	os.Remove(lastDir)
	err = os.Symlink(logDir, lastDir)
	if err != nil {
		panic(err)
	}

	system.SetupLogDir(logDir)
}

func (Manager) SetLogger(levels, format, output string) error {
	return SetSeelogger(levels, format, output)
}
