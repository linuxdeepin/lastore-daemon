// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/ratelimit"
)

func TestGetTaskIdMissingKey(t *testing.T) {
	overridePathVars(t)
	// Valid keyfile without a TaskID: LoadFromFile succeeds but
	// GetInt("General", "TaskID") fails, exercising that error branch.
	require.NoError(t, os.WriteFile(cacheTaskInfo, []byte("[General]\nOther=1\n"), 0644))
	assert.Equal(t, 0, getTaskId())
}

func TestGetTaskIdSuccess(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(cacheTaskInfo, []byte("[General]\nTaskID=123\n"), 0644))
	assert.Equal(t, 123, getTaskId())
}

func TestSaveCEVDataSuccess(t *testing.T) {
	overridePathVars(t)
	saveCEVData(CVEMeta{
		DateTime: "2026-01-01",
		Cves:     []CEVInfo{{CveId: "CVE-2026-0001"}},
	})

	data, err := os.ReadFile(cveLocalInfo)
	require.NoError(t, err)
	assert.Contains(t, string(data), "CVE-2026-0001")
}

func TestPostProcessEventMessageTruncateAndError(t *testing.T) {
	// Non-OK response exercises the getResponseData error branch, and a long
	// EventContent exercises the truncation branch and the ExecAt==0 branch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		targetBaseline: "target-baseline",
		Token:          "tok",
		config:         &Cfg.Config{IntranetUpdate: true},
	}
	m.PostProcessEventMessage(ProcessEvent{
		EventType:    CheckEnv,
		EventContent: strings.Repeat("x", 1000),
	})
}

func TestPostProcessEventMessageIntranetDisabled(t *testing.T) {
	// IntranetUpdate == false exercises the early-return branch.
	m := &UpdatePlatformManager{config: &Cfg.Config{IntranetUpdate: false}}
	m.PostProcessEventMessage(ProcessEvent{})
}

func TestPostProcessEventMessageConnectionError(t *testing.T) {
	// Unreachable server exercises the client.Do error branch.
	m := &UpdatePlatformManager{
		requestUrl:     "http://127.0.0.1:1",
		targetBaseline: "target-baseline",
		Token:          "tok",
		config:         &Cfg.Config{IntranetUpdate: true},
	}
	m.PostProcessEventMessage(ProcessEvent{EventType: CheckEnv, EventContent: "hello"})
}

func TestCheckInReleaseFromPlatformCdnBranch(t *testing.T) {
	overridePathVars(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "InRelease data")
	}))
	defer srv.Close()

	// Cdn != "" exercises the "use cdn uri" branch.
	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: "http://unused.invalid", Cdn: srv.URL, CodeName: "eagle"}},
		Token:     "tok",
	}
	assert.NotPanics(t, func() { m.checkInReleaseFromPlatform() })
}

func TestUpdateDeliverySpeedLimitFullPath(t *testing.T) {
	old := setIPFSRateLimit
	setIPFSRateLimit = func(uploadLimitRate, downloadLimitRate ratelimit.IPFSLimitRate) error {
		return nil
	}
	t.Cleanup(func() { setIPFSRateLimit = old })

	syncLimit := ratelimit.SyncLimit{
		AllDayRateLimit:   &ratelimit.RateLimitWithTime{StartTime: "00:00:00", EndTime: "23:59:59", RateLimit: 1024, Type: 1},
		BusyTimeRateLimit: &ratelimit.RateLimitWithTime{StartTime: "09:00:00", EndTime: "18:00:00", RateLimit: 512, Type: 1},
		FreeTimeRateLimit: &ratelimit.RateLimitWithTime{StartTime: "18:00:00", EndTime: "23:59:59", RateLimit: 2048, Type: 1},
	}

	m := &UpdatePlatformManager{
		config: &Cfg.Config{},
		IPFSConfig: ratelimit.IPFSConfig{
			UploadLimit:   &syncLimit,
			DownloadLimit: &syncLimit,
		},
	}
	assert.NoError(t, m.UpdateDeliverySpeedLimit())
}

func TestUpdateDeliverySpeedLimitEmptyRemote(t *testing.T) {
	// Empty SyncLimit yields all-nil remote limits, exercising the
	// updateNoLimitConfigIfChanged (else) branches.
	old := setIPFSRateLimit
	setIPFSRateLimit = func(uploadLimitRate, downloadLimitRate ratelimit.IPFSLimitRate) error {
		return nil
	}
	t.Cleanup(func() { setIPFSRateLimit = old })

	empty := ratelimit.SyncLimit{}
	m := &UpdatePlatformManager{
		config: &Cfg.Config{},
		IPFSConfig: ratelimit.IPFSConfig{
			UploadLimit:   &empty,
			DownloadLimit: &empty,
		},
	}
	assert.NoError(t, m.UpdateDeliverySpeedLimit())
}
