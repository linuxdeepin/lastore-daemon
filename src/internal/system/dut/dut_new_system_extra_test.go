// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSystem(t *testing.T) {
	dir := t.TempDir()

	// 覆盖 system 仓库目录与 platform.list 路径,避免触达 /var/lib/lastore
	oldOrigin := system.OriginSourceDir
	oldUnknown := system.UnknownSourceDir
	oldOther := system.OtherSystemSourceDir
	oldCustom := system.CustomSourceDir
	oldLastore := system.LastoreSourcesPath
	oldPlatform := system.PlatFormSourceFile

	system.OriginSourceDir = filepath.Join(dir, "sources.list.d")
	system.UnknownSourceDir = filepath.Join(dir, "unknownSource.d")
	system.OtherSystemSourceDir = filepath.Join(dir, "otherSystemSource.d")
	system.CustomSourceDir = filepath.Join(dir, "customSource.d")
	system.LastoreSourcesPath = filepath.Join(dir, "sources.list")
	system.PlatFormSourceFile = filepath.Join(dir, "platform.list")

	t.Cleanup(func() {
		system.OriginSourceDir = oldOrigin
		system.UnknownSourceDir = oldUnknown
		system.OtherSystemSourceDir = oldOther
		system.CustomSourceDir = oldCustom
		system.LastoreSourcesPath = oldLastore
		system.PlatFormSourceFile = oldPlatform
	})

	s := NewSystem(nil, nil, true)
	require.NotNil(t, s)

	// PlatFormSourceFile 不存在时应被创建
	assert.FileExists(t, system.PlatFormSourceFile)
}
