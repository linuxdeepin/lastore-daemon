// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prependFakeBin 在 PATH 前插入一个可执行假命令,返回目录。
func prependFakeBin(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return dir
}

func TestWaitDpkgLockRelease(t *testing.T) {
	// 无锁(或锁文件不可读)时应立即返回,不阻塞
	WaitDpkgLockRelease()
	assert.True(t, true)
}

func TestWaitDpkgLockReleaseWaitThenRelease(t *testing.T) {
	oldCheck := checkLockFn
	oldInterval := dpkgLockWaitInterval
	t.Cleanup(func() {
		checkLockFn = oldCheck
		dpkgLockWaitInterval = oldInterval
	})

	dpkgLockWaitInterval = 0
	calls := 0
	checkLockFn = func(p string) (string, bool) {
		calls++
		if calls == 1 {
			return "locked", true
		}
		return "", false
	}
	WaitDpkgLockRelease()
	assert.Equal(t, 3, calls)
}

func TestWaitDpkgLockReleaseFrontendWait(t *testing.T) {
	oldCheck := checkLockFn
	oldInterval := dpkgLockWaitInterval
	t.Cleanup(func() {
		checkLockFn = oldCheck
		dpkgLockWaitInterval = oldInterval
	})

	dpkgLockWaitInterval = 0
	frontendCalls := 0
	checkLockFn = func(p string) (string, bool) {
		if p == "/var/lib/dpkg/lock" {
			return "", false
		}
		frontendCalls++
		if frontendCalls == 1 {
			return "locked-frontend", true
		}
		return "", false
	}
	WaitDpkgLockRelease()
	assert.Equal(t, 2, frontendCalls)
}

func TestListInstallPackagesExtra(t *testing.T) {
	prependFakeBin(t, "apt-get", `cat <<'EOF'
The following additional packages will be installed:
  foo bar baz
EOF
`)
	got, err := ListInstallPackages([]string{"foo", "bar", "baz"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"foo", "bar", "baz"}, got)
}

func TestGenOnlineUpdatePackagesByEmulateInstallExtra(t *testing.T) {
	prependFakeBin(t, "apt-get", `cat <<'EOF'
The following packages will be upgraded:
  pkg-a pkg-b
Inst pkg-a [1.0] (2.0 deepin stable)
Inst pkg-b [1.0] (2.0 deepin stable)
The following packages will be REMOVED:
  pkg-c
Remv pkg-c [1.0]
EOF
`)
	install, remove, err := GenOnlineUpdatePackagesByEmulateInstall([]string{"pkg-a"}, nil)
	require.NoError(t, err)
	assert.Contains(t, install, "pkg-a")
	assert.Contains(t, install, "pkg-b")
	assert.Contains(t, remove, "pkg-c")
}

func TestListDistUpgradePackagesExtra(t *testing.T) {
	prependFakeBin(t, "apt-get", `cat <<'EOF'
The following packages will be upgraded:
  pkg-a pkg-b
EOF
`)
	got, err := ListDistUpgradePackages(t.TempDir(), nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"pkg-a", "pkg-b"}, got)
}

func TestListDistUpgradePackagesMissingSourceExtra(t *testing.T) {
	_, err := ListDistUpgradePackages(filepath.Join(t.TempDir(), "no-such-source"), nil)
	assert.Error(t, err)
}
