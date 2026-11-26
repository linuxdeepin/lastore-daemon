// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	internalutils "github.com/linuxdeepin/lastore-daemon/src/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain sets RuntimeDirPath to a writable temp directory so that
// EnsureRuntimeDir succeeds in non-root test environments. This unblocks
// PostStatusMessage, PostUpdateLogFiles, genPostProcessResponse, and
// checkInReleaseFromPlatform test paths that previously failed silently.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lastore-test-runtime-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	old := internalutils.RuntimeDirPath
	internalutils.RuntimeDirPath = dir
	code := m.Run()
	internalutils.RuntimeDirPath = old
	os.Exit(code)
}

func TestUpdateCVEMetaDataSyncSuccess(t *testing.T) {
	overridePathVars(t)

	cveResp := CVEMeta{
		DateTime: "2026-01-01T00:00:00Z",
		Cves: []CEVInfo{
			{CveId: "CVE-2026-001", Binary: "pkg-a (<< 1.0)"},
		},
	}
	cveBytes, err := json.Marshal(cveResp)
	require.NoError(t, err)

	body := `{"result":true,"code":0,"data":` + string(cveBytes) + `}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
		cvePkgs:    make(map[string][]string),
	}
	err = m.updateCVEMetaDataSync()
	require.NoError(t, err)
	assert.Contains(t, CVEs, "CVE-2026-001")
}

func TestUpdateCVEMetaDataSyncHTTPError(t *testing.T) {
	overridePathVars(t)

	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
		cvePkgs:    make(map[string][]string),
	}
	err := m.updateCVEMetaDataSync()
	assert.Error(t, err)
}

func TestUpdateCVEMetaDataSyncNonOK(t *testing.T) {
	overridePathVars(t)

	body := `{"result":false,"code":500,"msg":"server error"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
		cvePkgs:    make(map[string][]string),
	}
	err := m.updateCVEMetaDataSync()
	assert.Error(t, err)
}

func TestGenIpfsConfigSuccess(t *testing.T) {
	ipfsJSON := `{"id":"test-id","gateway":["http://127.0.0.1:8080"],"api":"/api/v0"}`
	body := `{"result":true,"code":0,"data":` + ipfsJSON + `}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	err := m.GenIpfsConfig()
	require.NoError(t, err)
	assert.Equal(t, "test-id", m.IPFSConfig.ID)
}

func TestGenIpfsConfigBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "://bad-url",
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	err := m.GenIpfsConfig()
	assert.Error(t, err)
}

func TestGenIpfsConfigNonOK(t *testing.T) {
	body := `{"result":false,"code":500,"msg":"error"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	err := m.GenIpfsConfig()
	assert.Error(t, err)
}

func TestPostStatusMessageFullFlow(t *testing.T) {
	srv := newOKServer(t)

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		preBaseline:    "pre",
		targetBaseline: "target",
		Token:          "tok",
		config: &Cfg.Config{
			UpdateProcessUpload: true,
		},
	}
	m.PostStatusMessage(StatusMessage{Type: "info", Detail: "ok"}, false)
}

func TestPostUpdateLogFilesFullFlow(t *testing.T) {
	srv := newOKServer(t)

	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.log")
	require.NoError(t, os.WriteFile(f1, []byte("log a"), 0644))

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		preBaseline:    "pre",
		targetBaseline: "target",
		Token:          "tok",
		config:         &Cfg.Config{},
	}
	m.PostUpdateLogFiles([]string{f1})
}

func TestGetUpdateTargetAllFields(t *testing.T) {
	m := &UpdatePlatformManager{
		targetVersion:  "26.0",
		targetBaseline: "2600",
		checkTime:      "2026-11-26T10:00:00Z",
	}
	result := m.GetUpdateTarget()
	assert.Contains(t, result, `"TargetVersion":"2600"`)
	assert.Contains(t, result, `"TargetOsVersion":"26.0"`)
	assert.Contains(t, result, `"CheckTime":"2026-11-26T10:00:00Z"`)
}

func TestEncryptMsgInvalidKey(t *testing.T) {
	oldKey := encodingAesKey
	encodingAesKey = "short"
	t.Cleanup(func() { encodingAesKey = oldKey })

	_, err := EncryptMsg([]byte("test data"))
	assert.Error(t, err)
}

func TestPostSystemUpgradeMessageBadURL(t *testing.T) {
	old := postContentCacheDir
	postContentCacheDir = t.TempDir()
	t.Cleanup(func() { postContentCacheDir = old })

	m := &UpdatePlatformManager{
		requestUrl: "://bad-url",
		config:     &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"uuid-1": {Uuid: "uuid-1", PostStatus: WaitPost},
		},
	}
	assert.NotPanics(t, func() { m.PostSystemUpgradeMessage("uuid-1") })
}
