// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachIndicator(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	ind := func(info system.JobProgressInfo) {}
	p.AttachIndicator(ind)
	assert.NotNil(t, p.Indicator)
}

func TestAttachDeliveryIndicator(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	ind := func(info system.JobDeliveryDownloadInfo) {}
	p.AttachDeliveryIndicator(ind)
	assert.NotNil(t, p.DeliveryIndicator)
}

func TestCheckSystem(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	err := p.CheckSystem("", "", nil, nil)
	assert.NoError(t, err)
}

func TestAbort(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	// CmdSet 为空,FindCMD 返回 nil,应返回 NotFoundError
	err := p.Abort("job1")
	assert.EqualError(t, err, system.NotFoundErrorMsg+"abort job1")
}

func TestAbortWithFailed(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	err := p.AbortWithFailed("job1")
	assert.EqualError(t, err, system.NotFoundErrorMsg+"abort job1")
}

func TestAbortExistingCommand(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	p.CmdSet["job1"] = &system.Command{JobId: "job1", Cancelable: false}
	// 命令不可取消时,Abort 走 c.Abort() 分支并返回 NotSupportError
	err := p.Abort("job1")
	assert.ErrorIs(t, err, system.NotSupportError)
}

func TestAbortWithFailedExistingCommand(t *testing.T) {
	p := NewSystem(nil, nil, false).(*APTSystem)
	p.CmdSet["job1"] = &system.Command{JobId: "job1", Cancelable: false}
	err := p.AbortWithFailed("job1")
	assert.ErrorIs(t, err, system.NotSupportError)
}

// overrideSystemDirs 将 system 包仓库目录变量指向临时目录,避免 New 触达 /var/lib/lastore。
func overrideSystemDirs(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldOrigin := system.OriginSourceDir
	oldUnknown := system.UnknownSourceDir
	oldOther := system.OtherSystemSourceDir
	oldCustom := system.CustomSourceDir
	oldLastore := system.LastoreSourcesPath

	system.OriginSourceDir = filepath.Join(dir, "sources.list.d")
	system.UnknownSourceDir = filepath.Join(dir, "unknownSource.d")
	system.OtherSystemSourceDir = filepath.Join(dir, "otherSystemSource.d")
	system.CustomSourceDir = filepath.Join(dir, "customSource.d")
	system.LastoreSourcesPath = filepath.Join(dir, "sources.list")

	t.Cleanup(func() {
		system.OriginSourceDir = oldOrigin
		system.UnknownSourceDir = oldUnknown
		system.OtherSystemSourceDir = oldOther
		system.CustomSourceDir = oldCustom
		system.LastoreSourcesPath = oldLastore
	})
}

func TestNew(t *testing.T) {
	overrideSystemDirs(t)

	p := New(nil, nil, true)
	require.NotNil(t, p.CmdSet)
	assert.True(t, p.IncrementalUpdate)
}

func TestNewSystem(t *testing.T) {
	overrideSystemDirs(t)

	s := NewSystem(nil, nil, false)
	require.NotNil(t, s)
	_, ok := s.(*APTSystem)
	assert.True(t, ok)
}
