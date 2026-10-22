// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureSecureDir(t *testing.T) {
	base := t.TempDir()

	fresh := filepath.Join(base, "fresh")
	require.NoError(t, EnsureSecureDir(fresh))
	fi, err := os.Lstat(fresh)
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
	assert.Zero(t, fi.Mode().Perm()&0077, "目录不应对属主以外的用户开放")

	// 同名普通文件
	plainFile := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(plainFile, []byte("x"), 0600))
	assert.Error(t, EnsureSecureDir(plainFile))

	// 其它用户可写入（显式 Chmod，避免 umask 022 把 0777 削成 0755 使该断言失去意义）
	openDir := filepath.Join(base, "open")
	require.NoError(t, os.Mkdir(openDir, 0700))
	require.NoError(t, os.Chmod(openDir, 0777))
	assert.Error(t, EnsureSecureDir(openDir))

	// 指向目录的符号链接：不允许借链接把 root 的文件写到别处
	realDir := filepath.Join(base, "real")
	require.NoError(t, os.Mkdir(realDir, 0700))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(realDir, link))
	assert.Error(t, EnsureSecureDir(link))
}

func TestEnsureRuntimeDir(t *testing.T) {
	old := RuntimeDirPath
	RuntimeDirPath = filepath.Join(t.TempDir(), "lastore")
	t.Cleanup(func() { RuntimeDirPath = old })

	require.NoError(t, EnsureRuntimeDir())
	fi, err := os.Lstat(RuntimeDirPath)
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
}
