// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplicationInfoJSONRoundTrip(t *testing.T) {
	orig := ApplicationInfo{
		Id:       "com.deepin.test",
		Category: "system",
		Icon:     "test-icon",
		Name:     "TestApp",
		LocaleName: map[string]string{
			"zh_CN": "测试应用",
			"en":    "TestApp",
		},
	}

	data, err := json.Marshal(orig)
	assert.NoError(t, err)

	var decoded ApplicationInfo
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, orig, decoded)
}

func TestApplicationInfoJSONTags(t *testing.T) {
	data := `{"id":"test","category":"utils","icon":"icon.png","name":"Test","locale_name":{"zh_CN":"测试"}}`
	var info ApplicationInfo
	err := json.Unmarshal([]byte(data), &info)
	assert.NoError(t, err)
	assert.Equal(t, "test", info.Id)
	assert.Equal(t, "utils", info.Category)
	assert.Equal(t, "icon.png", info.Icon)
	assert.Equal(t, "Test", info.Name)
	assert.Equal(t, "测试", info.LocaleName["zh_CN"])
}
