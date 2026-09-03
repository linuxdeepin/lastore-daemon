// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmartMirrorGetInterfaceName(t *testing.T) {
	s := &SmartMirror{}
	assert.Equal(t, "org.deepin.dde.Lastore1.Smartmirror", s.GetInterfaceName())
}

func TestSmartMirrorCanQuit(t *testing.T) {
	s := &SmartMirror{taskCount: 0}
	assert.True(t, s.canQuit())

	s2 := &SmartMirror{taskCount: 3}
	assert.False(t, s2.canQuit())
}

func TestSmartMirrorRouteInvalidURL(t *testing.T) {
	s := &SmartMirror{}
	// invalid original returns original
	assert.Equal(t, "not-a-url", s.route("not-a-url", "http://mirror.com"))
	// invalid officialMirror returns original
	assert.Equal(t, "http://a.com/pool/x", s.route("http://a.com/pool/x", "not-a-url"))
}

func TestSmartMirrorRoutePrefixMismatch(t *testing.T) {
	s := &SmartMirror{}
	// original doesn't start with officialMirror prefix → returns original
	assert.Equal(t, "http://other.com/dists/stable/main/binary-amd64/Packages",
		s.route("http://other.com/dists/stable/main/binary-amd64/Packages",
			"http://mirror.com"))
}
