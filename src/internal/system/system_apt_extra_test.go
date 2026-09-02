// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuestBasePackageName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"pkg-name", "pkg"},
		{"pkg:amd64", "pkg"},
		{"pkg_name", "pkg"},
		{"pkg", "pkg"},
		{"lib-test-1.0", "lib-test"},
		{"no-match", "no"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, guestBasePackageName(tt.input))
	}
}

func TestParsePackageSize(t *testing.T) {
	tests := []struct {
		line        string
		needSize    float64
		allSize     float64
		expectError bool
	}{
		{"Need to get 1,234 MB of archives", 1234 * 1000 * 1000, 1234 * 1000 * 1000, false},
		{"Need to get 100 MB of archives", 100 * 1000 * 1000, 100 * 1000 * 1000, false},
		{"Need to get 50 MB/100 MB of archives", 50 * 1000 * 1000, 100 * 1000 * 1000, false},
		{"Need to get 5 kB of archives", 5 * 1000, 5 * 1000, false},
		{"Need to get 1 GB of archives", 1 * 1000 * 1000 * 1000, 1 * 1000 * 1000 * 1000, false},
		{"invalid line", SizeUnknown, SizeUnknown, true},
	}
	for _, tt := range tests {
		need, all, err := parsePackageSize(tt.line)
		if tt.expectError {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.InDelta(t, tt.needSize, need, 1)
		assert.InDelta(t, tt.allSize, all, 1)
	}
}

func TestParseInstallAddSize(t *testing.T) {
	tests := []struct {
		line     string
		want     float64
		hasError bool
	}{
		{"After this operation, 200 MB of disk space will be used", 200 * 1000 * 1000, false},
		{"After this operation, 1,234 kB of disk space will be used", 1234 * 1000, false},
		{"After this operation, 200 GB disk space will be freed", 0, false},
		{"After this operation, 5 GB of disk space will be used", 5 * 1000 * 1000 * 1000, false},
		{"no match", SizeUnknown, true},
	}
	for _, tt := range tests {
		got, err := parseInstallAddSize(tt.line)
		if tt.hasError {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.InDelta(t, tt.want, got, 1)
	}
}
