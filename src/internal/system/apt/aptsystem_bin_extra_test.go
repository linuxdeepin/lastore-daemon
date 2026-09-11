// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// writeFakeBin 写一个 exit 0 的假二进制,返回其路径。
func writeFakeBin(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755))
	return path
}

func overrideBinVars(t *testing.T) {
	t.Helper()
	oldApt := AptGetBinPath
	oldClean := LastoreAptCleanBinPath
	oldImm := DeepinImmutableCtlPath
	oldDpkg := DpkgBinPath
	AptGetBinPath = writeFakeBin(t, "apt-get")
	LastoreAptCleanBinPath = writeFakeBin(t, "lastore-apt-clean")
	DeepinImmutableCtlPath = writeFakeBin(t, "deepin-immutable-ctl")
	DpkgBinPath = writeFakeBin(t, "dpkg")
	t.Cleanup(func() {
		AptGetBinPath = oldApt
		LastoreAptCleanBinPath = oldClean
		DeepinImmutableCtlPath = oldImm
		DpkgBinPath = oldDpkg
	})
}

func newAPTSystemWithIndicator() *APTSystem {
	return &APTSystem{
		CmdSet:    make(map[string]*system.Command),
		Indicator: func(system.JobProgressInfo) {},
	}
}

func TestAPTSystemUpdateSourceExtra(t *testing.T) {
	overrideBinVars(t)
	p := newAPTSystemWithIndicator()
	err := p.UpdateSource("job-1", map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestAPTSystemCleanExtra(t *testing.T) {
	overrideBinVars(t)
	p := newAPTSystemWithIndicator()
	err := p.Clean("job-1")
	assert.NoError(t, err)
}

func TestAPTSystemOsBackupExtra(t *testing.T) {
	overrideBinVars(t)
	p := newAPTSystemWithIndicator()
	err := p.OsBackup("job-1")
	assert.NoError(t, err)
}

func TestAPTSystemOsBackupMalformedLocaleEnv(t *testing.T) {
	overrideBinVars(t)
	oldLocales := system.OriginalLocaleEnvs
	system.OriginalLocaleEnvs = []string{"LC_ALL=en_US.UTF-8", "BROKEN"}
	t.Cleanup(func() { system.OriginalLocaleEnvs = oldLocales })

	p := newAPTSystemWithIndicator()
	err := p.OsBackup("job-1")
	assert.NoError(t, err)
}

func TestAPTSystemFixErrorDpkgInterruptedExtra(t *testing.T) {
	overrideBinVars(t)
	p := newAPTSystemWithIndicator()
	err := p.FixError("job-1", string(system.ErrorDpkgInterrupted), map[string]string{}, map[string]string{})
	assert.NoError(t, err)
}
