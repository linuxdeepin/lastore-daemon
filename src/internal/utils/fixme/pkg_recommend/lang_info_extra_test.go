// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package pkg_recommend

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
