// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package mirrors

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUnpublishedMirrorSources(t *testing.T) {
	mirrorData := unpublishedMirrors{
		Error: "",
		Mirrors: mirrors{
			{Id: "m1", Name: "Mirror1", Weight: 100, UrlHttp: "mirror1.com", Country: "CN"},
			{Id: "m2", Name: "Mirror2", Weight: 200, UrlHttps: "mirror2.com", Country: "US"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(mirrorData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	result, err := getUnpublishedMirrorSources(srv.URL)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "m2", result[0].Id)
	assert.Equal(t, "https://mirror2.com", result[0].Url)
	assert.Equal(t, "m1", result[1].Id)
	assert.Equal(t, "http://mirror1.com", result[1].Url)
}

func TestGetUnpublishedMirrorSourcesBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := getUnpublishedMirrorSources(srv.URL)
	assert.Error(t, err)
}

func TestGetUnpublishedMirrorSourcesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "{bad json}")
	}))
	defer srv.Close()

	_, err := getUnpublishedMirrorSources(srv.URL)
	assert.Error(t, err)
}

func TestGetUnpublishedMirrorSourcesConnectionError(t *testing.T) {
	_, err := getUnpublishedMirrorSources("http://127.0.0.1:1")
	assert.Error(t, err)
}

func TestLoadMirrorSourcesWithURL(t *testing.T) {
	mirrorList := mirrors{
		{Id: "m1", Name: "Mirror1", Weight: 100, UrlHttp: "m1.com"},
		{Id: "m2", Name: "Mirror2", Weight: 200, UrlHttps: "m2.com"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(mirrorList)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	result, err := LoadMirrorSources(srv.URL)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "m2", result[0].Id)
}

func TestLoadMirrorSourcesBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := LoadMirrorSources(srv.URL)
	assert.Error(t, err)
}

func TestLoadMirrorSourcesEmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "[]")
	}))
	defer srv.Close()

	_, err := LoadMirrorSources(srv.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetch mirrors")
}

func TestLoadMirrorSourcesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "{bad json}")
	}))
	defer srv.Close()

	_, err := LoadMirrorSources(srv.URL)
	assert.Error(t, err)
}

func TestLoadMirrorSourcesConnectionError(t *testing.T) {
	_, err := LoadMirrorSources("http://127.0.0.1:1")
	assert.Error(t, err)
}
