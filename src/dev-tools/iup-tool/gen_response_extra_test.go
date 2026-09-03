// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenCurrentPkgListsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/package", r.URL.Path)
		assert.Equal(t, "test-baseline", r.URL.Query().Get("baseline"))
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	resp, err := genCurrentPkgListsResponse(srv.URL, "token123", "test-baseline")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenCurrentPkgListsResponseBadURL(t *testing.T) {
	_, err := genCurrentPkgListsResponse("http://127.0.0.1:1", "token", "b1")
	assert.Error(t, err)
}

func TestGenCVEInfoResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/cve/sync", r.URL.Path)
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("synctime"))
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	resp, err := genCVEInfoResponse(srv.URL, "tok", "2024-01-01")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenUpdateLogResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/systemupdatelogs", r.URL.Path)
		assert.Equal(t, "b100", r.URL.Query().Get("baseline"))
		assert.Equal(t, "1", r.URL.Query().Get("isUnstable"))
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":[]}`)
	}))
	defer srv.Close()

	resp, err := genUpdateLogResponse(srv.URL, "tk", "b100", 1)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenUpdateLogResponseBadURL(t *testing.T) {
	_, err := genUpdateLogResponse("http://127.0.0.1:1", "tk", "b1", 1)
	assert.Error(t, err)
}

func TestGenVersionResponseMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/version", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		assert.NotEmpty(t, r.Header.Get("X-Packages"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestURL: srv.URL,
		Token:      "mytoken",
	}
	resp, err := m.genVersionResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenVersionResponseBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestURL: "http://127.0.0.1:1",
		Token:      "tok",
	}
	_, err := m.genVersionResponse()
	assert.Error(t, err)
}

func TestGenUpdatePolicyByToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(map[string]interface{}{
			"result": true,
			"code":   0,
			"data": map[string]interface{}{
				"tp": 1,
			},
		})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestURL: srv.URL,
		Token:      "tok",
	}
	err := m.genUpdatePolicyByToken()
	assert.NoError(t, err)
}

func TestGenUpdatePolicyByTokenError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestURL: "http://127.0.0.1:1",
		Token:      "tok",
	}
	err := m.genUpdatePolicyByToken()
	assert.Error(t, err)
}

func TestGenUpdatePolicyByTokenBadStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestURL: srv.URL,
		Token:      "tok",
	}
	err := m.genUpdatePolicyByToken()
	assert.Error(t, err)
}

func TestGetCurrentPkgListsData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		raw := json.RawMessage(`{"packages":{"core":[],"select":[],"freeze":[],"purge":[]}}`)
		result := getCurrentPkgListsData(raw)
		require.NotNil(t, result)
	})
	t.Run("invalid json", func(t *testing.T) {
		raw := json.RawMessage(`{bad}`)
		result := getCurrentPkgListsData(raw)
		assert.Nil(t, result)
	})
}

func TestGetCVEDataIupTool(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		raw := json.RawMessage(`{"data_time":"2024-01-01","cves":{},"pkg_cves":{}}`)
		result := getCVEData(raw)
		require.NotNil(t, result)
	})
	t.Run("invalid json", func(t *testing.T) {
		raw := json.RawMessage(`{bad}`)
		result := getCVEData(raw)
		assert.Nil(t, result)
	})
}

func TestGetUpdateLogDataIupTool(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		raw := json.RawMessage(`[{"en_log":"fix bug"}]`)
		result := getUpdateLogData(raw)
		require.NotNil(t, result)
		assert.Len(t, result, 1)
	})
	t.Run("invalid json", func(t *testing.T) {
		raw := json.RawMessage(`{bad}`)
		result := getUpdateLogData(raw)
		assert.Nil(t, result)
	})
}
