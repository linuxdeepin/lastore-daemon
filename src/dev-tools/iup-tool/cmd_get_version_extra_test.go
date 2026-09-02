// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersionData(t *testing.T) {
	msg := updateMessage{
		SystemType: "x86_64",
	}
	msg.Version.Baseline = "1.0.0"

	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	result := getVersionData(data)
	assert.NotNil(t, result)
	assert.Equal(t, "x86_64", result.SystemType)
	assert.Equal(t, "1.0.0", result.Version.Baseline)
}

func TestGetVersionDataInvalidJSON(t *testing.T) {
	result := getVersionData(json.RawMessage(`{invalid json`))
	assert.Nil(t, result)
}

func TestGetVersionDataEmpty(t *testing.T) {
	result := getVersionData(json.RawMessage(`{}`))
	assert.NotNil(t, result)
}
