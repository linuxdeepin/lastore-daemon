// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestCategorySupportAutoInstall(t *testing.T) {
	tests := []struct {
		name            string
		autoInstall     bool
		autoInstallType system.UpdateType
		category        system.UpdateType
		expected        bool
	}{
		{
			name:            "autoInstall disabled",
			autoInstall:     false,
			autoInstallType: system.SystemUpdate,
			category:        system.SystemUpdate,
			expected:        false,
		},
		{
			name:            "matching category",
			autoInstall:     true,
			autoInstallType: system.SystemUpdate,
			category:        system.SystemUpdate,
			expected:        true,
		},
		{
			name:            "non-matching category",
			autoInstall:     true,
			autoInstallType: system.SecurityUpdate,
			category:        system.SystemUpdate,
			expected:        false,
		},
		{
			name:            "multi-type includes category",
			autoInstall:     true,
			autoInstallType: system.SystemUpdate | system.SecurityUpdate,
			category:        system.SecurityUpdate,
			expected:        true,
		},
		{
			name:            "zero type",
			autoInstall:     true,
			autoInstallType: 0,
			category:        system.SystemUpdate,
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				updater: &Updater{
					AutoInstallUpdates:    tt.autoInstall,
					AutoInstallUpdateType: tt.autoInstallType,
				},
			}
			result := m.categorySupportAutoInstall(tt.category)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessLogFds_NilLogTmpFile(t *testing.T) {
	m := &Manager{
		logFds:     nil,
		logTmpFile: nil,
	}
	m.processLogFds("test message")
	assert.Empty(t, m.logFds)
}

func TestProcessLogFds_WithLogTmpFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "logtmp")
	require.NoError(t, err)
	defer tmpFile.Close()

	m := &Manager{
		logFds:     nil,
		logTmpFile: tmpFile,
	}
	m.processLogFds("hello world")

	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestProcessLogFds_ValidAndInvalidFds(t *testing.T) {
	tmpDir := t.TempDir()

	validFile, err := os.CreateTemp(tmpDir, "valid")
	require.NoError(t, err)
	defer validFile.Close()

	closedFile, err := os.CreateTemp(tmpDir, "closed")
	require.NoError(t, err)
	closedFile.Close()

	m := &Manager{
		logFds:     []*os.File{validFile, closedFile, nil},
		logTmpFile: nil,
	}
	m.processLogFds("log line")

	assert.Len(t, m.logFds, 1)
	assert.Equal(t, validFile, m.logFds[0])

	content, err := os.ReadFile(validFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "log line", string(content))
}

func TestInhibitAutoQuitCountAdd(t *testing.T) {
	m := &Manager{}
	m.inhibitAutoQuitCountAdd()
	assert.Equal(t, int32(1), m.inhibitAutoQuitCount)
	m.inhibitAutoQuitCountAdd()
	assert.Equal(t, int32(2), m.inhibitAutoQuitCount)
}

func TestInhibitAutoQuitCountSub(t *testing.T) {
	m := &Manager{inhibitAutoQuitCount: 3}
	m.inhibitAutoQuitCountSub()
	assert.Equal(t, int32(2), m.inhibitAutoQuitCount)
	m.inhibitAutoQuitCountSub()
	assert.Equal(t, int32(1), m.inhibitAutoQuitCount)
}

func TestSaveCacheJob_EmptyJobList(t *testing.T) {
	m := &Manager{}
	m.saveCacheJob()

	_, err := os.Stat(lastoreJobCacheJson)
	assert.NoError(t, err, "cache file should be written even with empty job list")
	_ = os.Remove(lastoreJobCacheJson)
}

func TestSaveCacheJob_WithFailedJob(t *testing.T) {
	job := &Job{
		Id:       "test-job-1",
		Name:     "test",
		Packages: []string{"pkg1"},
		Status:   system.FailedStatus,
	}
	m := &Manager{
		jobList: []*Job{job},
	}
	m.saveCacheJob()
	defer os.Remove(lastoreJobCacheJson)

	assert.FileExists(t, lastoreJobCacheJson)
}
