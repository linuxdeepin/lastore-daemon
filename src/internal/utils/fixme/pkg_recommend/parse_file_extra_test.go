// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package pkg_recommend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonDependentCategoriesGetInfos(t *testing.T) {
	categories := jsonDependentCategories{
		{Category: "tr", Infos: jsonDependentInfos{{LangCode: "zh", Dependent: "pkg1", PkgPull: "pull1"}}},
		{Category: "fn", Infos: jsonDependentInfos{{LangCode: "en", Dependent: "pkg2", PkgPull: "pull2"}}},
	}
	infos := categories.GetInfos("tr")
	assert.NotNil(t, infos)
	assert.Equal(t, "pkg1", infos[0].Dependent)

	infos2 := categories.GetInfos("nonexistent")
	assert.Nil(t, infos2)
}

func TestJsonDependentInfoGetPackagesByLangInfo(t *testing.T) {
	info := &jsonDependentInfo{PkgPull: "firefox-l10n-"}
	tests := []struct {
		name        string
		locale      string
		langCode    string
		countryCode string
		variant     string
		wantLen     int
	}{
		{"locale only", "zh_CN", "zh", "", "", 3},
		{"with country", "zh_CN", "zh", "CN", "", 5},
		{"with variant", "zh_CN@latin", "zh", "CN", "latin", 6},
		{"empty langCode", "zh_CN", "", "", "", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := info.getPackagesByLangInfo(tt.locale, tt.langCode, tt.countryCode, tt.variant)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestGetDependentCategories(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "depends.json")
	content := `{"PkgDepends": [{"Category": "tr", "PkgInfos": [{"LangCode": "zh", "DependentPkg": "pkg1", "PkgPull": "pull1"}]}]}`
	os.WriteFile(fpath, []byte(content), 0644)

	categories, err := getDependentCategories(fpath)
	require.NoError(t, err)
	assert.Len(t, categories, 1)
	assert.Equal(t, "tr", categories[0].Category)
}

func TestGetDependentCategoriesNonexistent(t *testing.T) {
	_, err := getDependentCategories("/nonexistent/file")
	assert.Error(t, err)
}

func TestJsonDependentCategoriesGetAllDependentInfos(t *testing.T) {
	categories := jsonDependentCategories{
		{Category: "tr", Infos: jsonDependentInfos{{Dependent: "tr-pkg", PkgPull: "tr-pull"}}},
		{Category: "wa", Infos: jsonDependentInfos{{Dependent: "wa-pkg", PkgPull: "wa-pull"}}},
		{Category: "fn", Infos: jsonDependentInfos{{Dependent: "fn-pkg", PkgPull: "fn-pull"}}},
	}
	// This will call GetLangCodeInfo which reads from LangInfoFile - may fail
	// but the nil LangCode path should still work
	infos := categories.GetAllDependentInfos("nonexistent_locale")
	// With no lang code match, the nil LangCode entries should be included
	assert.NotEmpty(t, infos)
}
func TestJsonDependentCategoriesGetDependentInfosNil(t *testing.T) {
	categories := jsonDependentCategories{
		{Category: "tr", Infos: jsonDependentInfos{{Dependent: "pkg1", PkgPull: "pull1"}}},
	}

	// non-existent key returns nil
	assert.Nil(t, categories.GetDependentInfos("nonexistent", "zh_CN"))

	// existing key returns the dependent infos (empty LangCode hits direct append branch)
	infos := categories.GetDependentInfos("tr", "zh_CN")
	assert.NotNil(t, infos)
	assert.Len(t, infos, 1)
}
