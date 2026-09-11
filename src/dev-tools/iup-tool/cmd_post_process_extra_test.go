// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPostProcessDataFileReadError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	processLogFiles = nil
	processDataFile = "/nonexistent/data.json"

	assert.Panics(t, func() { runPostProcess(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	processLogFiles = nil
	processDataFile = ""

	assert.Panics(t, func() { runPostProcess(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	processLogFiles = nil
	processDataFile = ""

	assert.Panics(t, func() { runPostProcess(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessArgsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/process", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	processLogFiles = nil
	processDataFile = ""
	processMessageType = MessageTypeInfo
	processDetail = "detail"

	runPostProcess(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}

func TestRunPostProcessDataFileSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	dataFile := filepath.Join(t.TempDir(), "status.json")
	require.NoError(t, os.WriteFile(dataFile, []byte(`{"type":"info","detail":"from file"}`), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	processLogFiles = nil
	processDataFile = dataFile

	runPostProcess(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}

func TestRunPostProcessLogFilesDelegation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	logFile := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(logFile, []byte("log line"), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	processLogFiles = []string{logFile}
	processDataFile = ""

	runPostProcess(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}

func TestUploadLogFilesMissingFile(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")

	assert.Panics(t, func() { uploadLogFiles([]string{"/nonexistent/app.log"}) })
	assert.Equal(t, 1, *code)
}

func TestUploadLogFilesTarError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")

	assert.Panics(t, func() { uploadLogFiles([]string{t.TempDir()}) })
	assert.Equal(t, 1, *code)
}

func TestUploadLogFilesResponseError(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(logFile, []byte("log line"), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")

	assert.Panics(t, func() { uploadLogFiles([]string{logFile}) })
	assert.Equal(t, 1, *code)
}

func TestUploadLogFilesDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	logFile := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(logFile, []byte("log line"), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")

	assert.Panics(t, func() { uploadLogFiles([]string{logFile}) })
	assert.Equal(t, 1, *code)
}

func TestUploadLogFilesSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	logFile := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(logFile, []byte("log line"), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")

	uploadLogFiles([]string{logFile})

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
