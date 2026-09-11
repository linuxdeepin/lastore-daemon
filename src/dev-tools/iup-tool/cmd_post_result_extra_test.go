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

func TestRunPostResultDataFileReadError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	resultDataFile = "/nonexistent/result.json"

	assert.Panics(t, func() { runPostResult(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostResultDataFileParseError(t *testing.T) {
	dataFile := filepath.Join(t.TempDir(), "result.json")
	require.NoError(t, os.WriteFile(dataFile, []byte(`{bad`), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	resultDataFile = dataFile

	assert.Panics(t, func() { runPostResult(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostResultResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	resultDataFile = ""

	assert.Panics(t, func() { runPostResult(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostResultDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	resultDataFile = ""

	assert.Panics(t, func() { runPostResult(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostResultArgsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/update/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	resultDataFile = ""
	resultStatus = int(UpgradeSucceed)

	runPostResult(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}

func TestRunPostResultDataFileSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	dataFile := filepath.Join(t.TempDir(), "result.json")
	require.NoError(t, os.WriteFile(dataFile, []byte(`{"taskId":1,"status":0}`), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	resultDataFile = dataFile

	runPostResult(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
