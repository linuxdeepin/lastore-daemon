// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)


func TestExtractURLPathFromLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"deb line with http", "deb http://example.com/path stable main", "example.com/path"},
		{"deb line with https", "deb https://example.com/repo stable main", "example.com/repo"},
		{"deb-src line", "deb-src http://example.com/src stable main", "example.com/src"},
		{"no url field", "deb stable main", ""},
		{"empty line", "", ""},
		{"multiple fields url is second", "deb http://mirror.com/deepin stable main contrib", "mirror.com/deepin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractURLPathFromLine(tt.line))
		})
	}
}

func TestReplaceRepoSchemeWithDelivery(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "http to delivery",
			line: "deb http://example.com/path stable main",
			want: "deb delivery://example.com/path stable main",
		},
		{
			name: "https to delivery",
			line: "deb https://example.com/path stable main",
			want: "deb delivery://example.com/path stable main",
		},
		{
			name: "already delivery unchanged",
			line: "deb delivery://example.com/path stable main",
			want: "deb delivery://example.com/path stable main",
		},
		{
			name: "non deb line unchanged",
			line: "some random line",
			want: "some random line",
		},
		{
			name: "deb line with options bracket",
			line: "deb [arch=amd64] http://example.com/path stable main",
			want: "deb [arch=amd64] delivery://example.com/path stable main",
		},
		{
			name: "trailing slash trimmed in delivery",
			line: "deb http://example.com/path/ stable main",
			want: "deb delivery://example.com/path stable main",
		},
		{
			name: "only one field unchanged",
			line: "deb",
			want: "deb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, replaceRepoSchemeWithDelivery(tt.line))
		})
	}
}

