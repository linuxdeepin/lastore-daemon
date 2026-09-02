// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckURLExistsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "lastore-tools", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.True(t, result.Result)
	assert.Equal(t, http.StatusOK, result.ResultCode)
	assert.Equal(t, srv.URL, result.URL)
	assert.GreaterOrEqual(t, result.Latency, int64(0))
}

func TestCheckURLExistsRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.True(t, result.Result)
	assert.Equal(t, http.StatusMovedPermanently, result.ResultCode)
}

func TestCheckURLExistsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.False(t, result.Result)
	assert.Equal(t, http.StatusNotFound, result.ResultCode)
}

func TestCheckURLExistsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	result := CheckURLExists(srv.URL)
	assert.False(t, result.Result)
	assert.Equal(t, http.StatusInternalServerError, result.ResultCode)
}

func TestCheckURLExistsInvalidURL(t *testing.T) {
	result := CheckURLExists("://invalid-url")
	assert.False(t, result.Result)
	assert.Equal(t, 0, result.ResultCode)
}

func TestCheckURLExistsConnectionError(t *testing.T) {
	result := CheckURLExists("http://127.0.0.1:1")
	assert.False(t, result.Result)
	assert.Equal(t, 0, result.ResultCode)
}

func TestParseIndex(t *testing.T) {
	indexData := []string{
		"http://mirror1.com",
		"http://mirror2.com",
		"http://mirror1.com",
		"http://mirror3.com",
		"http://mirror2.com",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(indexData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	result, err := ParseIndex(srv.URL)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Contains(t, result, "http://mirror1.com")
	assert.Contains(t, result, "http://mirror2.com")
	assert.Contains(t, result, "http://mirror3.com")
}

func TestParseIndexEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	result, err := ParseIndex(srv.URL)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseIndexInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{bad json}"))
	}))
	defer srv.Close()

	_, err := ParseIndex(srv.URL)
	assert.Error(t, err)
}

func TestParseIndexConnectionError(t *testing.T) {
	_, err := ParseIndex("http://127.0.0.1:1")
	assert.Error(t, err)
}

func TestParseIndexBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ParseIndex(srv.URL)
	assert.Error(t, err)
}
