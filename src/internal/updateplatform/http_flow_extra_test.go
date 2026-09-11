// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
)

func newOKServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":{"status":"ok"}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGenPostProcessResponse(t *testing.T) {
	srv := newOKServer(t)

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		preBaseline:    "pre-baseline",
		targetBaseline: "target-baseline",
		Token:          "tok",
		config:         &Cfg.Config{},
	}
	filePath := filepath.Join(t.TempDir(), "proc.xz")
	resp, err := m.genPostProcessResponse(strings.NewReader("hello platform"), filePath)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestPostProcessEventMessage(t *testing.T) {
	srv := newOKServer(t)

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		targetBaseline: "target-baseline",
		Token:          "tok",
		config: &Cfg.Config{
			IntranetUpdate: true,
		},
	}
	m.PostProcessEventMessage(ProcessEvent{EventType: CheckEnv, EventContent: "check env"})
}

func TestPostProcessEventMessageDisabled(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate:   true,
			PlatformDisabled: Cfg.DisabledProcess,
		},
	}
	m.PostProcessEventMessage(ProcessEvent{}) // 应静默返回,不 panic
}

func TestPostStatusMessage(t *testing.T) {
	srv := newOKServer(t)

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		targetBaseline: "target-baseline",
		Token:          "tok",
		config: &Cfg.Config{
			UpdateProcessUpload: true,
		},
	}
	m.PostStatusMessage(StatusMessage{Type: "info", Detail: "ok"}, false)
}

func TestPostUpdateLogFiles(t *testing.T) {
	srv := newOKServer(t)

	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.log")
	f2 := filepath.Join(dir, "b.log")
	require.NoError(t, os.WriteFile(f1, []byte("log a"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("log b"), 0644))

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		preBaseline:    "pre-baseline",
		targetBaseline: "target-baseline",
		Token:          "tok",
		config:         &Cfg.Config{},
	}
	m.PostUpdateLogFiles([]string{f1, f2})
}

func TestPostSystemUpgradeMessage(t *testing.T) {
	srv := newOKServer(t)

	old := postContentCacheDir
	postContentCacheDir = t.TempDir()
	t.Cleanup(func() { postContentCacheDir = old })

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"uuid-1": {
				Uuid:          "uuid-1",
				UpgradeStatus: UpgradeSucceed,
				PostStatus:    WaitPost,
			},
		},
	}
	m.PostSystemUpgradeMessage("uuid-1")
	assert.Len(t, m.jobPostMsgMap, 0)
}

func TestPostSystemUpgradeMessageMissingUUID(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl:    "http://127.0.0.1:1",
		Token:         "tok",
		config:        &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{},
	}
	m.PostSystemUpgradeMessage("not-exist") // 应静默返回
}

func TestPostUpgradeStatus(t *testing.T) {
	srv := newOKServer(t)

	old := postContentCacheDir
	postContentCacheDir = t.TempDir()
	t.Cleanup(func() { postContentCacheDir = old })

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		Token:          "tok",
		config:         &Cfg.Config{},
		preBuild:       "pre-build",
		targetVersion:  "target-version",
		targetBaseline: "target-baseline",
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"uuid-1": {
				Uuid:          "uuid-1",
				UpgradeStatus: UpgradeSucceed,
				PostStatus:    WaitPost,
			},
		},
	}
	m.PostUpgradeStatus("uuid-1", UpgradeSucceed, "")
	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)
}
