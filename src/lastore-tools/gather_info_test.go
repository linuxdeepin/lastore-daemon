// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
)

func TestParseSizeToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{
			name:     "1024 MB",
			input:    "1024 MB",
			expected: 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "2048 MB",
			input:    "2048 MB",
			expected: 2048 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "4096 MB",
			input:    "4096 MB",
			expected: 4096 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "8192 MB",
			input:    "8192 MB",
			expected: 8192 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "16 GB",
			input:    "16 GB",
			expected: 16 * 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "32 GB",
			input:    "32 GB",
			expected: 32 * 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "1 TB",
			input:    "1 TB",
			expected: 1 * 1024 * 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "512 KB",
			input:    "512 KB",
			expected: 512 * 1024,
			wantErr:  false,
		},
		{
			name:     "1024 B",
			input:    "1024 B",
			expected: 1024,
			wantErr:  false,
		},
		{
			name:     "decimal value 2.5 GB",
			input:    "2.5 GB",
			expected: int64(2.5 * 1024 * 1024 * 1024),
			wantErr:  false,
		},
		{
			name:     "value with leading zeros 08192 MB",
			input:    "08192 MB",
			expected: 8192 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "value starting with 0",
			input:    "0 MB",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "value ending with 9",
			input:    "9 GB",
			expected: 9 * 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "value containing 0 and 9",
			input:    "2090 MB",
			expected: 2090 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "no unit",
			input:    "1024",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid unit",
			input:    "1024 PB",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "non-numeric value",
			input:    "No Module Installed",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSizeToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSizeToBytes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseSizeToBytes(%q) = %d, expected %d (diff: %d)", tt.input, got, tt.expected, got-tt.expected)
			}
		})
	}
}
