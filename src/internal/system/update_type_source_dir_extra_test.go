// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/go-lib/strv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideDirVars 将 update_type.go 的仓库目录变量指向临时目录,并在测试结束后恢复。
func overrideDirVars(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldOrigin := OriginSourceDir
	oldSoft := SoftLinkSystemSourceDir
	oldSecurity := SecuritySourceDir
	oldUnknown := UnknownSourceDir
	oldOther := OtherSystemSourceDir
	oldCustom := CustomSourceDir
	oldLastore := LastoreSourcesPath
	oldSystemUpdate := SystemUpdateSource

	OriginSourceDir = filepath.Join(dir, "sources.list.d")
	SoftLinkSystemSourceDir = filepath.Join(dir, "SystemSource.d")
	SecuritySourceDir = filepath.Join(dir, "SecuritySource.d")
	UnknownSourceDir = filepath.Join(dir, "unknownSource.d")
	OtherSystemSourceDir = filepath.Join(dir, "otherSystemSource.d")
	CustomSourceDir = filepath.Join(dir, "sources.list.d-custom")
	LastoreSourcesPath = filepath.Join(dir, "sources.list")
	SystemUpdateSource = SoftLinkSystemSourceDir

	t.Cleanup(func() {
		OriginSourceDir = oldOrigin
		SoftLinkSystemSourceDir = oldSoft
		SecuritySourceDir = oldSecurity
		UnknownSourceDir = oldUnknown
		OtherSystemSourceDir = oldOther
		CustomSourceDir = oldCustom
		LastoreSourcesPath = oldLastore
		SystemUpdateSource = oldSystemUpdate
	})
}

// isSymlink 判断路径是否为软链接。
func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func TestUpdateSystemDefaultSourceDir(t *testing.T) {
	overrideDirVars(t)

	srcFile := filepath.Join(t.TempDir(), "system.list")
	require.NoError(t, os.WriteFile(srcFile, []byte("deb http://example.com beige main\n"), 0644))

	err := UpdateSystemDefaultSourceDir([]string{srcFile})
	require.NoError(t, err)

	linkPath := filepath.Join(SoftLinkSystemSourceDir, "system.list")
	assert.True(t, isSymlink(linkPath))
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestUpdateSecurityDefaultSourceDir(t *testing.T) {
	overrideDirVars(t)

	srcFile := filepath.Join(t.TempDir(), "security.list")
	require.NoError(t, os.WriteFile(srcFile, []byte("deb http://security.example.com beige-security main\n"), 0644))

	err := UpdateSecurityDefaultSourceDir([]string{srcFile})
	require.NoError(t, err)

	linkPath := filepath.Join(SecuritySourceDir, "security.list")
	assert.True(t, isSymlink(linkPath))
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestUpdateSourceDirUseUrl(t *testing.T) {
	overrideDirVars(t)

	repoUrls := []string{
		"deb https://packages.example.com/desktop beige main",
		"deb https://packages.example.com/commercial beige commercial",
	}
	err := UpdateSourceDirUseUrl(SystemUpdate, repoUrls, "platform.list", "test annotation")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(SoftLinkSystemSourceDir, "platform.list"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "test annotation")
	assert.Contains(t, string(data), "deb https://packages.example.com/desktop")
}

func TestUpdateUnknownSourceDir(t *testing.T) {
	overrideDirVars(t)

	// 构造 OriginSourceDir 下的 .list 文件
	require.NoError(t, os.MkdirAll(OriginSourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(OriginSourceDir, "custom.list"), []byte("deb http://custom.example.com beige main\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(OriginSourceDir, "appstore.list"), []byte("deb http://appstore.example.com beige main\n"), 0644))

	err := UpdateUnknownSourceDir(nil)
	require.NoError(t, err)

	// custom.list 不在默认 known 列表,应创建软链接
	assert.True(t, isSymlink(filepath.Join(UnknownSourceDir, "custom.list")))
	// appstore.list 在默认 known 列表,不应创建软链接
	assert.False(t, isSymlink(filepath.Join(UnknownSourceDir, "appstore.list")))
}

func TestUpdateOtherSystemSourceDir(t *testing.T) {
	overrideDirVars(t)

	srcFile := filepath.Join(t.TempDir(), "other.list")
	require.NoError(t, os.WriteFile(srcFile, []byte("deb http://other.example.com beige main\n"), 0644))

	err := UpdateOtherSystemSourceDir([]string{srcFile})
	require.NoError(t, err)

	linkPath := filepath.Join(OtherSystemSourceDir, "other.list")
	assert.True(t, isSymlink(linkPath))
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestUpdateOtherSystemSourceDirEmpty(t *testing.T) {
	overrideDirVars(t)

	err := UpdateOtherSystemSourceDir(nil)
	require.NoError(t, err)
}

func TestUpdateP2pDefaultSourceDir(t *testing.T) {
	overrideDirVars(t)

	// 构造 SystemUpdateSource 目录下的源文件
	require.NoError(t, os.MkdirAll(SystemUpdateSource, 0755))
	srcFile := filepath.Join(SystemUpdateSource, "p2p.list")
	require.NoError(t, os.WriteFile(srcFile, []byte("deb https://packages.example.com/desktop beige main\n"), 0644))

	platformRepos := []string{"deb https://packages.example.com/desktop beige main"}
	err := UpdateP2pDefaultSourceDir(SystemUpdate, platformRepos)
	require.NoError(t, err)

	// 源文件已被替换为 delivery 协议,输出文件名为 p2pSource-*.list
	entries, err := os.ReadDir(SystemUpdateSource)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	data, err := os.ReadFile(filepath.Join(SystemUpdateSource, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), "delivery://packages.example.com/desktop")
}

func TestUpdateSystemDefaultSourceDirDefaultList(t *testing.T) {
	overrideDirVars(t)

	err := UpdateSystemDefaultSourceDir(nil)
	require.NoError(t, err)

	for _, name := range []string{UnstableSourceList, "sources.list", HweSourceList} {
		assert.True(t, isSymlink(filepath.Join(SoftLinkSystemSourceDir, name)), name)
	}
}

func TestUpdateSecurityDefaultSourceDirDefaultList(t *testing.T) {
	overrideDirVars(t)

	err := UpdateSecurityDefaultSourceDir(nil)
	require.NoError(t, err)

	assert.True(t, isSymlink(filepath.Join(SecuritySourceDir, SecurityList)))
}

func TestUpdateSystemDefaultSourceDirSymlinkConflict(t *testing.T) {
	overrideDirVars(t)

	dirA := t.TempDir()
	dirB := t.TempDir()
	f1 := filepath.Join(dirA, "same.list")
	f2 := filepath.Join(dirB, "same.list")
	require.NoError(t, os.WriteFile(f1, []byte("x"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("y"), 0644))

	err := UpdateSystemDefaultSourceDir([]string{f1, f2})
	assert.Error(t, err)
}

func TestUpdateP2pDefaultSourceDirMissingSourceDir(t *testing.T) {
	overrideDirVars(t)

	err := UpdateP2pDefaultSourceDir(SystemUpdate, nil)
	assert.Error(t, err)
}

func TestUpdateP2pDefaultSourceDirSymlink(t *testing.T) {
	overrideDirVars(t)
	require.NoError(t, os.MkdirAll(SystemUpdateSource, 0755))

	// 真实目标文件位于 sourceDir 之外
	target := filepath.Join(t.TempDir(), "real.list")
	require.NoError(t, os.WriteFile(target, []byte("deb https://packages.example.com/desktop beige main\n"), 0644))
	require.NoError(t, os.Symlink(target, filepath.Join(SystemUpdateSource, "link.list")))

	// 指向不存在目标的软链接应被跳过
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "nonexistent.list"), filepath.Join(SystemUpdateSource, "broken.list")))

	platformRepos := []string{"deb https://packages.example.com/desktop beige main"}
	err := UpdateP2pDefaultSourceDir(SystemUpdate, platformRepos)
	require.NoError(t, err)

	entries, err := os.ReadDir(SystemUpdateSource)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	data, err := os.ReadFile(filepath.Join(SystemUpdateSource, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), "delivery://packages.example.com/desktop")
}

// redirectUnderFile makes path's parent a regular file so that RemoveAll and
// MkdirAll on it fail with ENOTDIR, exercising those error branches.
func redirectUnderFile(t *testing.T, target *string) {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0644))
	*target = filepath.Join(blocker, "sub")
}

func TestUpdateSourceDirUseUrlSecurity(t *testing.T) {
	overrideDirVars(t)

	repoUrls := []string{"deb https://security.example.com beige-security main"}
	err := UpdateSourceDirUseUrl(SecurityUpdate, repoUrls, "security.list", "security annotation")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(SecuritySourceDir, "security.list"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "security annotation")
	assert.Contains(t, string(data), "deb https://security.example.com")
}

func TestUpdateSourceDirUseUrlMkdirError(t *testing.T) {
	overrideDirVars(t)
	redirectUnderFile(t, &SecuritySourceDir)

	err := UpdateSourceDirUseUrl(SecurityUpdate, []string{"deb http://x beige main"}, "security.list", "x")
	assert.Error(t, err)
}

func TestUpdateSystemDefaultSourceDirMkdirError(t *testing.T) {
	overrideDirVars(t)
	redirectUnderFile(t, &SoftLinkSystemSourceDir)

	err := UpdateSystemDefaultSourceDir(nil)
	assert.Error(t, err)
}

func TestUpdateSecurityDefaultSourceDirSymlinkConflict(t *testing.T) {
	overrideDirVars(t)

	dirA := t.TempDir()
	dirB := t.TempDir()
	f1 := filepath.Join(dirA, "same.list")
	f2 := filepath.Join(dirB, "same.list")
	require.NoError(t, os.WriteFile(f1, []byte("x"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("y"), 0644))

	err := UpdateSecurityDefaultSourceDir([]string{f1, f2})
	assert.Error(t, err)
}

func TestUpdateSecurityDefaultSourceDirMkdirError(t *testing.T) {
	overrideDirVars(t)
	redirectUnderFile(t, &SecuritySourceDir)

	err := UpdateSecurityDefaultSourceDir(nil)
	assert.Error(t, err)
}

func TestUpdateUnknownSourceDirExplicitList(t *testing.T) {
	overrideDirVars(t)

	require.NoError(t, os.MkdirAll(OriginSourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(OriginSourceDir, "custom.list"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(OriginSourceDir, "known.list"), []byte("x"), 0644))

	err := UpdateUnknownSourceDir(strv.Strv{"known.list"})
	require.NoError(t, err)

	// custom.list is not in the provided nonUnknownList -> symlinked
	assert.True(t, isSymlink(filepath.Join(UnknownSourceDir, "custom.list")))
	// known.list is in nonUnknownList -> not symlinked
	assert.False(t, isSymlink(filepath.Join(UnknownSourceDir, "known.list")))
}

func TestUpdateUnknownSourceDirOriginMissing(t *testing.T) {
	overrideDirVars(t)

	// OriginSourceDir does not exist -> ReadDir fails
	err := UpdateUnknownSourceDir(nil)
	assert.Error(t, err)
}

func TestUpdateUnknownSourceDirMkdirError(t *testing.T) {
	overrideDirVars(t)
	redirectUnderFile(t, &UnknownSourceDir)

	err := UpdateUnknownSourceDir(nil)
	assert.Error(t, err)
}

func TestUpdateOtherSystemSourceDirSymlinkConflict(t *testing.T) {
	overrideDirVars(t)

	dirA := t.TempDir()
	dirB := t.TempDir()
	f1 := filepath.Join(dirA, "same.list")
	f2 := filepath.Join(dirB, "same.list")
	require.NoError(t, os.WriteFile(f1, []byte("x"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("y"), 0644))

	err := UpdateOtherSystemSourceDir([]string{f1, f2})
	assert.Error(t, err)
}

func TestUpdateOtherSystemSourceDirMkdirError(t *testing.T) {
	overrideDirVars(t)
	redirectUnderFile(t, &OtherSystemSourceDir)

	err := UpdateOtherSystemSourceDir([]string{filepath.Join(t.TempDir(), "x.list")})
	assert.Error(t, err)
}

func TestUpdateP2pDefaultSourceDirSymlinkToDir(t *testing.T) {
	overrideDirVars(t)
	require.NoError(t, os.MkdirAll(SystemUpdateSource, 0755))

	// A symlink pointing at a directory: ReadFile on the directory fails.
	targetDir := t.TempDir()
	require.NoError(t, os.Symlink(targetDir, filepath.Join(SystemUpdateSource, "dirlink.list")))

	err := UpdateP2pDefaultSourceDir(SystemUpdate, nil)
	assert.Error(t, err)
}

func TestUpdateP2pDefaultSourceDirSubdir(t *testing.T) {
	overrideDirVars(t)
	require.NoError(t, os.MkdirAll(SystemUpdateSource, 0755))
	// A real subdirectory (not a symlink) -> ReadFile on the directory fails.
	require.NoError(t, os.MkdirAll(filepath.Join(SystemUpdateSource, "subdir"), 0755))

	err := UpdateP2pDefaultSourceDir(SystemUpdate, nil)
	assert.Error(t, err)
}

func TestRefreshSymlinksForSourceDirCreateSymlinkError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root user to trigger symlink permission error")
	}

	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "a.list"), []byte("x"), 0644))
	SetTempSourceDir(tempDir)
	defer ClearTempSourceDir()

	// Read-only tempDir makes os.Symlink fail with EACCES.
	require.NoError(t, os.Chmod(tempDir, 0555))
	t.Cleanup(func() { _ = os.Chmod(tempDir, 0755) })

	RefreshSymlinksForSourceDir(sourceDir)

	_, err := os.Lstat(filepath.Join(tempDir, "a.list"))
	assert.True(t, os.IsNotExist(err))
}

func TestRefreshSymlinksForSourceDirRelinkSymlinkError(t *testing.T) {
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	// sourceDir lists a broken symlink named foo.list (IsFileExist false but present in ReadDir).
	require.NoError(t, os.Symlink(filepath.Join(sourceDir, "nonexistent-target"), filepath.Join(sourceDir, "foo.list")))
	// tempDir holds a stale symlink pointing into sourceDir.
	require.NoError(t, os.Symlink(filepath.Join(sourceDir, "foo.list"), filepath.Join(tempDir, "stale")))
	SetTempSourceDir(tempDir)
	defer ClearTempSourceDir()

	// Read-only tempDir makes the remove+recreate symlink step fail.
	require.NoError(t, os.Chmod(tempDir, 0555))
	t.Cleanup(func() { _ = os.Chmod(tempDir, 0755) })

	RefreshSymlinksForSourceDir(sourceDir)
}

func TestCustomSourceWrapperMultiMissingSource(t *testing.T) {
	overrideDirVars(t)

	// Neither SystemUpdateSource nor SecuritySourceDir exists -> os.Stat fails,
	// so the combined dir ends up empty.
	var gotPath string
	err := CustomSourceWrapper(SystemUpdate|SecurityUpdate, func(path string, unref func()) error {
		gotPath = path
		entries, e := os.ReadDir(path)
		require.NoError(t, e)
		assert.Empty(t, entries)
		if unref != nil {
			unref()
		}
		return nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, gotPath)
}

func TestCustomSourceWrapperMultiFileSource(t *testing.T) {
	overrideDirVars(t)

	// SystemUpdateSource is a regular file (not a dir) -> appended directly.
	require.NoError(t, os.WriteFile(SystemUpdateSource, []byte("deb http://x beige main\n"), 0644))

	err := CustomSourceWrapper(SystemUpdate|SecurityUpdate, func(path string, unref func()) error {
		entries, e := os.ReadDir(path)
		require.NoError(t, e)
		require.Len(t, entries, 1)
		if unref != nil {
			unref()
		}
		return nil
	})
	require.NoError(t, err)
}

func TestCustomSourceWrapperMultiActionError(t *testing.T) {
	overrideDirVars(t)
	require.NoError(t, os.MkdirAll(SystemUpdateSource, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(SystemUpdateSource, "a.list"), []byte("x"), 0644))

	err := CustomSourceWrapper(SystemUpdate|SecurityUpdate, func(path string, unref func()) error {
		if unref != nil {
			unref()
		}
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestCustomSourceWrapperMultiSymlinkConflict(t *testing.T) {
	overrideDirVars(t)
	require.NoError(t, os.MkdirAll(SystemUpdateSource, 0755))
	require.NoError(t, os.MkdirAll(SecuritySourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(SystemUpdateSource, "same.list"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(SecuritySourceDir, "same.list"), []byte("y"), 0644))

	err := CustomSourceWrapper(SystemUpdate|SecurityUpdate, func(path string, unref func()) error {
		return nil
	})
	assert.Error(t, err)
}
