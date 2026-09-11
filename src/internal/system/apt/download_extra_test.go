// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prependFakeAptGet 在 PATH 前插入一个假 apt-get,使 DownloadPackages/safeStart
// 等依赖 apt-get 的路径可在无 root 环境下执行。
func prependFakeAptGet(t *testing.T, exitCode int, output string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(output) + "\nexit " + strconv.Itoa(exitCode) + "\n"
	path := filepath.Join(dir, "apt-get")
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return dir
}

func TestDownloadPackagesSuccess(t *testing.T) {
	prependFakeAptGet(t, 0, "")

	tmpPath, err := DownloadPackages([]string{"pkg1", "pkg2"}, nil, map[string]string{"APT::foo": "bar"})
	require.NoError(t, err)
	assert.NotEmpty(t, tmpPath)
	// 清理 DownloadPackages 创建的临时目录
	_ = os.RemoveAll(tmpPath)
}

func TestDownloadPackagesFailure(t *testing.T) {
	prependFakeAptGet(t, 100, "E: Failed to fetch http://example.com/pkg.deb\n")

	_, err := DownloadPackages([]string{"pkg1"}, nil, nil)
	require.Error(t, err)
}
