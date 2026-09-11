// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import libCheck "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/cli"

// Injectable seams for the libCheck check functions so tests can exercise
// CheckSystem's branching without running real system checks. Each defaults to
// the real implementation and is never mutated in production code.
var (
	preUpdateCheckFn    = libCheck.PreUpdateCheck
	postUpdateCheckFn   = libCheck.PostUpdateCheck
	preDownloadCheckFn  = libCheck.PreDownloadCheck
	postDownloadCheckFn = libCheck.PostDownloadCheck
	preBackupCheckFn    = libCheck.PreBackupCheck
	postBackupCheckFn   = libCheck.PostBackupCheck
	preUpgradeCheckFn   = libCheck.PreUpgradeCheck
	midUpgradeCheckFn   = libCheck.MidUpgradeCheck
	postUpgradeCheckFn  = libCheck.PostUpgradeCheck
)
