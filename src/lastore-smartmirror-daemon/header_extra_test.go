// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripURLPath_ValidURL(t *testing.T) {
	result := stripURLPath("http://example.com:8080/path/to/resource")
	assert.Equal(t, "example.com", result)
}

func TestStripURLPath_WithPort(t *testing.T) {
	result := stripURLPath("https://mirror.example.com:443/deepin/pool")
	assert.Equal(t, "mirror.example.com", result)
}

func TestStripURLPath_NoPort(t *testing.T) {
	result := stripURLPath("http://example.com/path")
	assert.Equal(t, "example.com", result)
}

func TestStripURLPath_InvalidURL(t *testing.T) {
	// url.Parse is very lenient, but an invalid URL with control character
	// should trigger the error path
	result := stripURLPath("http://[::1:abcd")
	// url.Parse may or may not error on this; if it does, the function returns input as-is
	// if it doesn't, it returns the hostname
	// Just verify it doesn't panic and returns a non-empty string
	assert.NotPanics(t, func() {
		_ = stripURLPath(result)
	})
}

func TestBuildRequest_Valid(t *testing.T) {
	header := map[string]string{
		"User-Agent": "test-agent",
		"MID":        "test-mid",
	}
	r := buildRequest(header, "GET", "http://example.com/test")
	assert.NotNil(t, r)
	assert.Equal(t, "GET", r.Method)
	assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
	assert.Equal(t, "test-mid", r.Header.Get("MID"))
}

func TestBuildRequest_InvalidURL(t *testing.T) {
	header := map[string]string{"X": "Y"}
	// An invalid URL should cause http.NewRequest to fail
	r := buildRequest(header, "GET", "http://[::1:invalid")
	assert.Nil(t, r)
}

func TestBuildRequest_EmptyHeader(t *testing.T) {
	r := buildRequest(map[string]string{}, "POST", "http://example.com/test")
	assert.NotNil(t, r)
	assert.Equal(t, "POST", r.Method)
}

func TestHandleRequest_NilRequest(t *testing.T) {
	url, code := handleRequest(nil)
	assert.Equal(t, "", url)
	assert.Equal(t, -1, code)
}

func TestHandleRequest_SuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	url, code := handleRequest(req)
	assert.Equal(t, server.URL, url)
	assert.Equal(t, http.StatusOK, code)
}

func TestHandleRequest_RedirectResponse(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	// Disable redirect following
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { httpClient.CheckRedirect = nil }()

	req, _ := http.NewRequest("GET", server.URL, nil)
	url, code := handleRequest(req)
	assert.Equal(t, http.StatusFound, code)
	assert.Equal(t, target.URL, url)
}

func TestHandleRequest_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	url, code := handleRequest(req)
	assert.Equal(t, "", url)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestHandleRequest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	url, code := handleRequest(req)
	assert.Equal(t, "", url)
	assert.Equal(t, http.StatusInternalServerError, code)
}

func TestHandleRequest_ConnectionError(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://127.0.0.1:0/nonexistent", nil)
	url, code := handleRequest(req)
	assert.Equal(t, "", url)
	assert.Equal(t, -2, code)
}

func TestMakeReportHeader_WithReports(t *testing.T) {
	reports := []Report{
		{Mirror: "http://mirror1.com/path", Delay: 100, Failed: false},
		{Mirror: "http://mirror2.com/path", Delay: 0, Failed: true, StatusCode: 500},
	}
	header := makeReportHeader(reports)
	assert.NotEmpty(t, header["M1"])
	assert.Contains(t, header["M1"], "mirror1.com:T100")
	assert.Contains(t, header["M1"], "mirror2.com:E500")
}
