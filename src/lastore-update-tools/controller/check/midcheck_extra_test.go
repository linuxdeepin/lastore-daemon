// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPkgDependency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apt-get")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	err := CheckPkgDependency()
	assert.NoError(t, err)
}

func TestCheckCoreFileExistEmpty(t *testing.T) {
	err := CheckCoreFileExist("")
	assert.Error(t, err, "CheckCoreFileExist with empty path should return error")
}
