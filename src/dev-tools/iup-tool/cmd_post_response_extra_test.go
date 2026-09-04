// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenPostProcessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/process", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, base64.RawStdEncoding.EncodeToString([]byte("token123")), r.Header.Get("X-Repo-Token"))
		assert.Equal(t, "machine-1", r.Header.Get("X-MachineID"))
		assert.Equal(t, "cur-base", r.Header.Get("X-CurrentBaseline"))
		assert.Equal(t, "tgt-base", r.Header.Get("X-Baseline"))
		assert.NotEmpty(t, r.Header.Get("X-Time"))
		assert.NotEmpty(t, r.Header.Get("X-Sign"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	updatePlatform.machineID = "machine-1"
	updatePlatform.currentBaseline = "cur-base"
	updatePlatform.targetBaseline = "tgt-base"

	filePath := filepath.Join(t.TempDir(), "update.xz")
	resp, err := genPostProcessResponse(srv.URL, "token123", bytes.NewBufferString(`{"type":"info"}`), filePath)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenPostProcessEventResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/process/events", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, base64.RawStdEncoding.EncodeToString([]byte("token123")), r.Header.Get("X-Repo-Token"))
		assert.Equal(t, "machine-1", r.Header.Get("X-MachineID"))
		assert.Equal(t, "cur-base", r.Header.Get("X-CurrentBaseline"))
		assert.Equal(t, "tgt-base", r.Header.Get("X-Baseline"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	updatePlatform.machineID = "machine-1"
	updatePlatform.currentBaseline = "cur-base"
	updatePlatform.targetBaseline = "tgt-base"

	event := &ProcessEvent{
		TaskID:       1,
		EventType:    StartDownload,
		EventStatus:  true,
		EventContent: "download started",
	}
	resp, err := genPostProcessEventResponse(srv.URL, "token123", event)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenPostResultResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/update/status", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	result := &UpgradePostMsg{
		SerialNumber:    "SN-1",
		MachineID:       "machine-1",
		UpgradeStatus:   UpgradeSucceed,
		UpgradeErrorMsg: "",
		Version:         "1.0.0",
		PreBaseline:     "pre-base",
		NextBaseline:    "next-base",
		TaskId:          42,
	}
	resp, err := genPostResultResponse(srv.URL, "token123", result)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenPostProcessResponseBadURL(t *testing.T) {
	updatePlatform.machineID = "machine-1"
	filePath := filepath.Join(t.TempDir(), "update.xz")
	_, err := genPostProcessResponse("http://127.0.0.1:1", "token", bytes.NewBufferString(`{}`), filePath)
	assert.Error(t, err)
}

func TestGenPostResultResponseBadURL(t *testing.T) {
	result := &UpgradePostMsg{TaskId: 1}
	_, err := genPostResultResponse("http://127.0.0.1:1", "token", result)
	assert.Error(t, err)
}

func TestGenPostProcessEventResponseBadURL(t *testing.T) {
	event := &ProcessEvent{TaskID: 1, EventType: CheckEnv}
	_, err := genPostProcessEventResponse("http://127.0.0.1:1", "token", event)
	assert.Error(t, err)
}

func TestGenPostResultResponseInvalidURL(t *testing.T) {
	result := &UpgradePostMsg{TaskId: 1}
	_, err := genPostResultResponse("http://127.0.0.1:1/\x00", "token", result)
	assert.Error(t, err)
}

func TestGenPostProcessEventResponseInvalidURL(t *testing.T) {
	event := &ProcessEvent{TaskID: 1, EventType: CheckEnv}
	_, err := genPostProcessEventResponse("http://127.0.0.1:1/\x00", "token", event)
	assert.Error(t, err)
}

func TestGenPostProcessResponseInvalidURL(t *testing.T) {
	updatePlatform.machineID = "machine-1"
	filePath := filepath.Join(t.TempDir(), "update.xz")
	_, err := genPostProcessResponse("http://127.0.0.1:1/\x00", "token", bytes.NewBufferString(`{}`), filePath)
	assert.Error(t, err)
}
