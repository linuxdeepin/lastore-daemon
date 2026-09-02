// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstrNormal(t *testing.T) {
	result := Substr("hello world", 0, 5)
	assert.Equal(t, "hello", result)
}

func TestSubstrStartOffset(t *testing.T) {
	result := Substr("hello world", 6, 5)
	assert.Equal(t, "world", result)
}

func TestSubstrExceedLength(t *testing.T) {
	result := Substr("hello", 0, 100)
	assert.Equal(t, "hello", result)
}

func TestSubstrNegativeStart(t *testing.T) {
	result := Substr("hello", -1, 3)
	assert.Equal(t, "hel", result)
}

func TestSubstrNegativeLength(t *testing.T) {
	result := Substr("hello", 0, -1)
	assert.Equal(t, "", result)
}

func TestSubstrStartBeyondLength(t *testing.T) {
	result := Substr("hello", 100, 3)
	assert.Equal(t, "", result)
}

func TestPKCS7Encode(t *testing.T) {
	data := []byte("test")
	result := PKCS7Encode(data, BlockSize)
	assert.True(t, len(result)%BlockSize == 0)
	assert.True(t, len(result) > len(data))
	paddingByte := result[len(result)-1]
	for i := len(data); i < len(result); i++ {
		assert.Equal(t, paddingByte, result[i])
	}
}

func TestPKCS7EncodeFullBlock(t *testing.T) {
	data := make([]byte, BlockSize)
	result := PKCS7Encode(data, BlockSize)
	assert.Equal(t, BlockSize*2, len(result))
}

func TestGetRandomBytes(t *testing.T) {
	result, err := GetRandomBytes(16)
	require.NoError(t, err)
	assert.Len(t, result, 16)
}

func TestGetRandomBytesZero(t *testing.T) {
	result, err := GetRandomBytes(0)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestEncryptMsg(t *testing.T) {
	data := []byte("test message")
	encrypted, err := EncryptMsg(data)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.True(t, len(encrypted)%BlockSize == 0)
	assert.False(t, bytes.Equal(data, encrypted))
}

func TestEncryptMsgEmpty(t *testing.T) {
	encrypted, err := EncryptMsg([]byte{})
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
}
