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

func TestRunGetCurrentPackagesBaselineRequired(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	currentPkgBaseline = ""

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	currentPkgBaseline = "b1"

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	currentPkgBaseline = "b1"

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":42}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	currentPkgBaseline = "b1"

	assert.Panics(t, func() { runGetCurrentPackages(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCurrentPackagesSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/package", r.URL.Path)
		assert.Equal(t, "b1", r.URL.Query().Get("baseline"))
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"result": true,
			"code": 0,
			"data": {
				"preCheck":  [{"name":"pre1","shell":"echo pre"}],
				"midCheck":  [{"name":"mid1","shell":"echo mid"}],
				"postCheck": [{"name":"post1","shell":"echo post"}],
				"packages": {
					"core":   [{"name":"core1","need":true,"allArchVersion":[{"arch":"amd64","version":"1.0"}]}],
					"select": [{"name":"sel1","need":false}],
					"freeze": [{"name":"frz1","need":false}],
					"purge":  [{"name":"prg1","need":false}]
				}
			}
		}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	currentPkgBaseline = "b1"

	runGetCurrentPackages(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
