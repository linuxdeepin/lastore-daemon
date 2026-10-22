// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
)

// TestMain 把 root 独占的 /run/lastore 与各类缓存重定向到测试临时目录，
// 避免单元测试以非 root 身份运行时误操作或依赖真实的运行时目录。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lastore-runtime-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create temp dir failed:", err)
		os.Exit(1)
	}
	runtimeDir := filepath.Join(dir, "lastore")
	if err := os.Mkdir(runtimeDir, 0700); err != nil {
		fmt.Fprintln(os.Stderr, "create runtime dir failed:", err)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	oldRuntimeDir := utils.RuntimeDirPath
	utils.RuntimeDirPath = runtimeDir
	lastoreUnitCache = filepath.Join(runtimeDir, "lastoreUnitCache")
	lastoreJobCacheJson = filepath.Join(runtimeDir, "lastoreJobCache.json")
	userAgentRecordPath = filepath.Join(runtimeDir, "lastoreAgentCache")

	code := m.Run()

	utils.RuntimeDirPath = oldRuntimeDir
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// secureTempDir 返回 0700 的临时目录：生产代码要求 /run/lastore 为 root 独占目录，
// 而 t.TempDir() 在本发行版按 umask 创建（0002 → 0775），不满足该契约。
func secureTempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lastore")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatalf("mkdir %s failed: %v", dir, err)
	}
	return dir
}
