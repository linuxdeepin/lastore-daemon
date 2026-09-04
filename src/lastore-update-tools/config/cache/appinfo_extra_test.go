// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify(t *testing.T) {
	// valid AppInfo
	valid := AppInfo{
		Name:          "test-app",
		Version:       "1.0.0",
		Filename:      "test.deb",
		HashSha256:    "abc123",
		InstalledSize: 100,
		DebSize:       200,
	}
	assert.NoError(t, valid.Verify())

	// missing Name
	missingName := valid
	missingName.Name = ""
	assert.Error(t, missingName.Verify())

	// missing Version
	missingVersion := valid
	missingVersion.Version = ""
	assert.Error(t, missingVersion.Verify())

	// missing Filename
	missingFilename := valid
	missingFilename.Filename = ""
	assert.Error(t, missingFilename.Verify())

	// missing HashSha256
	missingHash := valid
	missingHash.HashSha256 = ""
	assert.Error(t, missingHash.Verify())

	// negative InstalledSize
	negInstalled := valid
	negInstalled.InstalledSize = -1
	assert.Error(t, negInstalled.Verify())

	// negative DebSize
	negDeb := valid
	negDeb.DebSize = -1
	assert.Error(t, negDeb.Verify())
}

func TestMerge(t *testing.T) {
	left := &AppInfo{
		Name:     "left-app",
		Version:  "1.0.0",
		Filename: "left.deb",
	}

	right := AppInfo{
		Name:     "", // empty, should not overwrite
		Version:  "2.0.0", // non-empty, should overwrite
		Filename: "right.deb", // non-empty, should overwrite
		Arch:     "amd64", // non-empty, should be set
	}

	err := left.Merge(right)
	assert.NoError(t, err)

	// Name should remain "left-app" (right was empty)
	assert.Equal(t, "left-app", left.Name)
	// Version should be overwritten
	assert.Equal(t, "2.0.0", left.Version)
	// Filename should be overwritten
	assert.Equal(t, "right.deb", left.Filename)
	// Arch should be set
	assert.Equal(t, "amd64", left.Arch)
}

func TestCheckFileExistTemp(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.deb")
	require.NoError(t, os.WriteFile(fp, []byte("data"), 0644))

	ai := &AppInfo{FilePath: dir, Filename: "test.deb"}
	assert.NoError(t, ai.CheckFileExist())

	aiMissing := &AppInfo{FilePath: dir, Filename: "missing.deb"}
	assert.Error(t, aiMissing.CheckFileExist())
}

func TestCompareHashSha256Temp(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.deb")
	content := []byte("hello world")
	require.NoError(t, os.WriteFile(fp, content, 0644))
	sum, err := fs.FileHashSha256(fp)
	require.NoError(t, err)

	ai := &AppInfo{FilePath: dir, Filename: "test.deb", HashSha256: sum}
	assert.NoError(t, ai.CompareHashSha256())

	aiBad := &AppInfo{FilePath: dir, Filename: "test.deb", HashSha256: "not-the-sum"}
	assert.Error(t, aiBad.CompareHashSha256())

	aiMissing := &AppInfo{FilePath: dir, Filename: "nope.deb", HashSha256: sum}
	assert.Error(t, aiMissing.CompareHashSha256())
}
