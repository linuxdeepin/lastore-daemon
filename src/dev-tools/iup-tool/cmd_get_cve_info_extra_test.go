// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunGetCVEInfoResponseError(t *testing.T) {
	restore, code := stubExit()
	defer restore()
	setPlatform("http://127.0.0.1:1", "tok")
	cveSyncTime = "2024-01-01"

	assert.Panics(t, func() { runGetCVEInfo(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCVEInfoDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":false,"code":500,"msg":"server error"}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	cveSyncTime = "2024-01-01"

	assert.Panics(t, func() { runGetCVEInfo(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCVEInfoParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":42}`)
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	cveSyncTime = "2024-01-01"

	assert.Panics(t, func() { runGetCVEInfo(nil, nil) })
	assert.Equal(t, 1, *code)
}

func TestRunGetCVEInfoSuccess(t *testing.T) {
	cves := make(map[string]CVEInfo, 11)
	for i := 1; i <= 11; i++ {
		id := fmt.Sprintf("CVE-2024-%04d", i)
		cves[id] = CVEInfo{ID: id, Description: "test", Severity: "high"}
	}
	payload := map[string]interface{}{
		"dataTime": "2024-01-01",
		"cves":     cves,
		"pkgCves":  map[string][]string{"pkg1": {"CVE-2024-0001"}},
	}
	data, _ := json.Marshal(map[string]interface{}{
		"result": true,
		"code":   0,
		"data":   payload,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/cve/sync", r.URL.Path)
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("synctime"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, string(data))
	}))
	defer srv.Close()

	restore, code := stubExit()
	defer restore()
	setPlatform(srv.URL, "tok")
	cveSyncTime = "2024-01-01"

	runGetCVEInfo(nil, nil)

	assert.Equal(t, 0, *code, "success path must not call osExit")
}
