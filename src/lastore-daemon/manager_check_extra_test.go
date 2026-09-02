// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckTypeJobType(t *testing.T) {
	tests := []struct {
		input checkType
		want  string
	}{
		{firstCheck, "first check"},
		{secondCheck, "second check"},
		{all, "invalid type"},
		{checkType(0), "invalid type"},
		{checkType(99), "invalid type"},
	}
	for _, tt := range tests {
		got := tt.input.JobType()
		assert.Equal(t, tt.want, got)
	}
}

func TestFullUpgradeOptionJSONRoundTrip(t *testing.T) {
	orig := fullUpgradeOption{
		DoUpgrade:         true,
		DoUpgradeMode:     1,
		IsPowerOff:        false,
		PreGreeterCheck:   true,
		AfterGreeterCheck: false,
		UUID:              "test-uuid-1234",
		MajorUpgrade:      true,
	}

	data, err := json.Marshal(orig)
	assert.NoError(t, err)

	var decoded fullUpgradeOption
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, orig, decoded)
}

func TestFullUpgradeOptionZeroValue(t *testing.T) {
	var orig fullUpgradeOption
	data, err := json.Marshal(orig)
	assert.NoError(t, err)

	var decoded fullUpgradeOption
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.False(t, decoded.DoUpgrade)
	assert.Equal(t, "", decoded.UUID)
}
