// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    float64
		expected string
	}{
		// 字节 (B)
		{"zero bytes", 0, "0 B"},
		{"1 byte", 1, "1 B"},
		{"500 bytes", 500, "500 B"},
		{"999 bytes", 999, "999 B"},

		// KiB (1024 bytes)
		{"1 KiB", 1024, "1.00 KiB"},
		{"1.5 KiB", 1536, "1.50 KiB"},
		{"10 KiB", 10240, "10.00 KiB"},
		{"100 KiB", 102400, "100.00 KiB"},
		{"1023.99 KiB", 1048575, "1024.00 KiB"},

		// MiB (1024 KiB = 1,048,576 bytes)
		{"1 MiB", 1048576, "1.00 MiB"},
		{"1.5 MiB", 1572864, "1.50 MiB"},
		{"15.03 MiB", 15754620, "15.02 MiB"},
		{"100 MiB", 104857600, "100.00 MiB"},
		{"999.99 MiB", 1048575000, "1000.00 MiB"},

		// GiB (1024 MiB = 1,073,741,824 bytes)
		{"1 GiB", 1073741824, "1.00 GiB"},
		{"1.5 GiB", 1610612736, "1.50 GiB"},
		{"2.3 GiB", 2469606195, "2.30 GiB"},
		{"10 GiB", 10737418240, "10.00 GiB"},
		{"100 GiB", 107374182400, "100.00 GiB"},

		// TiB (1024 GiB)
		{"1 TiB", 1099511627776, "1.00 TiB"},
		{"2 TiB", 2199023255552, "2.00 TiB"},

		// 边界值测试
		{"boundary 1023 bytes", 1023, "1023 B"},
		{"boundary 1024 bytes", 1024, "1.00 KiB"},
		{"boundary 1048575 bytes", 1048575, "1024.00 KiB"},
		{"boundary 1048576 bytes", 1048576, "1.00 MiB"},

		// 负数处理
		{"negative value", -1, "unknown"},
		{"negative large", -1000, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatSize(%v) = %v, want %v", tt.bytes, result, tt.expected)
			}
		})
	}
}
