// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDfAvailOuput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{"single value", "12345678", 12345678, false},
		{"with header", "Avail\n50000", 50000, false},
		{"empty", "", 0, true},
		{"invalid", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDfAvailOuput(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetDataDiskFreeSpaceFallbackToRoot(t *testing.T) {
	oldExist, oldRunner := dataDirExists, dfRunner
	dataDirExists = func(string) error { return errors.New("no /data") }
	dfRunner = func(int, string, ...string) (string, error) { return "Avail\n424242", nil }
	t.Cleanup(func() { dataDirExists, dfRunner = oldExist, oldRunner })

	got, err := GetDataDiskFreeSpace()
	require.NoError(t, err)
	assert.Equal(t, uint64(424242), got)
}

func TestGetDataDiskFreeSpaceFallbackRootError(t *testing.T) {
	oldExist, oldRunner := dataDirExists, dfRunner
	dataDirExists = func(string) error { return errors.New("no /data") }
	dfRunner = func(int, string, ...string) (string, error) { return "", errors.New("df failed") }
	t.Cleanup(func() { dataDirExists, dfRunner = oldExist, oldRunner })

	_, err := GetDataDiskFreeSpace()
	assert.Error(t, err)
}

func TestGetDataDiskFreeSpaceDataDiskPresent(t *testing.T) {
	oldExist, oldRunner := dataDirExists, dfRunner
	dataDirExists = func(string) error { return nil }
	dfRunner = func(int, string, ...string) (string, error) { return "Avail\n99999", nil }
	t.Cleanup(func() { dataDirExists, dfRunner = oldExist, oldRunner })

	got, err := GetDataDiskFreeSpace()
	require.NoError(t, err)
	assert.Equal(t, uint64(99999), got)
}

func TestGetDataDiskFreeSpaceDataDiskError(t *testing.T) {
	oldExist, oldRunner := dataDirExists, dfRunner
	dataDirExists = func(string) error { return nil }
	dfRunner = func(int, string, ...string) (string, error) { return "", errors.New("df failed") }
	t.Cleanup(func() { dataDirExists, dfRunner = oldExist, oldRunner })

	_, err := GetDataDiskFreeSpace()
	assert.Error(t, err)
}
