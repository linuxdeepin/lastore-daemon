// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package querydesktop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPkgsFilesEmpty(t *testing.T) {
	result := ListPkgsFiles(nil)
	assert.Nil(t, result, "ListPkgsFiles with empty pkgs should return nil")
}

func TestListPkgsFilesBash(t *testing.T) {
	result := ListPkgsFiles([]string{"bash"})
	// bash is installed, should return some files
	assert.NotNil(t, result, "ListPkgsFiles should return non-nil for installed package bash")
	assert.NotEmpty(t, result, "ListPkgsFiles should return non-empty list for bash")
}

func TestListPkgsFilesNonExistent(t *testing.T) {
	result := ListPkgsFiles([]string{"nonexistent-pkg-xyz123"})
	assert.Nil(t, result, "ListPkgsFiles should return nil for non-existent package")
}

func TestQuerySameSourcePkgsUnknown(t *testing.T) {
	// Before InitDB, maps are nil
	result := QuerySameSourcePkgs("unknown-pkg")
	assert.Nil(t, result, "QuerySameSourcePkgs should return nil for unknown package")
}

func TestQuerySameSourcePkgsAfterInitDB(t *testing.T) {
	InitDB()
	// After InitDB, try with bash which should be in the map
	// The result depends on dpkg-query output, just verify it doesn't panic
	result := QuerySameSourcePkgs("bash")
	_ = result
}

func TestListDesktopFilesEmptyPkg(t *testing.T) {
	result := ListDesktopFiles("nonexistent-pkg-xyz123")
	assert.Nil(t, result, "ListDesktopFiles should return nil for non-existent package")
}

func TestQueryRelateDependenciesNonExistent(t *testing.T) {
	stopCh := make(chan bool, 1)
	ch := queryRelateDependencies(stopCh, "nonexistent-pkg-xyz123", nil)

	var got []string
	for p := range ch {
		got = append(got, p)
	}
	// No dependencies resolvable, so only the package itself is emitted.
	assert.Equal(t, []string{"nonexistent-pkg-xyz123"}, got)
}

func TestQueryDesktopFilePathByDependenciesNonExistent(t *testing.T) {
	result := QueryDesktopFilePathByDependencies("nonexistent-pkg-xyz123")
	assert.Empty(t, result, "no desktop file should be found for a non-existent package")
}

func TestGroupBySource(t *testing.T) {
	s2b, b2s := groupBySource()
	// The function runs dpkg-query; on a system with packages installed it should return non-empty maps
	// Just verify it doesn't panic
	_ = s2b
	_ = b2s
}

// queryRelateDependencies walks package dependencies in a goroutine. The real
// implementation shells out to dpkg-query, so these tests inject the two
// package-level seams (queryPackageDepsFn / queryPackageInstalledFn) with
// in-memory fakes.
func TestQueryRelateDependenciesWalk(t *testing.T) {
	deps := map[string][]string{
		"root":    {"child-a"},
		"child-a": {"child-b"},
	}
	installed := map[string]bool{"child-a": true, "child-b": true}

	origDeps := queryPackageDepsFn
	origInstalled := queryPackageInstalledFn
	queryPackageDepsFn = func(pkg string) []string { return deps[pkg] }
	queryPackageInstalledFn = func(pkg string) bool { return installed[pkg] }
	t.Cleanup(func() {
		queryPackageDepsFn = origDeps
		queryPackageInstalledFn = origInstalled
	})

	stopCh := make(chan bool, 1)
	ch := queryRelateDependencies(stopCh, "root", nil)
	var got []string
	for p := range ch {
		got = append(got, p)
	}
	// The walk emits "root", then each newly-discovered installed dep. The
	// recursive inner loop re-emits the parent ("child-a") for its descendant
	// ("child-b") — that is the existing implementation's observable behaviour.
	assert.Equal(t, []string{"root", "child-a", "child-a"}, got)
}

func TestQueryRelateDependenciesAlreadySeen(t *testing.T) {
	// child-a depends back on root, which is already in the visited set, so it
	// is skipped instead of being emitted again.
	deps := map[string][]string{
		"root":    {"child-a"},
		"child-a": {"root"},
	}
	installed := map[string]bool{"child-a": true, "root": true}

	origDeps := queryPackageDepsFn
	origInstalled := queryPackageInstalledFn
	queryPackageDepsFn = func(pkg string) []string { return deps[pkg] }
	queryPackageInstalledFn = func(pkg string) bool { return installed[pkg] }
	t.Cleanup(func() {
		queryPackageDepsFn = origDeps
		queryPackageInstalledFn = origInstalled
	})

	stopCh := make(chan bool, 1)
	ch := queryRelateDependencies(stopCh, "root", nil)
	var got []string
	for p := range ch {
		got = append(got, p)
	}
	assert.Equal(t, []string{"root", "child-a"}, got)
}

func TestQueryRelateDependenciesNotInstalled(t *testing.T) {
	deps := map[string][]string{"root": {"notinstalled"}}
	installed := map[string]bool{"notinstalled": false}

	origDeps := queryPackageDepsFn
	origInstalled := queryPackageInstalledFn
	queryPackageDepsFn = func(pkg string) []string { return deps[pkg] }
	queryPackageInstalledFn = func(pkg string) bool { return installed[pkg] }
	t.Cleanup(func() {
		queryPackageDepsFn = origDeps
		queryPackageInstalledFn = origInstalled
	})

	stopCh := make(chan bool, 1)
	ch := queryRelateDependencies(stopCh, "root", nil)
	var got []string
	for p := range ch {
		got = append(got, p)
	}
	assert.Equal(t, []string{"root"}, got, "uninstalled dependency should be skipped")
}

func TestQueryRelateDependenciesStop(t *testing.T) {
	deps := map[string][]string{"root": {"child-a"}}
	installed := map[string]bool{"child-a": true}

	origDeps := queryPackageDepsFn
	origInstalled := queryPackageInstalledFn
	queryPackageDepsFn = func(pkg string) []string { return deps[pkg] }
	queryPackageInstalledFn = func(pkg string) bool { return installed[pkg] }
	t.Cleanup(func() {
		queryPackageDepsFn = origDeps
		queryPackageInstalledFn = origInstalled
	})

	// Unbuffered stopCh: the send below blocks until the goroutine reaches its
	// select. The output channel already holds "root" (buffered size 1), so the
	// "ch <- p" case is not ready and <-stopCh is chosen deterministically.
	stopCh := make(chan bool)
	ch := queryRelateDependencies(stopCh, "root", nil)
	stopCh <- true

	var got []string
	for p := range ch {
		got = append(got, p)
	}
	assert.Equal(t, []string{"root"}, got)
}

func TestQueryDesktopFilePathByDependenciesMatch(t *testing.T) {
	depsDone := make(chan struct{})
	origList := listPackageFileFn
	origDeps := queryPackageDepsFn
	listPackageFileFn = func(pkg ...string) []string {
		return []string{"/usr/share/applications/mypkg.desktop", "/usr/share/applications/other.desktop"}
	}
	// The function returns early on a name match while its dependency goroutine
	// is still winding down; signal completion so cleanup never restores the
	// seam under a goroutine that is still reading it.
	queryPackageDepsFn = func(pkg string) []string {
		close(depsDone)
		return nil
	}
	t.Cleanup(func() {
		listPackageFileFn = origList
		queryPackageDepsFn = origDeps
	})

	got := QueryDesktopFilePathByDependencies("mypkg")
	<-depsDone
	assert.Equal(t, "/usr/share/applications/mypkg.desktop", got)
}

func TestQueryDesktopFilePathByDependenciesAppend(t *testing.T) {
	dir := t.TempDir()
	low := writeDesktopFile(t, dir, "low.desktop", "")
	high := writeDesktopFile(t, dir, "high.desktop", "[Desktop Entry]\nExec=/usr/bin/x\nIcon=x\n")

	origList := listPackageFileFn
	origDeps := queryPackageDepsFn
	listPackageFileFn = func(pkg ...string) []string {
		// Neither file matches "mypkg.desktop", so they are collected and the
		// best one is chosen at the end.
		return []string{low, high}
	}
	queryPackageDepsFn = func(pkg string) []string { return nil }
	t.Cleanup(func() {
		listPackageFileFn = origList
		queryPackageDepsFn = origDeps
	})

	assert.Equal(t, high, QueryDesktopFilePathByDependencies("mypkg"))
}

func TestListDesktopFilesFilters(t *testing.T) {
	origS2B := __S2B__
	origB2S := __B2S__
	__B2S__ = map[string]string{"mypkg": "mysrc"}
	__S2B__ = map[string][]string{"mysrc": {"mypkg"}}

	origList := listPkgsFilesFn
	listPkgsFilesFn = func(pkgs []string) []string {
		return []string{
			"/usr/share/applications/foo.desktop",
			"/usr/share/doc/foo/readme.txt",
			"/usr/share/doc/foo/changelog.gz",
		}
	}
	t.Cleanup(func() {
		__S2B__ = origS2B
		__B2S__ = origB2S
		listPkgsFilesFn = origList
	})

	assert.Equal(t, []string{"/usr/share/applications/foo.desktop"}, ListDesktopFiles("mypkg"))
}
