// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	internalutils "github.com/linuxdeepin/lastore-daemon/src/internal/utils"
)

// redirectRuntimeDir 把 root 独占的 /run/lastore 指向测试临时目录（0700），
// 使依赖运行时目录的代码可在无 root 环境下执行。
func redirectRuntimeDir(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lastore")
	require.NoError(t, os.Mkdir(dir, 0700))
	old := internalutils.RuntimeDirPath
	internalutils.RuntimeDirPath = dir
	t.Cleanup(func() { internalutils.RuntimeDirPath = old })
}
