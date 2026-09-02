// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestBuildUpgradeInfoRegexExtra(t *testing.T) {
	archs := []system.Architecture{"amd64", "arm64"}
	re := buildUpgradeInfoRegex(archs)
	assert.NotNil(t, re)

	line := "pkgname/stable 1.2.3 amd64 [upgradable from: 1.2.2]"
	assert.True(t, re.MatchString(line))

	line2 := "pkgname/stable 1.2.3 all [upgradable from: 1.2.2]"
	assert.True(t, re.MatchString(line2))

	line3 := "pkgname/stable 1.2.3 i386 [upgradable from: 1.2.2]"
	assert.False(t, re.MatchString(line3))
}

func TestBuildUpgradeInfoExtra(t *testing.T) {
	archs := []system.Architecture{"amd64"}
	re := buildUpgradeInfoRegex(archs)

	t.Run("valid line", func(t *testing.T) {
		line := "pkgname/stable 1.2.3 amd64 [upgradable from: 1.2.2]"
		info := buildUpgradeInfo(re, line)
		assert.NotNil(t, info)
		assert.Equal(t, "pkgname", info.Package)
		assert.Equal(t, "1.2.3", info.LastVersion)
		assert.Equal(t, "1.2.2", info.CurrentVersion)
	})

	t.Run("no match", func(t *testing.T) {
		line := "not a valid line"
		info := buildUpgradeInfo(re, line)
		assert.Nil(t, info)
	})
}

func TestMapUpgradeInfoExtra(t *testing.T) {
	archs := []system.Architecture{"amd64"}
	re := buildUpgradeInfoRegex(archs)

	lines := []string{
		"pkg1/stable 1.0.0 amd64 [upgradable from: 0.9.0]",
		"invalid line",
		"pkg2/stable 2.0.0 amd64 [upgradable from: 1.0.0]",
	}
	infos := mapUpgradeInfo(lines, re, buildUpgradeInfo, "system")
	assert.Len(t, infos, 2)
	assert.Equal(t, "system", infos[0].Category)
	assert.Equal(t, "pkg1", infos[0].Package)
	assert.Equal(t, "pkg2", infos[1].Package)
}

func TestParseAptShowListExtra(t *testing.T) {
	input := "The following packages have unmet dependencies:\n pkg1\n pkg2\n"
	result := parseAptShowList(strings.NewReader(input), "The following packages have unmet dependencies:")
	assert.Equal(t, []string{"pkg1", "pkg2"}, result)
}

func TestParseAptShowListEmptyExtra(t *testing.T) {
	input := "no matching title here\n some line\n"
	result := parseAptShowList(strings.NewReader(input), "The following packages have unmet dependencies:")
	assert.Nil(t, result)
}
