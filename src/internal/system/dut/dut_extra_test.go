// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestCheckTypeString(t *testing.T) {
	tests := []struct {
		ct   CheckType
		want string
	}{
		{PreUpdateCheck, "pre_update_check"},
		{PostUpdateCheck, "post_update_check"},
		{PreDownloadCheck, "pre_download_check"},
		{PostDownloadCheck, "post_download_check"},
		{PreBackupCheck, "pre_backup_check"},
		{PostBackupCheck, "post_backup_check"},
		{PreUpgradeCheck, "pre_upgrade_check"},
		{MidUpgradeCheck, "mid_upgrade_check"},
		{PostUpgradeCheck, "post_upgrade_check"},
		{CheckType(99), ""},
	}
	for _, tt := range tests {
		got := tt.ct.String()
		assert.Equal(t, tt.want, got, "CheckType(%d).String()", tt.ct)
	}
}

func TestOptionToArgs(t *testing.T) {
	options := map[string]string{
		"--key1": "val1",
		"--key2": "val2",
	}
	args := OptionToArgs(options)
	assert.Len(t, args, 4)
}

func TestOptionToArgsEmpty(t *testing.T) {
	args := OptionToArgs(map[string]string{})
	assert.Empty(t, args)
}

func TestOptionToArgsNil(t *testing.T) {
	args := OptionToArgs(nil)
	assert.Empty(t, args)
}

func TestGenPkgList(t *testing.T) {
	pkgMap := map[string]system.PackageInfo{
		"pkg1": {Name: "pkg1", Version: "1.0"},
		"pkg2": {Name: "pkg2", Version: "2.0"},
	}
	list := genPkgList(pkgMap)
	assert.Len(t, list, 2)
	for _, info := range list {
		assert.Equal(t, skipVersion, info.Need)
	}
}
