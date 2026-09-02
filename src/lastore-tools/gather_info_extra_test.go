// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToHardwareBytes(t *testing.T) {
	gb := int64(1024 * 1024 * 1024)
	tests := []struct {
		input int64
		want  int64
	}{
		{0, 256 * gb},
		{100 * gb, 256 * gb},
		{255 * gb, 256 * gb},
		{256 * gb, 512 * gb},
		{300 * gb, 512 * gb},
		{511 * gb, 512 * gb},
		{512 * gb, 1024 * gb},
		{600 * gb, 1024 * gb},
		{1023 * gb, 1024 * gb},
		{1024 * gb, 2048 * gb},
		{1500 * gb, 2048 * gb},
		{2047 * gb, 2048 * gb},
		{2048 * gb, 4096 * gb},
		{3000 * gb, 4096 * gb},
		{4095 * gb, 4096 * gb},
		{4096 * gb, 4096 * gb},
		{5000 * gb, 5000 * gb},
	}
	for _, tt := range tests {
		got := convertToHardwareBytes(tt.input)
		assert.Equal(t, tt.want, got, "input=%d", tt.input)
	}
}

func TestParseMemoryModuleOutput(t *testing.T) {
	output := `Memory Device
	Size: 8 GB
Memory Device
	Size: 4096 MB
Memory Device
	Size: No Module Installed
`
	result, err := parseMemoryModuleOutput(output)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "1", result[0].MemoryNo)
	assert.Equal(t, int64(8*1024*1024*1024), result[0].Capacity)
	assert.Equal(t, "2", result[1].MemoryNo)
	assert.Equal(t, int64(4096*1024*1024), result[1].Capacity)
}

func TestParseMemoryModuleOutputEmpty(t *testing.T) {
	result, err := parseMemoryModuleOutput("")
	assert.NoError(t, err)
	assert.Empty(t, result)
}
