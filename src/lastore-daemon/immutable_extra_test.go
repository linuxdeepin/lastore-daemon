// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installFakeImmutableCtl substitutes a fake deepin-immutable-ctl so tests never
// execute the real binary, which triggers a polkit authorization prompt for the
// user. The fake returns a full-merge probe answer for the "admin deploy -h"
// call and a valid rollback JSON payload for the "--can-rollback -j" query.
func installFakeImmutableCtl(t *testing.T) {
	t.Helper()
	orig := deepinImmutableCtlPath
	path := filepath.Join(t.TempDir(), "deepin-immutable-ctl")
	script := `#!/bin/sh
case "$*" in
  *"--can-rollback"*) printf '%s' '{"code":0,"message":"ok","data":{"version":1,"can_rollback":true,"reboot":false}}' ;;
  *) printf '%s' '--full-merge' ;;
esac
`
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	deepinImmutableCtlPath = path
	t.Cleanup(func() { deepinImmutableCtlPath = orig })
}

func TestOstreeParseRollbackData(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	data, err := mgr.osTreeParseRollbackData()
	require.NoError(t, err)
	var raw json.RawMessage
	assert.NoError(t, json.Unmarshal([]byte(data), &raw))
}

// installFakeImmutableCtlScript substitutes deepin-immutable-ctl with an
// arbitrary shell script so tests can exercise osTree error branches without a
// real binary.
func installFakeImmutableCtlScript(t *testing.T, script string) {
	t.Helper()
	orig := deepinImmutableCtlPath
	path := filepath.Join(t.TempDir(), "deepin-immutable-ctl")
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	deepinImmutableCtlPath = path
	t.Cleanup(func() { deepinImmutableCtlPath = orig })
}

func TestOstreeParseRollbackDataCmdError(t *testing.T) {
	installFakeImmutableCtlScript(t, "#!/bin/sh\nexit 1\n")
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	_, err := mgr.osTreeParseRollbackData()
	assert.Error(t, err)
}

func TestOstreeParseRollbackDataBinaryMissing(t *testing.T) {
	orig := deepinImmutableCtlPath
	deepinImmutableCtlPath = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { deepinImmutableCtlPath = orig })
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	_, err := mgr.osTreeParseRollbackData()
	assert.Error(t, err)
}

func TestOstreeParseRollbackDataMalformed(t *testing.T) {
	installFakeImmutableCtlScript(t, "#!/bin/sh\nprintf '%s' 'not-json'\n")
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	_, err := mgr.osTreeParseRollbackData()
	assert.Error(t, err)
}

func TestOstreeParseRollbackDataRespError(t *testing.T) {
	installFakeImmutableCtlScript(t, `#!/bin/sh
printf '%s' '{"code":1,"message":"fail","error":{"code":"E1","message":["boom"]},"data":null}'`)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	_, err := mgr.osTreeParseRollbackData()
	assert.Error(t, err)
}

func TestOstreeCanRollback(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	canRollback, dataJson := mgr.osTreeCanRollback()
	assert.True(t, canRollback)
	assert.NotEmpty(t, dataJson)
}

func TestOstreeNeedRebootAfterRollback(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	assert.False(t, mgr.osTreeNeedRebootAfterRollback())
}

func TestCheckFullMergeSupport(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	assert.True(t, mgr.checkFullMergeSupport())
}

func TestOsTreeRefresh(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	assert.NoError(t, mgr.osTreeRefresh(false))
}

func TestOsTreeFinalize(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	assert.NoError(t, mgr.osTreeFinalize())
}

func TestOsTreeRollback(t *testing.T) {
	installFakeImmutableCtl(t)
	mgr := newImmutableManager(func(info system.JobProgressInfo) {})

	assert.NoError(t, mgr.osTreeRollback())
}
