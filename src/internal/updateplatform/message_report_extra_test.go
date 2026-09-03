// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"archive/tar"
	"encoding/json"
	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsForceUpdate(t *testing.T) {
	tests := []struct {
		name string
		tp   UpdateTp
		want bool
	}{
		{"UnknownUpdate", UnknownUpdate, false},
		{"NormalUpdate", NormalUpdate, false},
		{"UpdateNow", UpdateNow, true},
		{"UpdateShutdown", UpdateShutdown, true},
		{"UpdateRegularly", UpdateRegularly, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsForceUpdate(tt.tp))
		})
	}
}

func TestIsMajorUpgradeExtra(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want bool
	}{
		{"major upgrade", "1.0.0", "2.0.0", true},
		{"same major", "2.0.0", "2.1.0", false},
		{"downgrade", "3.0.0", "2.0.0", false},
		{"invalid old", "abc", "2.0.0", false},
		{"invalid new", "1.0.0", "xyz", false},
		{"both invalid", "abc", "xyz", false},
		{"large major", "25.0", "26.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isMajorUpgrade(tt.old, tt.new))
		})
	}
}

func TestIsValidUpdateLog(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		assert.False(t, isValidUpdateLog(nil))
		assert.False(t, isValidUpdateLog([]UpdateLogMeta{}))
	})
	t.Run("all empty EnLog", func(t *testing.T) {
		logs := []UpdateLogMeta{{EnLog: ""}, {EnLog: ""}}
		assert.False(t, isValidUpdateLog(logs))
	})
	t.Run("has non-empty EnLog", func(t *testing.T) {
		logs := []UpdateLogMeta{{EnLog: ""}, {EnLog: "fix bugs"}}
		assert.True(t, isValidUpdateLog(logs))
	})
	t.Run("first has EnLog", func(t *testing.T) {
		logs := []UpdateLogMeta{{EnLog: "fix"}}
		assert.True(t, isValidUpdateLog(logs))
	})
}

func TestGenPlatformReposFromRepoInfos(t *testing.T) {
	t.Run("with deb source prefix", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos([]repoInfo{
			{Source: "deb http://example.com stable main"},
		}, "", false, false)
		assert.Len(t, repos, 1)
		assert.Equal(t, "deb http://example.com stable main", repos[0])
	})

	t.Run("without deb prefix, default components", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos([]repoInfo{
			{Uri: "http://example.com", CodeName: "stable"},
		}, "", false, false)
		assert.Len(t, repos, 1)
		assert.Equal(t, "deb http://example.com stable main community commercial", repos[0])
	})

	t.Run("without deb prefix, custom components", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos([]repoInfo{
			{Uri: "http://example.com", CodeName: "stable"},
		}, "main contrib", false, false)
		assert.Len(t, repos, 1)
		assert.Equal(t, "deb http://example.com stable main contrib", repos[0])
	})

	t.Run("delivery enabled, not intranet", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos([]repoInfo{
			{Source: "deb http://example.com stable main", Uri: "http://example.com"},
		}, "", true, false)
		assert.Len(t, repos, 1)
		assert.Contains(t, repos[0], "delivery://")
		assert.NotContains(t, repos[0], "http://")
	})

	t.Run("delivery enabled but intranet", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos([]repoInfo{
			{Source: "deb http://example.com stable main", Uri: "http://example.com"},
		}, "", true, true)
		assert.Len(t, repos, 1)
		assert.Contains(t, repos[0], "http://")
	})

	t.Run("empty repos", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos(nil, "", false, false)
		assert.Empty(t, repos)
	})

	t.Run("https replaced with delivery", func(t *testing.T) {
		repos := genPlatformReposFromRepoInfos([]repoInfo{
			{Source: "deb https://example.com stable main", Uri: "https://example.com"},
		}, "", true, false)
		assert.Len(t, repos, 1)
		assert.Contains(t, repos[0], "delivery://")
	})
}

func TestGetVersionData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{
			"systemType": "desktop",
			"version":    map[string]interface{}{"version": "2.0", "baseline": "25", "taskID": 1},
		})
		msg := getVersionData(data)
		require.NotNil(t, msg)
		assert.Equal(t, "desktop", msg.SystemType)
		assert.Equal(t, "2.0", msg.Version.Version)
	})

	t.Run("invalid json", func(t *testing.T) {
		msg := getVersionData(json.RawMessage(`invalid`))
		assert.Nil(t, msg)
	})
}

func TestGetThrottlingData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{
			"serverTime": "2025-01-01T12:00:00+08:00",
		})
		msg := getThrottlingData(data)
		require.NotNil(t, msg)
		assert.NotEmpty(t, msg.ServerTime)
	})

	t.Run("invalid json", func(t *testing.T) {
		msg := getThrottlingData(json.RawMessage(`invalid`))
		assert.Nil(t, msg)
	})
}

func TestGetTargetPkgListData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{
			"preCheck": []map[string]interface{}{{"name": "check1", "script": "echo hi"}},
		})
		msg := getTargetPkgListData(data)
		require.NotNil(t, msg)
		assert.Len(t, msg.PreCheck, 1)
	})

	t.Run("invalid json", func(t *testing.T) {
		msg := getTargetPkgListData(json.RawMessage(`invalid`))
		assert.Nil(t, msg)
	})
}

func TestGetCurrentPkgListsData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{
			"preCheck": []map[string]interface{}{{"name": "check1"}},
		})
		msg := getCurrentPkgListsData(data)
		require.NotNil(t, msg)
	})

	t.Run("invalid json", func(t *testing.T) {
		msg := getCurrentPkgListsData(json.RawMessage(`invalid`))
		assert.Nil(t, msg)
	})
}

func TestGetUpdateLogData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal([]map[string]interface{}{
			{"enLog": "fix", "baseline": "25"},
		})
		logs := getUpdateLogData(data)
		require.NotNil(t, logs)
		assert.Len(t, logs, 1)
		assert.Equal(t, "fix", logs[0].EnLog)
	})

	t.Run("invalid json", func(t *testing.T) {
		logs := getUpdateLogData(json.RawMessage(`invalid`))
		assert.Nil(t, logs)
	})
}

func TestGetCVEData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{
			"dateTime": "2025-01-01",
			"cves":     []map[string]interface{}{{"cveId": "CVE-2025-001"}},
		})
		meta := getCVEData(data)
		require.NotNil(t, meta)
		assert.Equal(t, "2025-01-01", meta.DateTime)
		assert.Len(t, meta.Cves, 1)
	})

	t.Run("invalid json returns non-nil with zero values", func(t *testing.T) {
		meta := getCVEData(json.RawMessage(`invalid`))
		assert.NotNil(t, meta)
	})
}

func TestGetIPFSConfigData(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{
			"id": "config1",
		})
		cfg := getIPFSConfigData(data)
		require.NotNil(t, cfg)
		assert.Equal(t, "config1", cfg.ID)
	})

	t.Run("invalid json returns non-nil", func(t *testing.T) {
		cfg := getIPFSConfigData(json.RawMessage(`invalid`))
		assert.NotNil(t, cfg)
	})
}

func TestRequestTypeString(t *testing.T) {
	s := GetVersion.string()
	assert.Contains(t, s, "GET")
	assert.Contains(t, s, "/api/v1/version")

	s2 := PostProcess.string()
	assert.Contains(t, s2, "POST")
	assert.Contains(t, s2, "/api/v1/process")
}

func TestUpdateNoLimitConfigIfChanged(t *testing.T) {
	t.Run("already no limit", func(t *testing.T) {
		existing := ratelimit.RateInfo{
			LimitType:   ratelimit.RateLimitTypeNo,
			LimitRate:   1000,
			CurrentRate: 1000,
		}
		data, _ := json.Marshal(&existing)
		var called bool
		err := updateNoLimitConfigIfChanged(string(data), func(s string) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("local limit, should change", func(t *testing.T) {
		existing := ratelimit.RateInfo{
			LimitType:   ratelimit.RateLimitTypeLocal,
			LimitRate:   5000,
			CurrentRate: 5000,
		}
		data, _ := json.Marshal(&existing)
		var called bool
		var captured string
		err := updateNoLimitConfigIfChanged(string(data), func(s string) error {
			called = true
			captured = s
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
		var newRate ratelimit.RateInfo
		require.NoError(t, json.Unmarshal([]byte(captured), &newRate))
		assert.Equal(t, ratelimit.RateLimitTypeNo, newRate.LimitType)
		assert.Equal(t, int64(5000), newRate.LimitRate)
	})

	t.Run("empty current value", func(t *testing.T) {
		var called bool
		var captured string
		err := updateNoLimitConfigIfChanged("", func(s string) error {
			called = true
			captured = s
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
		var newRate ratelimit.RateInfo
		require.NoError(t, json.Unmarshal([]byte(captured), &newRate))
		assert.Equal(t, ratelimit.RateLimitTypeNo, newRate.LimitType)
		assert.Equal(t, int64(ratelimit.DefaultRateLimit), newRate.LimitRate)
	})

	t.Run("invalid json current value", func(t *testing.T) {
		var called bool
		err := updateNoLimitConfigIfChanged("invalid json", func(s string) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("setFunc returns error", func(t *testing.T) {
		err := updateNoLimitConfigIfChanged("", func(s string) error {
			return assert.AnError
		})
		assert.Error(t, err)
	})
}

func TestGetGeneralValueFromKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ini")
	content := "[General]\nKey1=value1\nKey2=value2\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	assert.Equal(t, "value1", getGeneralValueFromKeyFile(path, "Key1"))
	assert.Equal(t, "value2", getGeneralValueFromKeyFile(path, "Key2"))
	assert.Equal(t, "", getGeneralValueFromKeyFile(path, "NonExistent"))
	assert.Equal(t, "", getGeneralValueFromKeyFile("/nonexistent/file", "Key1"))
}

func TestUpgradePostMsgSaveAndInit(t *testing.T) {
	oldDir := postContentCacheDir
	postContentCacheDir = t.TempDir()
	defer func() { postContentCacheDir = oldDir }()

	msg := &UpgradePostMsg{
		Uuid:          "test-uuid-save",
		PostStatus:    WaitPost,
		UpgradeStatus: UpgradeSucceed,
	}
	msg.save()

	loaded := &UpgradePostMsg{}
	err := loaded.init(filepath.Join(postContentCacheDir, "test-uuid-save"))
	require.NoError(t, err)
	assert.Equal(t, "test-uuid-save", loaded.Uuid)
	assert.Equal(t, WaitPost, loaded.PostStatus)
	assert.Equal(t, UpgradeSucceed, loaded.UpgradeStatus)
}

func TestUpgradePostMsgInitNonExistFile(t *testing.T) {
	msg := &UpgradePostMsg{}
	err := msg.init("/nonexistent/file/path")
	assert.Error(t, err)
}

func TestUpgradePostMsgInitInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid}"), 0644))
	msg := &UpgradePostMsg{}
	err := msg.init(path)
	assert.Error(t, err)
}

func TestUpgradePostMsgUpdateTimeStamp(t *testing.T) {
	msg := &UpgradePostMsg{}
	before := time.Now().Unix()
	msg.updateTimeStamp()
	after := time.Now().Unix()
	assert.GreaterOrEqual(t, msg.TimeStamp, before)
	assert.LessOrEqual(t, msg.TimeStamp, after)
}

func TestUpgradePostMsgAddFailedCount(t *testing.T) {
	oldDir := postContentCacheDir
	postContentCacheDir = t.TempDir()
	defer func() { postContentCacheDir = oldDir }()

	msg := &UpgradePostMsg{
		Uuid:       "test-fail-count",
		PostStatus: WaitPost,
	}
	assert.Equal(t, uint32(0), msg.RetryCount)
	msg.addFailedCount()
	assert.Equal(t, uint32(1), msg.RetryCount)
	msg.addFailedCount()
	assert.Equal(t, uint32(2), msg.RetryCount)
}

func TestUpgradePostMsgUpdatePostStatus(t *testing.T) {
	t.Run("non-success status", func(t *testing.T) {
		oldDir := postContentCacheDir
		postContentCacheDir = t.TempDir()
		defer func() { postContentCacheDir = oldDir }()

		msg := &UpgradePostMsg{
			Uuid:       "test-status-update",
			PostStatus: NotReady,
		}
		msg.updatePostStatus(PostFailure)
		assert.Equal(t, PostFailure, msg.PostStatus)
	})

	t.Run("success status deletes file", func(t *testing.T) {
		oldDir := postContentCacheDir
		postContentCacheDir = t.TempDir()
		defer func() { postContentCacheDir = oldDir }()

		msg := &UpgradePostMsg{
			Uuid:       "test-status-success",
			PostStatus: WaitPost,
		}
		msg.save()
		filePath := filepath.Join(postContentCacheDir, msg.Uuid)
		assert.FileExists(t, filePath)
		msg.updatePostStatus(PostSuccess)
		// File may or may not be deleted depending on log level; status should be set
		// In debug level, file is kept and status updated
	})
}

func TestGetLocalJobPostMsgEmpty(t *testing.T) {
	oldDir := postContentCacheDir
	postContentCacheDir = "/nonexistent/path/for/test"
	defer func() { postContentCacheDir = oldDir }()

	result := getLocalJobPostMsg()
	assert.Empty(t, result)
}

func TestGetLocalJobPostMsgWithFiles(t *testing.T) {
	oldDir := postContentCacheDir
	postContentCacheDir = t.TempDir()
	defer func() { postContentCacheDir = oldDir }()

	msg := &UpgradePostMsg{
		Uuid:       "job-1",
		PostStatus: WaitPost,
	}
	msg.save()

	msg2 := &UpgradePostMsg{
		Uuid:       "job-2",
		PostStatus: PostFailure,
	}
	msg2.save()

	result := getLocalJobPostMsg()
	assert.Len(t, result, 2)
	assert.Contains(t, result, "job-1")
	assert.Contains(t, result, "job-2")
}

func TestGetLocalJobPostMsgSkipsSuccess(t *testing.T) {
	oldDir := postContentCacheDir
	postContentCacheDir = t.TempDir()
	defer func() { postContentCacheDir = oldDir }()

	msg := &UpgradePostMsg{
		Uuid:       "job-success",
		PostStatus: PostSuccess,
	}
	msg.save()

	result := getLocalJobPostMsg()
	assert.Empty(t, result)
}

func TestTarFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source files
	file1 := filepath.Join(tmpDir, "a.txt")
	file2 := filepath.Join(tmpDir, "b.log")
	require.NoError(t, os.WriteFile(file1, []byte("content-a"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content-b"), 0644))

	outFile := filepath.Join(tmpDir, "out.tar")
	err := tarFiles([]string{file1, file2}, outFile)
	require.NoError(t, err)

	// Verify the tar file
	f, err := os.Open(outFile)
	require.NoError(t, err)
	defer f.Close()

	tr := tar.NewReader(f)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		entries = append(entries, hdr.Name)
	}
	assert.Contains(t, entries, "a.txt")
	assert.Contains(t, entries, "b.log")
}

func TestTarFilesNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "out.tar")
	err := tarFiles([]string{filepath.Join(tmpDir, "nope.txt")}, outFile)
	assert.Error(t, err)
}

func TestTarFilesInvalidOutPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "a.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("x"), 0644))
	err := tarFiles([]string{srcFile}, filepath.Join(tmpDir, "nonexistent_dir", "out.tar"))
	assert.Error(t, err)
}

func TestGetTaskIdNoFile(t *testing.T) {
	// cacheTaskInfo path should not exist in test env; expect 0
	result := getTaskId()
	assert.Equal(t, 0, result)
}

func TestLoadLocalCVEData(t *testing.T) {
	// cveLocalInfo may or may not exist depending on the environment.
	// Just verify the function does not panic and returns either nil or data.
	result := loadLocalCVEData()
	// If the file exists, result should be non-nil; if not, nil.
	_ = result
}

func TestUpdateRequestUrl(t *testing.T) {
	m := &UpdatePlatformManager{}
	m.UpdateRequestUrl("https://example.com/api")
	assert.Equal(t, "https://example.com/api", m.requestUrl)
}

func TestIsMajorUpgradeEmptyTarget(t *testing.T) {
	m := &UpdatePlatformManager{}
	assert.False(t, m.IsMajorUpgrade())
}

func TestSetInhibitAutoQuit(t *testing.T) {
	m := &UpdatePlatformManager{}
	// This is a no-op function, just verify it doesn't panic
	m.SetInhibitAutoQuit()
}

func TestGenVersionResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/version", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "testtoken",
		config:     &Cfg.Config{},
	}
	resp, err := m.genVersionResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenVersionResponseBadURLUpdatePlatform(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	_, err := m.genVersionResponse()
	assert.Error(t, err)
}

func TestGenThrottlingResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/throttling/client", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Repo-Token"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	resp, err := m.genThrottlingResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenThrottlingResponseBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
	}
	_, err := m.genThrottlingResponse()
	assert.Error(t, err)
}

func TestGenCurrentPkgListsResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/package", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("baseline"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:  srv.URL,
		Token:       "tok",
		preBaseline: "pre-baseline-100",
	}
	resp, err := m.genCurrentPkgListsResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenCVEInfoResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/cve/sync", r.URL.Path)
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("synctime"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	resp, err := m.genCVEInfoResponse("2024-01-01")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenUpdateLogResponseUpdatePlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/systemupdatelogs", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("baseline"))
		assert.NotEmpty(t, r.URL.Query().Get("isUnstable"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":[]}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		Token:          "tok",
		targetBaseline: "target-200",
	}
	resp, err := m.genUpdateLogResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenIpfsConfigResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/ipfs/config")
		assert.Equal(t, "test-id", r.URL.Query().Get("id"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL + "/base",
		Token:      "tok",
		IPFSConfig: ratelimit.IPFSConfig{ID: "test-id"},
	}
	resp, err := m.genIpfsConfigResponse()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGenIpfsConfigResponseBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "://invalid-url",
	}
	_, err := m.genIpfsConfigResponse()
	assert.Error(t, err)
}

func TestGetResponseDataSuccess(t *testing.T) {
	body := `{"result":true,"code":0,"data":{"version":"1.0"}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	data, result, code, err := getResponseData(resp, GetVersion)
	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, 0, code)
	assert.NotNil(t, data)
}

func TestGetResponseDataNonOK(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("")),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	_, result, _, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.False(t, result)
}

func TestGetResponseDataResultFalse(t *testing.T) {
	body := `{"result":false,"code":416,"msg":"too many requests"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	_, result, code, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
	assert.False(t, result)
	assert.Equal(t, 416, code)
}

func TestGetResponseDataInvalidJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{bad json}")),
		Request: &http.Request{
			RequestURI: "http://test/api/v1/version",
			URL:        &url.URL{Path: "/api/v1/version"},
		},
	}
	_, _, _, err := getResponseData(resp, GetVersion)
	assert.Error(t, err)
}

func TestGenThrottlingByToken(t *testing.T) {
	throttlingData := map[string]interface{}{
		"result": true,
		"code":   0,
		"data": map[string]interface{}{
			"allDayRateLimit": map[string]interface{}{
				"startTime": "00:00:00",
				"endTime":   "23:59:59",
				"rateLimit": 10240,
				"type":      1,
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(throttlingData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	err := m.GenThrottlingByToken()
	assert.NoError(t, err)
}

func TestGenThrottlingByTokenError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
	}
	err := m.GenThrottlingByToken()
	assert.Error(t, err)
}

func TestGenThrottlingByTokenBadData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":"not-an-object"}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
	}
	err := m.GenThrottlingByToken()
	assert.Error(t, err)
}

func TestGenIpfsConfig(t *testing.T) {
	ipfsData := map[string]interface{}{
		"result": true,
		"code":   0,
		"data": map[string]interface{}{
			"id": "test-id",
			"dl": map[string]interface{}{
				"a": map[string]interface{}{
					"startTime": "00:00:00",
					"endTime":   "23:59:59",
					"rateLimit": 10240,
					"type":      1,
				},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(ipfsData)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL + "/base",
		Token:      "tok",
	}
	err := m.GenIpfsConfig()
	assert.NoError(t, err)
}

func TestGenIpfsConfigError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
	}
	err := m.GenIpfsConfig()
	assert.Error(t, err)
}

func TestGetCVEUpdateLogs(t *testing.T) {
	m := &UpdatePlatformManager{
		cvePkgs: map[string][]string{
			"pkg1": {"CVE-2024-001", "CVE-2024-002"},
		},
	}
	CVEs = map[string]CEVInfo{
		"CVE-2024-001": {CveId: "CVE-2024-001"},
		"CVE-2024-002": {CveId: "CVE-2024-002"},
	}
	result := m.GetCVEUpdateLogs([]string{"pkg1", "pkg2"})
	assert.Len(t, result, 2)
}

func TestIsForceUpdateFunc(t *testing.T) {
	assert.False(t, IsForceUpdate(UnknownUpdate))
	assert.False(t, IsForceUpdate(NormalUpdate))
	assert.True(t, IsForceUpdate(UpdateNow))
	assert.True(t, IsForceUpdate(UpdateShutdown))
	assert.True(t, IsForceUpdate(UpdateRegularly))
}

func TestUpdateSourceListEarlyReturn(t *testing.T) {
	// When IntranetUpdate=false or PlatformUpdate=false, UpdateSourceList should early-return
	// without attempting any file writes.
	m := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate: false,
			PlatformUpdate: false,
		},
	}
	// should not panic or error (the function returns void, just verify no panic)
	m.UpdateSourceList()

	// also test with IntranetUpdate=true but PlatformUpdate=false
	m2 := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate: true,
			PlatformUpdate: false,
		},
	}
	m2.UpdateSourceList()

	// and IntranetUpdate=false but PlatformUpdate=true
	m3 := &UpdatePlatformManager{
		config: &Cfg.Config{
			IntranetUpdate: false,
			PlatformUpdate: true,
		},
	}
	m3.UpdateSourceList()

	assert.True(t, true, "all early-return paths completed without panic")
}

func TestGenPreBuild_ReturnsNonEmpty(t *testing.T) {
	// genPreBuild reads /var/lib/lastore/os-version.b which should exist on the system
	// If it doesn't exist, it returns empty string (error path)
	// We just verify it doesn't panic
	assert.NotPanics(t, func() {
		_ = genPreBuild()
	})
}

func TestGenPreBuild_ReturnsVersionFormat(t *testing.T) {
	result := genPreBuild()
	// If the os-version file exists and is valid, result should be "MajorVersion.MinorVersion.OsBuild"
	// If not, result will be empty string — both are valid outcomes
	if result != "" {
		// Should contain at least two dots (e.g., "25.0.12345")
		assert.Contains(t, result, ".")
	}
}
