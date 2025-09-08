// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"fmt"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestGetRules(t *testing.T) {
	upm := &UpdatePlatformManager{
		requestUrl:     "https://update-platform-pre.uniontech.com",
		targetBaseline: "pro-20-std-0029",
		TargetCorePkgs: make(map[string]system.PackageInfo),
		BaselinePkgs:   make(map[string]system.PackageInfo),
		SelectPkgs:     make(map[string]system.PackageInfo),
		FreezePkgs:     make(map[string]system.PackageInfo),
		PurgePkgs:      make(map[string]system.PackageInfo),
	}
	err := upm.updateTargetPkgMetaSync()
	if err != nil {
		fmt.Println(err)
	}

	upm.GetRules()
	assert.NotEmpty(t, upm.PreCheck)
	assert.NotEmpty(t, upm.PostCheck)
	assert.NotEmpty(t, upm.PostCheck)
}
