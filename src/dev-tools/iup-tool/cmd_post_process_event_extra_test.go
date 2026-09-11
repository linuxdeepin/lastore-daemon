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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPostProcessEventDataFileReadError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	eventDataFile = "/nonexistent/event.json"

	assert.Panics(t, func() { runPostProcessEvent(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessEventDataFileParseError(t *testing.T) {
	dataFile := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(dataFile, []byte(`{bad`), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	eventDataFile = dataFile

	assert.Panics(t, func() { runPostProcessEvent(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessEventInvalidType(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	eventDataFile = ""
	eventType = ProcessEventType(0)

	assert.Panics(t, func() { runPostProcessEvent(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessEventResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	eventDataFile = ""
	eventType = CheckEnv

	assert.Panics(t, func() { runPostProcessEvent(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessEventDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	eventDataFile = ""
	eventType = CheckEnv

	assert.Panics(t, func() { runPostProcessEvent(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunPostProcessEventArgsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/process/events", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	eventDataFile = ""
	eventType = CheckEnv
	eventStatus = true
	eventContent = "hello"

	runPostProcessEvent(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}

func TestRunPostProcessEventDataFileSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	dataFile := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(dataFile, []byte(`{"taskID":1,"eventType":2,"eventStatus":true,"eventContent":"from file"}`), 0644))

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	eventDataFile = dataFile

	runPostProcessEvent(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}

func TestRunPostProcessEventContentTruncation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{}}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	eventDataFile = ""
	eventType = CheckEnv
	eventContent = strings.Repeat("a", 1000)

	runPostProcessEvent(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
