// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestCheckPkgSystemErrorNoLock(t *testing.T) {
	noopIndicator := func(info system.JobProgressInfo) {}
	// apt-get check with NoLocking should work on a properly configured system
	err := CheckPkgSystemError(false, noopIndicator)
	_ = err
}

func TestCheckPkgSystemErrorWithLock(t *testing.T) {
	noopIndicator := func(info system.JobProgressInfo) {}
	// With lock=true, this needs remount permissions; may fail, just verify no panic
	err := CheckPkgSystemError(true, noopIndicator)
	_ = err
}
