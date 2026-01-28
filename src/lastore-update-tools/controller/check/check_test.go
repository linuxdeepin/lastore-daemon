// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
)

var TmpBaseDir string

func ensureWritableBaseDir(t *testing.T) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "lastore-check-")
	if err != nil {
		t.Fatalf("unable to create temp dir: %v", err)
	}
	if !strings.HasSuffix(tmp, string(os.PathSeparator)) {
		tmp += string(os.PathSeparator)
	}
	TmpBaseDir = tmp
	oldBaseDir := CheckBaseDir
	CheckBaseDir = TmpBaseDir
	t.Cleanup(func() {
		CheckBaseDir = oldBaseDir
		_ = os.RemoveAll(strings.TrimSuffix(TmpBaseDir, string(os.PathSeparator)))
		TmpBaseDir = ""
	})
}

func dirHasEntries(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

func ensureCleanDir(t *testing.T, dir string) {
	t.Helper()
	if has, err := dirHasEntries(dir); err == nil && has {
		t.Skipf("dir %s has existing scripts; skip to avoid running real hooks", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir %s: %v", dir, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func writeScript(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write script %s: %v", path, err)
	}
}

func TestCheckDynHookAllTypes(t *testing.T) {
	ensureWritableBaseDir(t)

	cases := []struct {
		typ int8
		dir string
	}{
		{cache.PreUpdateCheck, filepath.Join(TmpBaseDir, "pre_update_check")},
		{cache.PostUpdateCheck, filepath.Join(TmpBaseDir, "post_update_check")},
		{cache.PreDownloadCheck, filepath.Join(TmpBaseDir, "pre_download_check")},
		{cache.PostDownloadCheck, filepath.Join(TmpBaseDir, "post_download_check")},
		{cache.PreBackupCheck, filepath.Join(TmpBaseDir, "pre_backup_check")},
		{cache.PostBackupCheck, filepath.Join(TmpBaseDir, "post_backup_check")},
		{cache.PreUpgradeCheck, filepath.Join(TmpBaseDir, "pre_check")},
		{cache.MidUpgradeCheck, filepath.Join(TmpBaseDir, "mid_check")},
		{cache.PostUpgradeCheck, filepath.Join(TmpBaseDir, "post_check")},
	}

	for _, c := range cases {
		if has, err := dirHasEntries(c.dir); err == nil && has {
			t.Skipf("dir %s has existing scripts; skip to avoid running real hooks", c.dir)
		}
	}

	for _, c := range cases {
		ensureCleanDir(t, c.dir)
		writeScript(t, c.dir, "hook.sh", "#!/bin/sh\necho ok\nexit 0\n")
		defer os.RemoveAll(c.dir)
	}

	for _, c := range cases {
		if err := CheckDynHook(c.typ); err != nil {
			t.Fatalf("CheckDynHook(%d) failed: %v", c.typ, err)
		}
	}
}

func TestCheckDynHookMissingDir(t *testing.T) {
	ensureWritableBaseDir(t)
	dir := filepath.Join(TmpBaseDir, "pre_update_check")
	if has, err := dirHasEntries(dir); err == nil && has {
		t.Skipf("dir %s has existing scripts; skip to avoid running real hooks", dir)
	}
	_ = os.RemoveAll(dir)
	if err := CheckDynHook(cache.PreUpdateCheck); err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
}

func TestCheckDynHookScriptFail(t *testing.T) {
	ensureWritableBaseDir(t)
	dir := filepath.Join(TmpBaseDir, "pre_backup_check")
	ensureCleanDir(t, dir)
	defer os.RemoveAll(dir)
	writeScript(t, dir, "fail.sh", "#!/bin/sh\nexit 1\n")

	if err := CheckDynHook(cache.PreBackupCheck); err == nil {
		t.Fatalf("expected error for failing hook, got nil")
	}
}

func TestCheckDynHookInvalidType(t *testing.T) {
	if err := CheckDynHook(99); err == nil {
		t.Fatalf("expected error for invalid check type, got nil")
	}
}
