// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "github.com/linuxdeepin/go-lib/dbusutil"

const (
	smartMirrorDBusName      = "org.deepin.dde.Lastore1.Smartmirror"
	smartMirrorDBusPath      = "/org/deepin/dde/Lastore1/Smartmirror"
	smartMirrorDBusInterface = "org.deepin.dde.Lastore1.Smartmirror"
)

func newSmartMirrorReporter(service *dbusutil.Service) func(actualDownloadReport) {
	if service == nil {
		return nil
	}
	return func(report actualDownloadReport) {
		if report.URI == "" {
			return
		}
		call := service.Conn().Object(smartMirrorDBusName, smartMirrorDBusPath).Call(
			smartMirrorDBusInterface+".RecordActualDownload", 0,
			report.URI, report.UsedMirror, report.Bytes, report.ElapsedMs, report.WaitMs,
			report.Failed, report.BytesKnown,
		)
		if err := call.Err; err != nil {
			logger.Warningf("report smartmirror actual download failed: %v", err)
		}
	}
}
