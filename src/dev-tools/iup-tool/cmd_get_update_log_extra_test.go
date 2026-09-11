// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunGetUpdateLogBaselineRequired(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	updateLogBaseline = ""

	assert.Panics(t, func() { runGetUpdateLog(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetUpdateLogResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	updateLogBaseline = "b1"

	assert.Panics(t, func() { runGetUpdateLog(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetUpdateLogDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	updateLogBaseline = "b1"

	assert.Panics(t, func() { runGetUpdateLog(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetUpdateLogParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":42}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	updateLogBaseline = "b1"

	assert.Panics(t, func() { runGetUpdateLog(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetUpdateLogSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/systemupdatelogs", r.URL.Path)
		assert.Equal(t, "b1", r.URL.Query().Get("baseline"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"result": true,
			"code": 0,
			"data": [{"baseline":"b1","showVersion":"6.2","enLog":"fix","cnLog":"修复","logType":1}]
		}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	updateLogBaseline = "b1"

	runGetUpdateLog(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
