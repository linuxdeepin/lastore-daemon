// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanAllCacheExtra(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	argsFile := filepath.Join(t.TempDir(), "apt-get.args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "apt-get"), []byte(script), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cleanAllCache()

	data, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := strings.Fields(string(data))
	require.Len(t, args, 3)
	assert.Equal(t, "clean", args[0])
	assert.Equal(t, "-c", args[1])
	assert.Equal(t, system.LastoreAptV2CommonConfPath, args[2])
}
