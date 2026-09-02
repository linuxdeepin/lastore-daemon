// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/ratelimit"
)

func TestGenVersionResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/version", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "testtoken",
		config:     &Cfg.Config{},
	}
	resp, err := m.genVersionResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenVersionResponseBadURLUpdatePlatform(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	_, err := m.genVersionResponse()
	assert.Error(t, err)
}

func TestGenThrottlingResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/throttling/client", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	resp, err := m.genThrottlingResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenThrottlingResponseBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
	}
	_, err := m.genThrottlingResponse()
	assert.Error(t, err)
}

func TestGenCurrentPkgListsResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/package", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("baseline"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:  srv.URL,
		Token:       "tok",
		preBaseline: "pre-baseline-100",
	}
	resp, err := m.genCurrentPkgListsResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenCVEInfoResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/cve/sync", r.URL.Path)
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("synctime"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	resp, err := m.genCVEInfoResponse("2024-01-01")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenUpdateLogResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/systemupdatelogs", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("baseline"))
		assert.NotEmpty(t, r.URL.Query().Get("isUnstable"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":[]}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		Token:          "tok",
		targetBaseline: "target-200",
	}
	resp, err := m.genUpdateLogResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenIpfsConfigResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/ipfs/config")
		assert.Equal(t, "test-id", r.URL.Query().Get("id"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL + "/base",
		Token:      "tok",
		IPFSConfig: ratelimit.IPFSConfig{ID: "test-id"},
	}
	resp, err := m.genIpfsConfigResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenIpfsConfigResponseBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "://invalid-url",
	}
	_, err := m.genIpfsConfigResponse()
	assert.Error(t, err)
}

func TestGetResponseDataSuccess(t *testing.T) {
	body := `{"result":true,"code":0,"data":{"version":"1.0"}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	data, result, code, err := getResponseData(resp, GetVersion)
	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, 0, code)
	assert.NotNil(t, data)
}

func TestGetResponseDataNonOK(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("")),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	_, result, _, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.False(t, result)
}

func TestGetResponseDataResultFalse(t *testing.T) {
	body := `{"result":false,"code":416,"msg":"too many requests"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	_, result, code, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.False(t, result)
	assert.Equal(t, 416, code)
}

func TestGetResponseDataInvalidJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{bad json}")),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	_, _, _, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
}

func TestGenThrottlingByToken(t *testing.T) {
	throttlingData := map[string]interface{}{
		"result": true,
		"code":   0,
		"data": map[string]interface{}{
			"allDayRateLimit": map[string]interface{}{
				"startTime": "00:00:00",
				"endTime":   "23:59:59",
				"rateLimit": 10240,
				"type":      1,
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(throttlingData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	err := m.GenThrottlingByToken()
	assert.NoError(t, err)
}

func TestGenThrottlingByTokenError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
	}
	err := m.GenThrottlingByToken()
	assert.Error(t, err)
}

func TestGenThrottlingByTokenBadData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":"not-an-object"}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	err := m.GenThrottlingByToken()
	assert.Error(t, err)
}

func TestGenIpfsConfig(t *testing.T) {
	ipfsData := map[string]interface{}{
		"result": true,
		"code":   0,
		"data": map[string]interface{}{
			"id": "test-id",
			"dl": map[string]interface{}{
				"a": map[string]interface{}{
					"startTime": "00:00:00",
					"endTime":   "23:59:59",
					"rateLimit": 10240,
					"type":      1,
				},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(ipfsData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL + "/base",
		Token:      "tok",
	}
	err := m.GenIpfsConfig()
	assert.NoError(t, err)
}

func TestGenIpfsConfigError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
	}
	err := m.GenIpfsConfig()
	assert.Error(t, err)
}

func TestGetCVEUpdateLogs(t *testing.T) {
	m := &UpdatePlatformManager{
		cvePkgs: map[string][]string{
			"pkg1": {"CVE-2024-001", "CVE-2024-002"},
		},
	}
	CVEs = map[string]CEVInfo{
		"CVE-2024-001": {CveId: "CVE-2024-001"},
		"CVE-2024-002": {CveId: "CVE-2024-002"},
	}
	result := m.GetCVEUpdateLogs([]string{"pkg1", "pkg2"})
	assert.Len(t, result, 2)
}

func TestIsForceUpdateFunc(t *testing.T) {
	assert.False(t, IsForceUpdate(UnknownUpdate))
	assert.False(t, IsForceUpdate(NormalUpdate))
	assert.True(t, IsForceUpdate(UpdateNow))
	assert.True(t, IsForceUpdate(UpdateShutdown))
	assert.True(t, IsForceUpdate(UpdateRegularly))
}
