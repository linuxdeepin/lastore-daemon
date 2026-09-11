// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptMsgRandomError(t *testing.T) {
	old := randRead
	randRead = func([]byte) (int, error) { return 0, errors.New("entropy exhausted") }
	t.Cleanup(func() { randRead = old })

	_, err := EncryptMsg([]byte("payload"))
	assert.Error(t, err)
}

func TestGetResponseDataResultFalseInvalidErrorMsg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `[1,2,3]`)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// A JSON array cannot unmarshal into tokenMessage; Result stays false, and
	// the subsequent error-message unmarshal also fails, hitting that branch.
	data, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.Nil(t, data)
}
