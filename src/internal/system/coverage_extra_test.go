// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckLockLockedExtra(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "locked.lock")
	require.NoError(t, os.WriteFile(fpath, []byte("x"), 0644))

	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 not available for lock test: %v", err)
	}
	// Hold an exclusive fcntl (POSIX) write lock on the file from a separate
	// process so that F_GETLK reports F_WRLCK.
	cmd := exec.Command(py, "-c", `
import fcntl, time, sys
f = open(sys.argv[1], "r+")
fcntl.lockf(f, fcntl.LOCK_EX)
print("ready", flush=True)
time.sleep(30)
`, fpath)

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, line, "ready")

	path, locked := CheckLock(fpath)
	assert.Equal(t, fpath, path)
	assert.True(t, locked)
}

func TestGetArchivesDirSuccessExtra(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "apt.conf")
	content := "Dir \"/aptroot\";\nDir::Cache \"var/cache/apt\";\nDir::Cache::archives \"archives/\";\n"
	require.NoError(t, os.WriteFile(cfg, []byte(content), 0644))

	dir, err := GetArchivesDir(cfg)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/aptroot", "var/cache/apt", "archives"), dir)
}

func TestGetArchivesDirEmptyDirExtra(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "apt.conf")
	require.NoError(t, os.WriteFile(cfg, []byte("Dir \"\";\n"), 0644))

	dir, err := GetArchivesDir(cfg)
	require.Error(t, err)
	assert.Empty(t, dir)
}

func TestGetArchivesDirEmptyCacheExtra(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "apt.conf")
	require.NoError(t, os.WriteFile(cfg, []byte("Dir::Cache \"\";\n"), 0644))

	dir, err := GetArchivesDir(cfg)
	require.Error(t, err)
	assert.Empty(t, dir)
}

func TestGetArchivesDirEmptyArchivesExtra(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "apt.conf")
	require.NoError(t, os.WriteFile(cfg, []byte("Dir::Cache::archives \"\";\n"), 0644))

	dir, err := GetArchivesDir(cfg)
	require.Error(t, err)
	assert.Empty(t, dir)
}

func TestQueryPackageInstalledExtra(t *testing.T) {
	assert.True(t, QueryPackageInstalled("dpkg"))
	assert.False(t, QueryPackageInstalled("definitely-not-installed-pkg-xyz123"))
}

func TestQueryPackageInstallableExtra(t *testing.T) {
	assert.True(t, QueryPackageInstallable("dpkg"))
	assert.False(t, QueryPackageInstallable("definitely-not-installed-pkg-xyz123"))
}

func TestQueryPackageInstallableNoCandidateExtra(t *testing.T) {
	// "tigerbeetle" is left in config-files state on this system; apt-cache
	// show succeeds but policy reports "Candidate: (none)" in C locale.
	t.Setenv("LANG", "C.UTF-8")
	t.Setenv("LC_ALL", "C.UTF-8")
	assert.False(t, QueryPackageInstallable("tigerbeetle"))
}

func TestQueryPackageDownloadSizeEmptyExtra(t *testing.T) {
	need, all, err := QueryPackageDownloadSize(SystemUpdate)
	assert.Error(t, err)
	assert.Equal(t, float64(SizeDownloaded), need)
	assert.Equal(t, float64(SizeDownloaded), all)
}

func TestQuerySourceDownloadSizeNoMatchExtra(t *testing.T) {
	need, all, err := QuerySourceDownloadSize(UpdateType(0), nil)
	assert.Error(t, err)
	assert.Equal(t, float64(SizeDownloaded), need)
	assert.Equal(t, float64(SizeDownloaded), all)
}

func TestQuerySourceAddSizeNoMatchExtra(t *testing.T) {
	size, err := QuerySourceAddSize(UpdateType(0))
	assert.Error(t, err)
	assert.Equal(t, float64(SizeUnknown), size)
}

func TestCheckInstallAddSizeExtra(t *testing.T) {
	// updateType(0) makes QuerySourceAddSize fail with SizeUnknown(-1);
	// free space is always greater than -1, so the result is true.
	assert.True(t, CheckInstallAddSize(UpdateType(0)))
}

func TestRefreshSymlinksForSourceDirCreatesLinksExtra(t *testing.T) {
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	SetTempSourceDir(tempDir)
	defer ClearTempSourceDir()

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "a.list"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "b.list"), []byte("y"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "c.txt"), []byte("z"), 0644))

	RefreshSymlinksForSourceDir(sourceDir)

	for _, name := range []string{"a.list", "b.list"} {
		link := filepath.Join(tempDir, name)
		target, err := os.Readlink(link)
		require.NoError(t, err, name)
		assert.Equal(t, filepath.Join(sourceDir, name), target)
	}
	// non-.list file must not be symlinked
	_, err := os.Lstat(filepath.Join(tempDir, "c.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestRefreshSymlinksForSourceDirSourceDirMissingExtra(t *testing.T) {
	tempDir := t.TempDir()
	SetTempSourceDir(tempDir)
	defer ClearTempSourceDir()

	RefreshSymlinksForSourceDir(filepath.Join(t.TempDir(), "nonexistent-source"))

	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestRefreshSymlinksForSourceDirTempDirMissingExtra(t *testing.T) {
	sourceDir := t.TempDir()
	SetTempSourceDir(filepath.Join(t.TempDir(), "nonexistent-temp"))
	defer ClearTempSourceDir()

	RefreshSymlinksForSourceDir(sourceDir)
}

func TestCustomSourceWrapperNoMatchExtra(t *testing.T) {
	err := CustomSourceWrapper(UpdateType(0), func(path string, unref func()) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to match")
}

func TestCustomSourceWrapperNilActionSingleExtra(t *testing.T) {
	err := CustomSourceWrapper(SystemUpdate, nil)
	assert.Error(t, err)
	assert.Equal(t, "doRealAction is nil", err.Error())
}

func TestCustomSourceWrapperSingleExtra(t *testing.T) {
	var gotPath string
	var gotUnref func()
	err := CustomSourceWrapper(SystemUpdate, func(path string, unref func()) error {
		gotPath = path
		gotUnref = unref
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, GetCategorySourceMap()[SystemUpdate], gotPath)
	assert.Nil(t, gotUnref)
}

func TestRefreshSymlinksForSourceDirExistingLinksExtra(t *testing.T) {
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	SetTempSourceDir(tempDir)
	defer ClearTempSourceDir()

	// existing source .list files
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "keep.list"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "add.list"), []byte("y"), 0644))

	// 1) symlink pointing outside sourceDir -> must be left untouched (HasPrefix false)
	outsideLink := filepath.Join(tempDir, "outside.list")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "elsewhere.list"), outsideLink))

	// 2) symlink pointing into sourceDir at an existing file -> IsFileExist true, untouched
	insideLink := filepath.Join(tempDir, "keep.list")
	require.NoError(t, os.Symlink(filepath.Join(sourceDir, "keep.list"), insideLink))

	RefreshSymlinksForSourceDir(sourceDir)

	// outside link still points to its original target
	outsideTarget, err := os.Readlink(outsideLink)
	require.NoError(t, err)
	assert.NotContains(t, outsideTarget, sourceDir)

	// keep.list link unchanged
	keepTarget, err := os.Readlink(insideLink)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sourceDir, "keep.list"), keepTarget)

	// add.list got a new symlink
	addTarget, err := os.Readlink(filepath.Join(tempDir, "add.list"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sourceDir, "add.list"), addTarget)
}

func TestRefreshSymlinksForSourceDirRelinkExtra(t *testing.T) {
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	SetTempSourceDir(tempDir)
	defer ClearTempSourceDir()

	// sourceDir contains a broken symlink named "foo.list"; ReadDir lists it,
	// so sourceFileMap["foo.list"] is true, but os.Stat (IsFileExist) fails.
	require.NoError(t, os.Symlink(filepath.Join(sourceDir, "nonexistent-target"), filepath.Join(sourceDir, "foo.list")))

	// tempDir holds a stale symlink whose target has sourceDir prefix but whose
	// basename resolves to a non-existent file -> triggers remove + re-create.
	staleLink := filepath.Join(tempDir, "stale")
	require.NoError(t, os.Symlink(filepath.Join(sourceDir, "foo.list"), staleLink))

	RefreshSymlinksForSourceDir(sourceDir)

	target, err := os.Readlink(staleLink)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sourceDir, "foo.list"), target)
}

func TestCustomSourceWrapperNilActionMultiExtra(t *testing.T) {
	err := CustomSourceWrapper(SystemUpdate|SecurityUpdate, nil)
	assert.Error(t, err)
	assert.Equal(t, "doRealAction is nil", err.Error())
}

func TestCustomSourceWrapperMultiExtra(t *testing.T) {
	var gotPath string
	var gotUnref func()
	err := CustomSourceWrapper(SystemUpdate|SecurityUpdate, func(path string, unref func()) error {
		gotPath = path
		gotUnref = unref
		return nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, gotPath)
	assert.NotNil(t, gotUnref)
	defer ClearTempSourceDir()

	info, err := os.Stat(gotPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	gotUnref()
	_, err = os.Stat(gotPath)
	assert.True(t, os.IsNotExist(err))
}
