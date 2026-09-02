// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestGetFilterPackages_SingleType(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType: {"pkg1", "pkg2"},
	}
	result := getFilterPackages(infosMap, system.SystemUpdate)
	assert.Equal(t, []string{"pkg1", "pkg2"}, result)
}

func TestGetFilterPackages_MultipleTypes(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType:   {"pkg1"},
		system.SecurityUpgradeJobType: {"pkg2", "pkg3"},
	}
	result := getFilterPackages(infosMap, system.SystemUpdate|system.SecurityUpdate)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2", "pkg3"}, result)
}

func TestGetFilterPackages_NoMatch(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType: {"pkg1"},
	}
	result := getFilterPackages(infosMap, system.SecurityUpdate)
	assert.Empty(t, result)
}

func TestGetFilterPackages_EmptyMap(t *testing.T) {
	result := getFilterPackages(map[string][]string{}, system.SystemUpdate)
	assert.Empty(t, result)
}

func TestGetFilterPackages_AllInstallUpdate(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType:   {"pkg1"},
		system.SecurityUpgradeJobType: {"pkg2"},
		system.UnknownUpgradeJobType:  {"pkg3"},
	}
	result := getFilterPackages(infosMap, system.AllInstallUpdate)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2", "pkg3"}, result)
}

func TestInitConfig_PopulatesAllRepoTypes(t *testing.T) {
	sc := UpdateSourceConfig{}
	oemCfg := config.OemRepoConfig{
		RepoShowNameZh: "测试仓库",
		RepoShowNameEn: "Test Repo",
		RepoUrl:        []string{"deb http://example.com/repo stable main"},
	}
	customRepo := []string{"deb http://custom.com/repo stable main"}

	InitConfig(sc, oemCfg, customRepo)

	assert.NotNil(t, sc[config.OSDefaultRepo])
	assert.NotNil(t, sc[config.OemDefaultRepo])
	assert.NotNil(t, sc[config.CustomRepo])

	oemInfo := sc[config.OemDefaultRepo]
	assert.Equal(t, "测试仓库", oemInfo.RepoShowNameZh)
	assert.Equal(t, "Test Repo", oemInfo.RepoShowNameEn)
	assert.Equal(t, []string{"deb http://example.com/repo stable main"}, oemInfo.RepoConfig)

	customInfo := sc[config.CustomRepo]
	assert.Equal(t, []string{"deb http://custom.com/repo stable main"}, customInfo.RepoConfig)

	osInfo := sc[config.OSDefaultRepo]
	assert.Nil(t, osInfo.RepoConfig)
}

func TestSetUsingRepoType_SetsCorrectFlag(t *testing.T) {
	sc := UpdateSourceConfig{
		config.OSDefaultRepo:  &RepoInfo{IsUsing: true},
		config.OemDefaultRepo: &RepoInfo{IsUsing: false},
		config.CustomRepo:     &RepoInfo{IsUsing: false},
	}

	SetUsingRepoType(sc, config.OemDefaultRepo)

	assert.False(t, sc[config.OSDefaultRepo].IsUsing)
	assert.True(t, sc[config.OemDefaultRepo].IsUsing)
	assert.False(t, sc[config.CustomRepo].IsUsing)
}

func TestSetUsingRepoType_AllFalseExceptTarget(t *testing.T) {
	sc := UpdateSourceConfig{
		config.OSDefaultRepo:  &RepoInfo{IsUsing: true},
		config.OemDefaultRepo: &RepoInfo{IsUsing: true},
		config.CustomRepo:     &RepoInfo{IsUsing: true},
	}

	SetUsingRepoType(sc, config.CustomRepo)

	assert.False(t, sc[config.OSDefaultRepo].IsUsing)
	assert.False(t, sc[config.OemDefaultRepo].IsUsing)
	assert.True(t, sc[config.CustomRepo].IsUsing)
}
