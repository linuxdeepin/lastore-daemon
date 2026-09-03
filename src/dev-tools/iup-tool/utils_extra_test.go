// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

func TestGetResponseDataSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tokenMessage{
			Result: true,
			Code:   200,
			Data:   json.RawMessage(`{"key":"value"}`),
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := getResponseData(resp, GetVersion)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Equal(t, `{"key":"value"}`, string(data))
}

func TestGetResponseDataResultFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errMsg := tokenErrorMessage{
			Result: false,
			Code:   500,
			Msg:    "server error",
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(errMsg)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestGetResponseDataBadStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestGetResponseDataInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not json")
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestTarFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("hello"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("world"), 0644))

	outFile := filepath.Join(dir, "out.tar")
	err := tarFiles([]string{file1, file2}, outFile)
	assert.NoError(t, err)

	info, err := os.Stat(outFile)
	assert.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestTarFilesNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.tar")
	err := tarFiles([]string{filepath.Join(dir, "nonexistent.txt")}, outFile)
	assert.Error(t, err)
}

func TestTarFilesBadOutputPath(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("hello"), 0644))
	err := tarFiles([]string{file1}, "/nonexistent_dir/out.tar")
	assert.Error(t, err)
}

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
