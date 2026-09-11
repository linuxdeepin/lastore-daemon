// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestGetWhetherGatherInfo(t *testing.T) {
	resp, err := getWhetherGatherInfo(&config.Config{})
	// Empty PlatformUrl yields an invalid relative URL; the entry path is covered.
	t.Logf("getWhetherGatherInfo: resp=%v err=%v", resp, err)
}

func TestPostHardwareInfo(t *testing.T) {
	err := postHardwareInfo(&config.Config{})
	t.Logf("postHardwareInfo returned: %v", err)
}

func TestMainPostHardwareInfo(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	c := cli.NewContext(&cli.App{}, set, nil)
	err := MainPostHardwareInfo(c)
	t.Logf("MainPostHardwareInfo returned: %v", err)
}

func TestMainGatherInfo(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	c := cli.NewContext(&cli.App{}, set, nil)
	err := MainGatherInfo(c)
	t.Logf("MainGatherInfo returned: %v", err)
}

func newGatherInfoCtx(t *testing.T) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	return cli.NewContext(&cli.App{}, set, nil)
}

func setGatherInfoResponse(t *testing.T, status int, body string) {
	t.Helper()
	old := getWhetherGatherInfoFn
	t.Cleanup(func() { getWhetherGatherInfoFn = old })
	getWhetherGatherInfoFn = func(c *config.Config) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
}

func TestMainGatherInfoNetworkError(t *testing.T) {
	old := getWhetherGatherInfoFn
	t.Cleanup(func() { getWhetherGatherInfoFn = old })
	getWhetherGatherInfoFn = func(c *config.Config) (*http.Response, error) {
		return nil, errors.New("network down")
	}

	assert.Error(t, MainGatherInfo(newGatherInfoCtx(t)))
}

func TestMainGatherInfoNoFill(t *testing.T) {
	setGatherInfoResponse(t, http.StatusOK, `{"data":{"fill":false,"custom":false}}`)
	assert.NoError(t, MainGatherInfo(newGatherInfoCtx(t)))
}

func TestMainGatherInfoFillCustom(t *testing.T) {
	// Fill && Custom triggers /usr/bin/dde-gather-info, which is absent in the
	// test environment; its run error is swallowed and nil is returned.
	setGatherInfoResponse(t, http.StatusOK, `{"data":{"fill":true,"custom":true}}`)
	assert.NoError(t, MainGatherInfo(newGatherInfoCtx(t)))
}

func TestMainGatherInfoBadJSON(t *testing.T) {
	setGatherInfoResponse(t, http.StatusOK, `{not-json}`)
	assert.Error(t, MainGatherInfo(newGatherInfoCtx(t)))
}

func TestMainGatherInfoNon200(t *testing.T) {
	setGatherInfoResponse(t, http.StatusNotFound, ``)
	assert.NoError(t, MainGatherInfo(newGatherInfoCtx(t)))
}

func TestMainGatherInfoReadError(t *testing.T) {
	old := getWhetherGatherInfoFn
	t.Cleanup(func() { getWhetherGatherInfoFn = old })
	getWhetherGatherInfoFn = func(c *config.Config) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &failingReadCloser{},
		}, nil
	}

	assert.Error(t, MainGatherInfo(newGatherInfoCtx(t)))
}

// failingReadCloser returns an error on Read and is a no-op on Close.
type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }
