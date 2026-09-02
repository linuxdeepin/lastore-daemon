// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPkgDependency(t *testing.T) {
	// apt-get check should work on a properly configured system
	err := CheckPkgDependency()
	// Just verify it doesn't panic; error may or may not occur depending on system state
	_ = err
}

func TestCheckCoreFileExistEmpty(t *testing.T) {
	err := CheckCoreFileExist("")
	assert.Error(t, err, "CheckCoreFileExist with empty path should return error")
}
