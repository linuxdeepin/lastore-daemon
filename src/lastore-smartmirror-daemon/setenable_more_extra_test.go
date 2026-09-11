// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmartMirrorSetEnableChangedEmitError(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "smartmirror_config.json")
	s := &SmartMirror{
		service: dbusutil.NewService(nil),
		config:  newConfig(tmp),
		Enable:  false,
	}

	err := s.SetEnable(true)
	// config.save succeeds, then the changed path emits a property change on a
	// service that has not exported this object, so SetEnable surfaces an error.
	require.NotNil(t, err)
	assert.True(t, s.Enable)
	assert.True(t, s.config.Enable)
}

func TestSmartMirrorSetEnableConfigSaveError(t *testing.T) {
	// filePath points at a non-empty directory so config.save() fails; the error
	// is wrapped into a dbus error before any property change is emitted.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0644))
	s := &SmartMirror{
		service: dbusutil.NewService(nil),
		config:  newConfig(dir),
		Enable:  true,
	}

	err := s.SetEnable(false)
	require.NotNil(t, err)
	// Enable is assigned before the save error is returned.
	assert.False(t, s.Enable)
}
