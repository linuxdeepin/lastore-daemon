// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package pkg_recommend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSupportedLocaleTrue(t *testing.T) {
	// en_US.UTF-8 should be in /usr/share/i18n/SUPPORTED on most systems
	assert.True(t, IsSupportedLocale("en_US.UTF-8"))
}

func TestIsSupportedLocaleFalse(t *testing.T) {
	assert.False(t, IsSupportedLocale("xx_XX.INVALID"))
}

func TestGetSupportedLangInfos(t *testing.T) {
	infos, err := GetSupportedLangInfos()
	assert.NoError(t, err)
	// Should return at least some entries on a system with i18n data
	if len(infos) > 0 {
		found := false
		for _, info := range infos {
			if info.Locale == "en_US.UTF-8" {
				found = true
				assert.Equal(t, "en", info.LangCode)
			}
		}
		assert.True(t, found, "en_US.UTF-8 should be in supported lang infos")
	}
}
func TestGetLangInfosFromFile(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "lang.json")
		content := `{"LanguageList":[{"Locale":"en_US","Description":"English","LangCode":"en","CountryCode":"US"}]}`
		require.NoError(t, os.WriteFile(fpath, []byte(content), 0644))

		infos, err := getLangInfosFromFile(fpath)
		require.NoError(t, err)
		require.Len(t, infos, 1)
		assert.Equal(t, "en_US", infos[0].Locale)
		assert.Equal(t, "en", infos[0].LangCode)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := getLangInfosFromFile("/nonexistent/lang_info.json")
		assert.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "bad.json")
		require.NoError(t, os.WriteFile(fpath, []byte("{invalid"), 0644))

		_, err := getLangInfosFromFile(fpath)
		assert.Error(t, err)
	})
}
