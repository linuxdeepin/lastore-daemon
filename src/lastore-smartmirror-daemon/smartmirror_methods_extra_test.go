// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/stretchr/testify/assert"
)

func TestSmartMirrorSetEnableUnchanged(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "smartmirror_config.json")
	s := &SmartMirror{
		service: dbusutil.NewService(nil),
		config:  newConfig(tmp),
		Enable:  true,
	}

	err := s.SetEnable(true)
	assert.Nil(t, err)
	assert.True(t, s.Enable)
	assert.True(t, s.config.Enable)
}

func TestSmartMirrorQueryDisabled(t *testing.T) {
	s := &SmartMirror{service: dbusutil.NewService(nil), Enable: false}

	url, err := s.Query("https://official/pool/x.deb", "https://official", "https://mirror")
	assert.Nil(t, err)
	assert.Equal(t, "https://mirror/pool/x.deb", url)
}

func TestSmartMirrorGetExportedMethods(t *testing.T) {
	s := &SmartMirror{}
	methods := s.GetExportedMethods()
	assert.Len(t, methods, 2)

	names := []string{methods[0].Name, methods[1].Name}
	assert.ElementsMatch(t, []string{"Query", "SetEnable"}, names)
}
