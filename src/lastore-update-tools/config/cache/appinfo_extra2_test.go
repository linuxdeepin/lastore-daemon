// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
