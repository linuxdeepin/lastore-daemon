// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTokenFromAptConfig(t *testing.T) {
	// apt-config should be available; the token may or may not be set
	// Just verify it doesn't panic and returns a string
	token := getTokenFromAptConfig()
	_ = token
}

func TestExtractMachineIDFromTokenEmpty(t *testing.T) {
	result := extractMachineIDFromToken("")
	assert.Empty(t, result, "extractMachineIDFromToken with empty token should return empty string")
}

func TestExtractMachineIDFromTokenNoMachineID(t *testing.T) {
	result := extractMachineIDFromToken("a=value;b=value;c=value")
	assert.Empty(t, result, "extractMachineIDFromToken without i= field should return empty string")
}

func TestExtractMachineIDFromTokenValid(t *testing.T) {
	result := extractMachineIDFromToken("a=system;b=product;i=machine123;c=edition")
	assert.Equal(t, "machine123", result, "extractMachineIDFromToken should extract machine ID")
}

func TestGetClientPackageInfoIupTool(t *testing.T) {
	result := getClientPackageInfo("lastore-daemon")
	assert.NotEmpty(t, result, "getClientPackageInfo should return non-empty string")
	assert.Contains(t, result, "client=")
}

func TestMarshalJSONIupTool(t *testing.T) {
	data := map[string]string{"key": "value"}
	bytes, err := marshalJSON(data)
	assert.NoError(t, err)
	assert.NotEmpty(t, bytes)
}

func TestNewHTTPClientIupTool(t *testing.T) {
	client := newHTTPClient()
	assert.NotNil(t, client)
}
