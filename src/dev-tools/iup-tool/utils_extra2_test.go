// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
