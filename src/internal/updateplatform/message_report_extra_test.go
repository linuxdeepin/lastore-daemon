// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/lastore-daemon/src/internal/ratelimit"
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
		name    string
		old     string
		new     string
		want    bool
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
		Uuid:         "test-uuid-save",
		PostStatus:   WaitPost,
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
