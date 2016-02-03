/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package system

import (
	"io"
	"log"
	"os"
	"path"
	"time"
)

var baseLogDir string = "/var/log/lastore"

func SetupLogDir(dir string) {
	baseLogDir = dir
}

func CreateLogOutput(cmdType string, packageId string) io.WriteCloser {
	now := time.Now()
	var logName = path.Join(baseLogDir,
		cmdType+"_"+packageId+"_"+now.Format("15:04:05")+".log")
	os.MkdirAll(path.Dir(logName), 0755)
	w, err := os.Create(logName)
	if err != nil {
		log.Println("create log file :", err)
		return nil
	}
	return w
}
