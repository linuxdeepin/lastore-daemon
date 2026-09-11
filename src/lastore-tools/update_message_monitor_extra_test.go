// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	config "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectLogs(t *testing.T) {
	oldPlatform := updatePlatform
	oldLogFiles := logFiles
	defer func() {
		updatePlatform = oldPlatform
		logFiles = oldLogFiles
	}()

	// Disable the process so PostUpdateLogFiles returns before any upload.
	updatePlatform = updateplatform.NewUpdatePlatformManager(&config.Config{
		PlatformDisabled: config.DisabledProcess,
	}, false)
	logFiles = []string{}
	collectLogs()
}

func TestCollectLogsWithFile(t *testing.T) {
	oldPlatform := updatePlatform
	oldLogFiles := logFiles
	defer func() {
		updatePlatform = oldPlatform
		logFiles = oldLogFiles
	}()

	updatePlatform = updateplatform.NewUpdatePlatformManager(&config.Config{
		PlatformDisabled: config.DisabledProcess,
	}, false)

	const base = "lastore_tools_collect.log"
	src := filepath.Join(t.TempDir(), base)
	require.NoError(t, os.WriteFile(src, []byte("line1\n"), 0644))
	logFiles = []string{src}
	collectLogs()

	out := "/tmp/" + base
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "line1\n", string(data))
	_ = os.Remove(out)
}
