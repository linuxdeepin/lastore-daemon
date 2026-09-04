// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package querydesktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// writeDesktopFile creates (with any missing parent dirs) a file under dir and
// returns its absolute path. It is a pure helper so score() tests never touch
// external binaries or system directories.
func writeDesktopFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writefile: %v", err)
	}
	return p
}

func TestScoreReadError(t *testing.T) {
	fs := DesktopFiles{PkgName: "nomatchpkg", Files: []string{"/nonexistent/path/foo.desktop"}}
	assert.Equal(t, -10, fs.score(0), "unreadable file should score -10")
}

// TestScoreBranches exercises every branch of score() using real temp files.
// All content is in-memory; no dpkg/DBus required.
func TestScoreBranches(t *testing.T) {
	dir := t.TempDir()

	empty := writeDesktopFile(t, dir, "empty.desktop", "")

	full := writeDesktopFile(t, dir, "pkgmatch.desktop", `[Desktop Entry]
Exec=/usr/bin/foo
TryExec=foo
Type=Application
StartupNotify=true
Icon=foo
`)

	noDisplay := writeDesktopFile(t, dir, "nodisplay.desktop", `[Desktop Entry]
Exec=/usr/bin/foo
NoDisplay=true
`)

	// XDG scan: path containing "applications" gets +10 (the code also injects
	// path.Join("", "applications") == "applications" from an empty env var).
	xdg := writeDesktopFile(t, dir, "applications/foo.desktop", "[Desktop Entry]\n")

	// black list paths
	xsessions := writeDesktopFile(t, dir, "xsessions/foo.desktop", "[Desktop Entry]\n")
	qtTemplates := writeDesktopFile(t, dir, "qtcreator/templates/foo.desktop", "[Desktop Entry]\n")
	autostart := writeDesktopFile(t, dir, "autostart/foo.desktop", "[Desktop Entry]\n")
	desktopBase := writeDesktopFile(t, dir, "desktop-base/foo.desktop", "[Desktop Entry]\n")
	xgreeters := writeDesktopFile(t, dir, "xgreeters/foo.desktop", "[Desktop Entry]\n")

	cases := []struct {
		name    string
		pkgName string
		path    string
		want    int
	}{
		{"empty file", "nomatchpkg", empty, -23},
		{"pkgname match with full features", "pkgmatch", full, 39},
		{"nodisplay true", "nomatchpkg", noDisplay, -102},
		{"xdg applications dir", "nomatchpkg", xdg, -2},
		{"xsessions blacklist", "nomatchpkg", xsessions, -22},
		{"qtcreator templates blacklist", "nomatchpkg", qtTemplates, -17},
		{"autostart blacklist", "nomatchpkg", autostart, -13},
		{"desktop-base blacklist", "nomatchpkg", desktopBase, -17},
		{"xgreeters blacklist", "nomatchpkg", xgreeters, -17},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			fs := DesktopFiles{PkgName: c.pkgName, Files: []string{c.path}}
			assert.Equal(t, c.want, fs.score(0))
		})
	}
}

func TestLessTieBreak(t *testing.T) {
	dir := t.TempDir()
	// Both files have identical (empty) content, so both score the same (-23).
	// The tie-break then compares filename length.
	short := writeDesktopFile(t, dir, "a.desktop", "")
	long := writeDesktopFile(t, dir, "much-longer-filename.desktop", "")

	fs := DesktopFiles{PkgName: "nomatchpkg", Files: []string{short, long}}
	// equal score: Less(i,j) == len(files[i]) > len(files[j])
	assert.False(t, fs.Less(0, 1), "shorter first should not be Less")
	assert.True(t, fs.Less(1, 0), "longer first should be Less")
}

func TestLessDifferentScores(t *testing.T) {
	dir := t.TempDir()
	low := writeDesktopFile(t, dir, "low.desktop", "")
	high := writeDesktopFile(t, dir, "high.desktop", `[Desktop Entry]
Exec=/usr/bin/x
Icon=x
`)

	fs := DesktopFiles{PkgName: "nomatchpkg", Files: []string{low, high}}
	// low scores -23, high scores 4.
	assert.True(t, fs.Less(0, 1), "lower score sorts first")
	assert.False(t, fs.Less(1, 0), "higher score does not sort before lower")
}

func TestBestOneEmpty(t *testing.T) {
	assert.Equal(t, "", DesktopFiles{}.BestOne())
}

func TestBestOneOrdering(t *testing.T) {
	dir := t.TempDir()
	low := writeDesktopFile(t, dir, "low.desktop", "")
	high := writeDesktopFile(t, dir, "high.desktop", `[Desktop Entry]
Exec=/usr/bin/x
Icon=x
`)

	fs := DesktopFiles{PkgName: "nomatchpkg", Files: []string{low, high}}
	assert.Equal(t, high, fs.BestOne(), "highest-scoring file should win")
}
