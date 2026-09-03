// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

func TestManagerGetInterfaceName(t *testing.T) {
	m := &Manager{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Manager", m.GetInterfaceName())
}

func TestJobGetInterfaceName(t *testing.T) {
	j := &Job{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Job", j.GetInterfaceName())
}

func TestUpdaterGetInterfaceName(t *testing.T) {
	u := &Updater{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Updater", u.GetInterfaceName())
}

func TestJobGetPath(t *testing.T) {
	tests := []struct {
		id   string
		want dbus.ObjectPath
	}{
		{"/1", "/org/deepin/dde/Lastore1/Job/1"},
		{"/abc", "/org/deepin/dde/Lastore1/Job/abc"},
		{"", "/org/deepin/dde/Lastore1/Job"},
	}
	for _, tt := range tests {
		j := &Job{Id: tt.id}
		got := j.getPath()
		assert.Equal(t, tt.want, got)
	}
}
