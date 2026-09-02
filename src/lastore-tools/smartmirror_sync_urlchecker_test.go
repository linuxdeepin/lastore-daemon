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

func TestNewURLChecker(t *testing.T) {
	c := NewURLChecker(2)
	assert.NotNil(t, c)
	assert.NotNil(t, c.result)
}

func TestURLCheckerCheckDuplicatePanic(t *testing.T) {
	c := NewURLChecker(1)
	url := "http://example.com/test"
	c.Check(url)

	assert.Panics(t, func() {
		c.Check(url)
	})
}

func TestURLCheckerResultMissingPanic(t *testing.T) {
	c := NewURLChecker(1)

	assert.Panics(t, func() {
		c.Result("http://nonexistent-url.com")
	})
}

func TestURLCheckerCheckAndResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewURLChecker(1)
	url := srv.URL + "/test"
	c.Check(url)
	c.SendAllRequest()

	r := c.Result(url)
	assert.NotNil(t, r)
	assert.Equal(t, url, r.URL)
	assert.True(t, r.Result)
}
