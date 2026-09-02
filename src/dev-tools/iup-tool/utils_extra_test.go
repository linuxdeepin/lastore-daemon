// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMachineIDFromToken(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{"", ""},
		{"a=1;b=2;i=machine123;c=3", "machine123"},
		{"i=onlyID", "onlyID"},
		{"a=1;b=2", ""},
		{"x=y;i=abc;z=w", "abc"},
	}
	for _, tt := range tests {
		got := extractMachineIDFromToken(tt.token)
		assert.Equal(t, tt.want, got, "extractMachineIDFromToken(%q)", tt.token)
	}
}

func TestMarshalJSON(t *testing.T) {
	data := map[string]int{"a": 1}
	result, err := marshalJSON(data)
	assert.NoError(t, err)
	assert.Contains(t, string(result), `"a":1`)
}

func TestPKCS7Encode(t *testing.T) {
	tests := []struct {
		input     []byte
		blockSize int
		wantLen   int
	}{
		{[]byte("hello"), 32, 32},
		{make([]byte, 32), 32, 64},
		{[]byte("a"), 16, 16},
		{[]byte(""), 8, 8},
	}
	for _, tt := range tests {
		result := PKCS7Encode(tt.input, tt.blockSize)
		assert.Equal(t, tt.wantLen, len(result))
		assert.Equal(t, 0, len(result)%tt.blockSize)
	}
}

func TestGetRandomBytes(t *testing.T) {
	result, err := GetRandomBytes(16)
	assert.NoError(t, err)
	assert.Equal(t, 16, len(result))

	result2, err := GetRandomBytes(0)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result2))
}

func TestSubstr(t *testing.T) {
	tests := []struct {
		str    string
		start  int
		length int
		want   string
	}{
		{"hello world", 0, 5, "hello"},
		{"hello", 0, 10, "hello"},
		{"hello", 2, 3, "llo"},
		{"hello", 6, 3, ""},
		{"hello", -1, 3, "hel"},
		{"hello", 0, -1, ""},
	}
	for _, tt := range tests {
		got := Substr(tt.str, tt.start, tt.length)
		assert.Equal(t, tt.want, got, "Substr(%q, %d, %d)", tt.str, tt.start, tt.length)
	}
}

func TestEncryptMsg(t *testing.T) {
	data := []byte(`{"test":"value"}`)
	encrypted, err := EncryptMsg(data)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.True(t, len(encrypted)%BlockSize == 0)
}

func TestGetClientPackageInfo(t *testing.T) {
	result := getClientPackageInfo("test")
	assert.Contains(t, result, "client=lastore-daemon")
	assert.Contains(t, result, "version=")
}
