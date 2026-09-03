// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendSuffix(t *testing.T) {
	tests := []struct {
		r, suffix, want string
	}{
		{"http://example.com", "/", "http://example.com/"},
		{"http://example.com/", "/", "http://example.com/"},
		{"http://example.com", "/dists/", "http://example.com/dists/"},
		{"http://example.com/dists/", "/dists/", "http://example.com/dists/"},
	}
	for _, tt := range tests {
		got := appendSuffix(tt.r, tt.suffix)
		assert.Equal(t, tt.want, got)
	}
}
