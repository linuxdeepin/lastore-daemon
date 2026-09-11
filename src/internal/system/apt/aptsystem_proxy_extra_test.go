// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// newNoopAPTSystem 构造一个使用假 apt-get(exit 0)的 APTSystem,
// 使 proxy.go 中依赖 apt-get 的方法可在无 root 环境下执行。
func newNoopAPTSystem(t *testing.T) *APTSystem {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "apt-get")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	return &APTSystem{
		CmdSet:    make(map[string]*system.Command),
		Indicator: func(system.JobProgressInfo) {},
	}
}

func TestAPTSystemDownloadPackagesExtra(t *testing.T) {
	p := newNoopAPTSystem(t)
	err := p.DownloadPackages("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemDownloadSourceExtra(t *testing.T) {
	p := newNoopAPTSystem(t)
	err := p.DownloadSource("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemRemoveExtra(t *testing.T) {
	p := newNoopAPTSystem(t)
	err := p.Remove("job-1", []string{"pkg1"}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemInstallExtra(t *testing.T) {
	p := newNoopAPTSystem(t)
	err := p.Install("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemDistUpgradeExtra(t *testing.T) {
	p := newNoopAPTSystem(t)
	err := p.DistUpgrade("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemFixErrorDependenciesBrokenExtra(t *testing.T) {
	p := newNoopAPTSystem(t)
	err := p.FixError("job-1", string(system.ErrorDependenciesBroken), map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

// writeScriptBin 写一个自定义脚本的假二进制,返回其路径。
func writeScriptBin(t *testing.T, name, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0755))
	return path
}

func TestAPTSystemDownloadSourceIncremental(t *testing.T) {
	p := newNoopAPTSystem(t)
	oldImm := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = writeFakeBin(t, "deepin-immutable-ctl")
	t.Cleanup(func() { DeepinImmutableCtlPath = oldImm })

	p.IncrementalUpdate = true
	err := p.DownloadSource("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{
		aptHttpLimitKey:   "100",
		aptSourceListKey:  "/tmp/s.list",
		aptSourcePartsKey: "/tmp/s.list.d",
	})
	assert.NoError(t, err)
}

func TestAPTSystemDownloadSourceIncrementalNoOpts(t *testing.T) {
	p := newNoopAPTSystem(t)
	oldImm := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = writeFakeBin(t, "deepin-immutable-ctl")
	t.Cleanup(func() { DeepinImmutableCtlPath = oldImm })

	p.IncrementalUpdate = true
	err := p.DownloadSource("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemDistUpgradeIncremental(t *testing.T) {
	p := newNoopAPTSystem(t)
	oldImm := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = writeFakeBin(t, "deepin-immutable-ctl")
	t.Cleanup(func() { DeepinImmutableCtlPath = oldImm })

	p.IncrementalUpdate = true
	err := p.DistUpgrade("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{
		aptHttpLimitKey: "200",
	})
	assert.NoError(t, err)
}

func TestAPTSystemDistUpgradeDependenciesBrokenContinues(t *testing.T) {
	prependFakeBin(t, "apt-get", "echo 'Unmet dependencies' >&2\nexit 1\n")
	p := &APTSystem{
		CmdSet:    make(map[string]*system.Command),
		Indicator: func(system.JobProgressInfo) {},
	}
	err := p.DistUpgrade("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemDistUpgradeOtherErrorReturns(t *testing.T) {
	prependFakeBin(t, "apt-get", "echo 'dpkg was interrupted' >&2\nexit 1\n")
	p := &APTSystem{
		CmdSet:    make(map[string]*system.Command),
		Indicator: func(system.JobProgressInfo) {},
	}
	err := p.DistUpgrade("job-1", []string{"pkg1"}, map[string]string{}, map[string]string{})
	assert.Error(t, err)
}

func TestAPTSystemUpdateSourceIncremental(t *testing.T) {
	overrideBinVars(t)
	p := newAPTSystemWithIndicator()
	p.IncrementalUpdate = true
	err := p.UpdateSource("job-1", map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemUpdateSourceIncrementalRemoteError(t *testing.T) {
	oldImm := DeepinImmutableCtlPath
	DeepinImmutableCtlPath = writeScriptBin(t, "deepin-immutable-ctl", "echo boom >&2\nexit 1\n")
	t.Cleanup(func() { DeepinImmutableCtlPath = oldImm })
	oldApt := AptGetBinPath
	AptGetBinPath = writeFakeBin(t, "apt-get")
	t.Cleanup(func() { AptGetBinPath = oldApt })

	p := newAPTSystemWithIndicator()
	p.IncrementalUpdate = true
	err := p.UpdateSource("job-1", map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemUpdateSourceIndexDownloadFailed(t *testing.T) {
	oldApt := AptGetBinPath
	AptGetBinPath = writeScriptBin(t, "apt-get", "echo 'Some index files failed to download' >&2\nexit 0\n")
	t.Cleanup(func() { AptGetBinPath = oldApt })

	ch := make(chan system.JobProgressInfo, 4)
	p := &APTSystem{
		CmdSet: make(map[string]*system.Command),
		Indicator: func(i system.JobProgressInfo) {
			if i.Status == system.FailedStatus {
				ch <- i
			}
		},
	}
	err := p.UpdateSource("job-1", map[string]string{}, map[string]string{})
	require.NoError(t, err)

	select {
	case info := <-ch:
		require.NotNil(t, info.Error)
		assert.Equal(t, system.ErrorIndexDownloadFailed, info.Error.ErrType)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for update source to finish")
	}
}

func TestAPTSystemUpdateSourceNoSpace(t *testing.T) {
	oldApt := AptGetBinPath
	AptGetBinPath = writeScriptBin(t, "apt-get", "echo 'Some index files failed to download' >&2\necho 'No space left on device' >&2\nexit 0\n")
	t.Cleanup(func() { AptGetBinPath = oldApt })

	ch := make(chan system.JobProgressInfo, 4)
	p := &APTSystem{
		CmdSet: make(map[string]*system.Command),
		Indicator: func(i system.JobProgressInfo) {
			if i.Status == system.FailedStatus {
				ch <- i
			}
		},
	}
	err := p.UpdateSource("job-1", map[string]string{}, map[string]string{})
	require.NoError(t, err)

	select {
	case info := <-ch:
		require.NotNil(t, info.Error)
		assert.Equal(t, system.ErrorInsufficientSpace, info.Error.ErrType)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for update source to finish")
	}
}
