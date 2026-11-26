// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUpdateTargetWithValues(t *testing.T) {
	m := &UpdatePlatformManager{
		targetVersion:  "26.0",
		targetBaseline: "2600",
		checkTime:      "2026-01-01",
	}
	result := m.GetUpdateTarget()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "26.0")
	assert.Contains(t, result, "2600")
}


func TestPostStatusMessagePlatformDisabled(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{PlatformDisabled: Cfg.DisabledProcess},
	}
	assert.NotPanics(t, func() {
		m.PostStatusMessage(StatusMessage{Type: "info"}, false)
	})
}

func TestPostStatusMessageIntranetNoForce(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{
			UpdateProcessUpload: false,
			IntranetUpdate:      true,
		},
	}
	assert.NotPanics(t, func() {
		m.PostStatusMessage(StatusMessage{Type: "info"}, false)
	})
}


func TestPostStatusMessageIntranetForceUpload(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{
			UpdateProcessUpload: false,
			IntranetUpdate:      true,
		},
		targetBaseline: "2600",
	}
	assert.NotPanics(t, func() {
		m.PostStatusMessage(StatusMessage{Type: "info"}, true)
	})
}



func TestGetSystemMetaWithPkgs(t *testing.T) {
	m := &UpdatePlatformManager{
		TargetCorePkgs: map[string]system.PackageInfo{
			"pkg1": {Name: "pkg1"},
			"pkg2": {Name: "pkg2"},
		},
	}
	result := m.GetSystemMeta()
	assert.Len(t, result, 2)
	assert.Contains(t, result, "pkg1")
}

func TestGetSystemMetaEmpty(t *testing.T) {
	m := &UpdatePlatformManager{}
	result := m.GetSystemMeta()
	assert.Empty(t, result)
}

func TestReplaceVersionCacheWithSymlink(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realVersion, []byte("real-version-content"), 0644))
	require.NoError(t, os.Symlink(realVersion, CacheVersion))

	m := &UpdatePlatformManager{}
	m.ReplaceVersionCache()
	data, err := os.ReadFile(CacheVersion)
	require.NoError(t, err)
	assert.Equal(t, "real-version-content", string(data))
}



func TestGetCurrentBaseline(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realBaseline, []byte("[General]\nBaseline=2500\n"), 0644))
	assert.Equal(t, "2500", getCurrentBaseline())
}

func TestGetCurrentSystemType(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(realBaseline, []byte("[General]\nSystemType=Professional\n"), 0644))
	assert.Equal(t, "Professional", getCurrentSystemType())
}





func TestGetPlatformRepoSourcesWithDebSource(t *testing.T) {
	m := &UpdatePlatformManager{
		config:    &Cfg.Config{PlatformRepoComponents: "main"},
		repoInfos: []repoInfo{{Source: "deb http://example.com deepin main", Uri: "http://example.com", CodeName: "deepin"}},
	}
	result := m.GetPlatformRepoSources()
	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "deb http://example.com deepin main")
}

func TestGetPlatformRepoSourcesWithoutSource(t *testing.T) {
	m := &UpdatePlatformManager{
		config:    &Cfg.Config{PlatformRepoComponents: "main"},
		repoInfos: []repoInfo{{Uri: "http://example.com", CodeName: "deepin"}},
	}
	result := m.GetPlatformRepoSources()
	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "deb http://example.com deepin main")
}

func TestGetPlatformRepoSourcesEmptyConfig(t *testing.T) {
	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: "http://example.com", CodeName: "deepin"}},
	}
	result := m.GetPlatformRepoSources()
	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "main community commercial")
}

func TestUpdateSourceList(t *testing.T) {
	overridePathVars(t)
	m := &UpdatePlatformManager{
		config: &Cfg.Config{IntranetUpdate: true, PlatformUpdate: true},
		repoInfos: []repoInfo{
			{Source: "deb http://example.com deepin main\n"},
		},
	}
	m.UpdateSourceList()
}

func TestUpdateSourceListNoIntranet(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{IntranetUpdate: false, PlatformUpdate: true},
	}
	m.UpdateSourceList()
}

func TestUpdateBaselineCache(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(cacheBaseline, []byte("[General]\n"), 0644))

	m := &UpdatePlatformManager{
		targetBaseline:      "2600",
		systemTypeFromPlatform: "Professional",
		targetVersion:       "26.0",
	}
	m.UpdateBaselineCache()
	data, err := os.ReadFile(cacheBaseline)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "2600")
	assert.Contains(t, content, "Professional")
	assert.Contains(t, content, "26.0")
}

func TestGetToken(t *testing.T) {
	m := &UpdatePlatformManager{
		Token:  "existing-token",
		config: &Cfg.Config{},
	}
	assert.Equal(t, "existing-token", m.getToken())
}

func TestPostStatusMessageWithServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":true,"data":"","code":0}`))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:        &Cfg.Config{UpdateProcessUpload: true},
		requestUrl:    srv.URL,
		targetBaseline: "2600",
	}
	assert.NotPanics(t, func() {
		m.PostStatusMessage(StatusMessage{Type: "info", UpdateType: "system"}, false)
	})
}

func TestPostStatusMessageServerResultFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":false,"code":500,"msg":"error"}`))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:        &Cfg.Config{UpdateProcessUpload: true},
		requestUrl:    srv.URL,
		targetBaseline: "2600",
	}
	assert.NotPanics(t, func() {
		m.PostStatusMessage(StatusMessage{Type: "info"}, false)
	})
}

func TestPostStatusMessageBadResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:        &Cfg.Config{UpdateProcessUpload: true},
		requestUrl:    srv.URL,
		targetBaseline: "2600",
	}
	assert.NotPanics(t, func() {
		m.PostStatusMessage(StatusMessage{Type: "info"}, false)
	})
}

func TestPostUpdateLogFilesPlatformDisabled(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{PlatformDisabled: Cfg.DisabledProcess},
	}
	assert.NotPanics(t, func() {
		m.PostUpdateLogFiles(nil)
	})
}


func TestPostUpdateLogFilesWithServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":true,"data":"","code":0}`))
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "test.log")
	require.NoError(t, os.WriteFile(tmpFile, []byte("log content"), 0644))

	m := &UpdatePlatformManager{
		config:     &Cfg.Config{},
		requestUrl: srv.URL,
	}
	assert.NotPanics(t, func() {
		m.PostUpdateLogFiles([]string{tmpFile})
	})
}

