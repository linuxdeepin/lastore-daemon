// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	config "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainCheckPolicy(t *testing.T) {
	err := MainCheckPolicy(nil)
	t.Logf("MainCheckPolicy returned: %v", err)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// setupCheckPolicySeams installs a canned genVersionResponseFn and a temp cache
// file, returning a cleanup that restores all package-level seams it touched.
func setupCheckPolicySeams(t *testing.T, body string, status int) {
	t.Helper()
	oldGen := genVersionResponseFn
	oldCache := checkPolicyCacheFile
	oldBus := systemBusFn
	t.Cleanup(func() {
		genVersionResponseFn = oldGen
		checkPolicyCacheFile = oldCache
		systemBusFn = oldBus
	})

	genVersionResponseFn = func(c *config.Config) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	checkPolicyCacheFile = filepath.Join(t.TempDir(), "checkpolicy.cache")
	systemBusFn = func() (*dbus.Conn, error) { return nil, errors.New("no bus") }
}

func TestMainCheckPolicyNoChange(t *testing.T) {
	const body = "hello"
	setupCheckPolicySeams(t, body, http.StatusOK)

	// Pre-populate the cache with the same md5 sum so oldSum == newSum and the
	// D-Bus/notify branch is skipped.
	cache := checkPolicyCacheFile
	require.NoError(t, os.WriteFile(cache, []byte(md5Hex(body)+"\n"+
		time.Now().Format(time.RFC3339)+"\n"), 0600))

	require.NoError(t, MainCheckPolicy(nil))
}

func TestMainCheckPolicyChangedBusFails(t *testing.T) {
	const body = "hello"
	setupCheckPolicySeams(t, body, http.StatusOK)

	// A mismatched cached sum forces the write-cache + D-Bus branch; the
	// injected bus function fails fast, so the call degrades to a warning.
	cache := checkPolicyCacheFile
	require.NoError(t, os.WriteFile(cache, []byte("stale\n"), 0600))

	require.NoError(t, MainCheckPolicy(nil))

	// The cache file was rewritten with the new sum.
	data, err := os.ReadFile(cache)
	require.NoError(t, err)
	assert.Contains(t, string(data), md5Hex(body))
}

func TestMainCheckPolicyResponseError(t *testing.T) {
	oldGen := genVersionResponseFn
	oldCache := checkPolicyCacheFile
	t.Cleanup(func() {
		genVersionResponseFn = oldGen
		checkPolicyCacheFile = oldCache
	})
	genVersionResponseFn = func(c *config.Config) (*http.Response, error) {
		return nil, errors.New("network down")
	}
	checkPolicyCacheFile = filepath.Join(t.TempDir(), "checkpolicy.cache")

	assert.Error(t, MainCheckPolicy(nil))
}
