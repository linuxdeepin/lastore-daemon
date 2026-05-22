// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/ratelimit"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestGenPlatformReposFromRepoInfosConvertsToDeliveryWhenDeliveryEnabled(t *testing.T) {
	repos := genPlatformReposFromRepoInfos([]repoInfo{
		{
			Uri:      "https://professional-packages.chinauos.com/desktop-professional",
			CodeName: "eagle",
		},
	}, "main", true, false)

	if len(repos) != 1 {
		t.Fatalf("len(repos) = %d, want 1", len(repos))
	}
	want := "deb delivery://professional-packages.chinauos.com/desktop-professional eagle main"
	if repos[0] != want {
		t.Fatalf("repos[0] = %q, want %q", repos[0], want)
	}
}

func TestGenPlatformReposFromRepoInfosKeepsServerRepoPrefixWhenDeliveryDisabled(t *testing.T) {
	repos := genPlatformReposFromRepoInfos([]repoInfo{
		{
			Uri:      "https://professional-packages.chinauos.com/desktop-professional",
			CodeName: "eagle",
		},
	}, "main", false, false)

	if len(repos) != 1 {
		t.Fatalf("len(repos) = %d, want 1", len(repos))
	}
	want := "deb https://professional-packages.chinauos.com/desktop-professional eagle main"
	if repos[0] != want {
		t.Fatalf("repos[0] = %q, want %q", repos[0], want)
	}
}

func TestGenPlatformReposFromRepoInfosKeepsServerRepoPrefixForIntranet(t *testing.T) {
	repos := genPlatformReposFromRepoInfos([]repoInfo{
		{
			Uri:      "https://professional-packages.chinauos.com/desktop-professional",
			CodeName: "eagle",
		},
	}, "main", true, true)

	if len(repos) != 1 {
		t.Fatalf("len(repos) = %d, want 1", len(repos))
	}
	want := "deb https://professional-packages.chinauos.com/desktop-professional eagle main"
	if repos[0] != want {
		t.Fatalf("repos[0] = %q, want %q", repos[0], want)
	}
}

func TestGenPlatformReposFromRepoInfosKeepsDeliverySource(t *testing.T) {
	repos := genPlatformReposFromRepoInfos([]repoInfo{
		{
			Source: "deb delivery://professional-packages.chinauos.com/desktop-professional eagle main",
		},
	}, "", false, false)

	if len(repos) != 1 {
		t.Fatalf("len(repos) = %d, want 1", len(repos))
	}
	want := "deb delivery://professional-packages.chinauos.com/desktop-professional eagle main"
	if repos[0] != want {
		t.Fatalf("repos[0] = %q, want %q", repos[0], want)
	}
}

func TestHasDeliveryRepo(t *testing.T) {
	manager := &UpdatePlatformManager{
		repoInfos: []repoInfo{
			{Source: "deb https://packages.example.com/desktop beige main"},
			{Source: "deb delivery://packages.example.com/apps beige main"},
		},
	}

	if !manager.HasDeliveryRepo() {
		t.Fatal("expected delivery repo to be detected")
	}
}

func TestUpdateTargetPkgMetaSyncClearsTargetPkgMetaWhenDataIsNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/package" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/package")
		}
		if got := r.URL.Query().Get("baseline"); got != "test-baseline" {
			t.Fatalf("baseline = %q, want %q", got, "test-baseline")
		}
		fmt.Fprint(w, `{"result":true,"code":0,"data":null}`)
	}))
	defer server.Close()

	manager := &UpdatePlatformManager{
		requestUrl:     server.URL,
		targetBaseline: "test-baseline",
		Token:          "abcd",
		PreUpgradeCheck: []ShellCheck{
			{Name: "pre-upgrade.sh", Shell: "ZWNobyBwcmU="},
		},
		PreUpdateCheck: []ShellCheck{
			{Name: "pre-update.sh", Shell: "ZWNobyBwcmU="},
		},
		TargetCorePkgs: map[string]system.PackageInfo{
			"old-core": {Name: "old-core", Version: "1.0"},
		},
		SelectPkgs: map[string]system.PackageInfo{
			"old-select": {Name: "old-select", Version: "1.0"},
		},
		FreezePkgs: map[string]system.PackageInfo{
			"old-freeze": {Name: "old-freeze", Version: "1.0"},
		},
		PurgePkgs: map[string]system.PackageInfo{
			"old-purge": {Name: "old-purge", Version: "1.0"},
		},
	}

	if err := manager.updateTargetPkgMetaSync(); err != nil {
		t.Fatalf("updateTargetPkgMetaSync() error = %v, want nil", err)
	}
	if len(manager.PreUpgradeCheck) != 0 {
		t.Fatalf("len(PreUpgradeCheck) = %d, want 0", len(manager.PreUpgradeCheck))
	}
	if len(manager.PreUpdateCheck) != 0 {
		t.Fatalf("len(PreUpdateCheck) = %d, want 0", len(manager.PreUpdateCheck))
	}
	if len(manager.TargetCorePkgs) != 0 {
		t.Fatalf("len(TargetCorePkgs) = %d, want 0", len(manager.TargetCorePkgs))
	}
	if len(manager.SelectPkgs) != 0 {
		t.Fatalf("len(SelectPkgs) = %d, want 0", len(manager.SelectPkgs))
	}
	if len(manager.FreezePkgs) != 0 {
		t.Fatalf("len(FreezePkgs) = %d, want 0", len(manager.FreezePkgs))
	}
	if len(manager.PurgePkgs) != 0 {
		t.Fatalf("len(PurgePkgs) = %d, want 0", len(manager.PurgePkgs))
	}
}

func TestUpdateTargetPkgMetaSyncReturnsErrorForInvalidTargetPkgListData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":true,"code":0,"data":"invalid"}`)
	}))
	defer server.Close()

	manager := &UpdatePlatformManager{
		requestUrl:     server.URL,
		targetBaseline: "test-baseline",
		Token:          "abcd",
	}

	if err := manager.updateTargetPkgMetaSync(); err == nil {
		t.Fatal("updateTargetPkgMetaSync() error = nil, want non-nil")
	}
}

func TestUpdateDeliverySpeedLimitWithConfigUsesLocalIPFSConfig(t *testing.T) {
	uploadLimit := &ratelimit.SyncLimit{
		AllDayRateLimit: &ratelimit.RateLimitWithTime{
			RateLimit: 128,
			Type:      1,
		},
	}
	downloadLimit := &ratelimit.SyncLimit{
		AllDayRateLimit: &ratelimit.RateLimitWithTime{
			RateLimit: 256,
			Type:      1,
		},
	}
	manager := &UpdatePlatformManager{
		config: &Cfg.Config{},
		IPFSConfig: ratelimit.IPFSConfig{
			ID: "previous",
		},
	}

	var gotUpload ratelimit.IPFSLimitRate
	var gotDownload ratelimit.IPFSLimitRate
	oldSetIPFSRateLimit := setIPFSRateLimit
	setIPFSRateLimit = func(uploadLimitRate, downloadLimitRate ratelimit.IPFSLimitRate) error {
		gotUpload = uploadLimitRate
		gotDownload = downloadLimitRate
		return nil
	}
	defer func() {
		setIPFSRateLimit = oldSetIPFSRateLimit
	}()

	err := manager.UpdateDeliverySpeedLimitWithConfig(ratelimit.IPFSConfig{
		ID:            "local",
		UploadLimit:   uploadLimit,
		DownloadLimit: downloadLimit,
	})
	if err != nil {
		t.Fatalf("UpdateDeliverySpeedLimitWithConfig() error = %v, want nil", err)
	}
	if manager.IPFSConfig.ID != "local" {
		t.Fatalf("IPFSConfig.ID = %q, want %q", manager.IPFSConfig.ID, "local")
	}
	if gotUpload.GlobalLimitRemote == nil || gotUpload.GlobalLimitRemote.CurrentRate != 128*1024 {
		t.Fatalf("upload global remote = %#v, want current rate %d", gotUpload.GlobalLimitRemote, 128*1024)
	}
	if gotDownload.GlobalLimitRemote == nil || gotDownload.GlobalLimitRemote.CurrentRate != 256*1024 {
		t.Fatalf("download global remote = %#v, want current rate %d", gotDownload.GlobalLimitRemote, 256*1024)
	}
}

func TestResetIntranetUpdateSettingsAfterUnregisterEnablesDeliveryAndDisablesSpeedLimits(t *testing.T) {
	limitedRateInfo := `{"LimitType":1,"StartTime":"0001-01-01T00:00:00Z","EndTime":"0001-01-01T00:00:00Z","LimitRate":40960,"CurrentRate":40960}`
	manager := &UpdatePlatformManager{
		config: &Cfg.Config{
			UpgradeDeliveryEnabled:             false,
			DownloadSpeedLimitConfig:           `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"4096","IsOnlineSpeedLimit":true}`,
			LocalDownloadSpeedLimitConfig:      `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"2048","IsOnlineSpeedLimit":false}`,
			DeliveryRemoteDownloadGlobalLimit:  limitedRateInfo,
			DeliveryRemoteUploadGlobalLimit:    limitedRateInfo,
			DeliveryRemoteDownloadPeakLimit:    limitedRateInfo,
			DeliveryRemoteUploadPeakLimit:      limitedRateInfo,
			DeliveryRemoteDownloadOffPeakLimit: limitedRateInfo,
			DeliveryRemoteUploadOffPeakLimit:   limitedRateInfo,
			DeliveryLocalDownloadGlobalLimit:   limitedRateInfo,
			DeliveryLocalUploadGlobalLimit:     limitedRateInfo,
			DeliveryLocalDownloadPeakLimit:     limitedRateInfo,
			DeliveryLocalUploadPeakLimit:       limitedRateInfo,
			DeliveryLocalDownloadOffPeakLimit:  limitedRateInfo,
			DeliveryLocalUploadOffPeakLimit:    limitedRateInfo,
		},
	}

	var gotUpload ratelimit.IPFSLimitRate
	var gotDownload ratelimit.IPFSLimitRate
	oldSetIPFSRateLimit := setIPFSRateLimit
	setIPFSRateLimit = func(uploadLimitRate, downloadLimitRate ratelimit.IPFSLimitRate) error {
		gotUpload = uploadLimitRate
		gotDownload = downloadLimitRate
		return nil
	}
	defer func() {
		setIPFSRateLimit = oldSetIPFSRateLimit
	}()

	manager.resetIntranetUpdateSettingsAfterUnregister()

	if !manager.config.UpgradeDeliveryEnabled {
		t.Fatal("UpgradeDeliveryEnabled = false, want true")
	}
	if manager.config.DownloadSpeedLimitConfig != defaultDownloadSpeedLimitConfig {
		t.Fatalf("DownloadSpeedLimitConfig = %q, want %q", manager.config.DownloadSpeedLimitConfig, defaultDownloadSpeedLimitConfig)
	}
	if manager.config.LocalDownloadSpeedLimitConfig != defaultDownloadSpeedLimitConfig {
		t.Fatalf("LocalDownloadSpeedLimitConfig = %q, want %q", manager.config.LocalDownloadSpeedLimitConfig, defaultDownloadSpeedLimitConfig)
	}
	for name, config := range map[string]string{
		"DeliveryRemoteDownloadGlobalLimit":  manager.config.DeliveryRemoteDownloadGlobalLimit,
		"DeliveryRemoteUploadGlobalLimit":    manager.config.DeliveryRemoteUploadGlobalLimit,
		"DeliveryRemoteDownloadPeakLimit":    manager.config.DeliveryRemoteDownloadPeakLimit,
		"DeliveryRemoteUploadPeakLimit":      manager.config.DeliveryRemoteUploadPeakLimit,
		"DeliveryRemoteDownloadOffPeakLimit": manager.config.DeliveryRemoteDownloadOffPeakLimit,
		"DeliveryRemoteUploadOffPeakLimit":   manager.config.DeliveryRemoteUploadOffPeakLimit,
		"DeliveryLocalDownloadGlobalLimit":   manager.config.DeliveryLocalDownloadGlobalLimit,
		"DeliveryLocalUploadGlobalLimit":     manager.config.DeliveryLocalUploadGlobalLimit,
		"DeliveryLocalDownloadPeakLimit":     manager.config.DeliveryLocalDownloadPeakLimit,
		"DeliveryLocalUploadPeakLimit":       manager.config.DeliveryLocalUploadPeakLimit,
		"DeliveryLocalDownloadOffPeakLimit":  manager.config.DeliveryLocalDownloadOffPeakLimit,
		"DeliveryLocalUploadOffPeakLimit":    manager.config.DeliveryLocalUploadOffPeakLimit,
	} {
		var rateInfo ratelimit.RateInfo
		if err := json.Unmarshal([]byte(config), &rateInfo); err != nil {
			t.Fatalf("%s unmarshal error = %v", name, err)
		}
		if rateInfo.LimitType != ratelimit.RateLimitTypeNo {
			t.Fatalf("%s LimitType = %d, want %d", name, rateInfo.LimitType, ratelimit.RateLimitTypeNo)
		}
		if rateInfo.LimitRate != ratelimit.DefaultRateLimit {
			t.Fatalf("%s LimitRate = %d, want %d", name, rateInfo.LimitRate, ratelimit.DefaultRateLimit)
		}
		if rateInfo.CurrentRate != ratelimit.DefaultRateLimit {
			t.Fatalf("%s CurrentRate = %d, want %d", name, rateInfo.CurrentRate, ratelimit.DefaultRateLimit)
		}
	}
	assertNoIPFSLimitRate(t, "upload", gotUpload)
	assertNoIPFSLimitRate(t, "download", gotDownload)
}

func assertNoIPFSLimitRate(t *testing.T, name string, limit ratelimit.IPFSLimitRate) {
	t.Helper()

	for period, rateInfo := range map[string]*ratelimit.RateInfo{
		"global remote": limit.GlobalLimitRemote,
		"global local":  limit.GlobalLimitLocal,
		"busy remote":   limit.BusyLimitRemote,
		"busy local":    limit.BusyLimitLocal,
		"free remote":   limit.FreeLimitRemote,
		"free local":    limit.FreeLimitLocal,
	} {
		if rateInfo != nil && rateInfo.LimitType != ratelimit.RateLimitTypeNo {
			t.Fatalf("%s %s limit type = %d, want %d", name, period, rateInfo.LimitType, ratelimit.RateLimitTypeNo)
		}
	}
}

func TestParseBinaryPackagesOldFormat(t *testing.T) {
	pkgs := parseBinaryPackages("['postgresql-11', 'libpq5']")
	if len(pkgs) != 2 || pkgs[0] != "postgresql-11" || pkgs[1] != "libpq5" {
		t.Fatalf("expected [postgresql-11 libpq5], got %v", pkgs)
	}
}

func TestParseBinaryPackagesJSONArray(t *testing.T) {
	pkgs := parseBinaryPackages(`[{"name":"linux-libc-dev","version":"5.10.168-1"},{"name":"libtiff5","version":"4.1.0"}]`)
	if len(pkgs) != 2 || pkgs[0] != "linux-libc-dev" || pkgs[1] != "libtiff5" {
		t.Fatalf("expected [linux-libc-dev libtiff5], got %v", pkgs)
	}
}

func TestParseBinaryPackagesStringEncodedJSON(t *testing.T) {
	pkgs := parseBinaryPackages(`"[{\"name\":\"libtiff5\",\"version\":\"4.1.0\"}]"`)
	if len(pkgs) != 1 || pkgs[0] != "libtiff5" {
		t.Fatalf("expected [libtiff5], got %v", pkgs)
	}
}

func TestParseBinaryPackagesNone(t *testing.T) {
	pkgs := parseBinaryPackages("None")
	if len(pkgs) != 0 {
		t.Fatalf("expected empty, got %v", pkgs)
	}
}

func TestParseBinaryPackagesEmpty(t *testing.T) {
	pkgs := parseBinaryPackages("")
	if len(pkgs) != 0 {
		t.Fatalf("expected empty, got %v", pkgs)
	}
}

func TestGetCVEUpdateLogsIncludesNewFormatCVEs(t *testing.T) {
	CVEs = make(map[string]CEVInfo)
	manager := &UpdatePlatformManager{cvePkgs: make(map[string][]string)}
	cves := []CEVInfo{
		{SyncTime: "2026-05-02T12:04:25+08:00", CveId: "CVE-2026-31431", Binary: `[{"name":"linux-libc-dev","version":"5.10"}]`},
		{SyncTime: "2025-05-02T12:04:25+08:00", CveId: "CVE-2025-32728", Binary: `[{"name":"openssh-client","version":"8.2"}]`},
		{SyncTime: "2024-05-02T12:04:25+08:00", CveId: "CVE-2021-3677", Binary: "['libpq5']"},
	}
	for _, cve := range cves {
		CVEs[cve.CveId] = cve
		for _, binary := range parseBinaryPackages(cve.Binary) {
			manager.cvePkgs[binary] = append(manager.cvePkgs[binary], cve.CveId)
		}
	}

	logs := manager.GetCVEUpdateLogs([]string{"linux-libc-dev", "openssh-client", "libpq5"})

	if len(logs) != 3 {
		t.Fatalf("expected 3 CVE logs, got %d", len(logs))
	}
	if _, ok := logs["CVE-2026-31431"]; !ok {
		t.Fatal("expected CVE-2026-31431 in results")
	}
	if _, ok := logs["CVE-2025-32728"]; !ok {
		t.Fatal("expected CVE-2025-32728 in results")
	}
	if _, ok := logs["CVE-2021-3677"]; !ok {
		t.Fatal("expected CVE-2021-3677 in results")
	}
}

func TestGetCVEUpdateLogsStringEncodedJSON(t *testing.T) {
	CVEs = make(map[string]CEVInfo)
	manager := &UpdatePlatformManager{cvePkgs: make(map[string][]string)}
	cves := []CEVInfo{
		{SyncTime: "2026-05-02T12:04:25+08:00", CveId: "CVE-2024-7006", Binary: `"[{\"name\":\"libtiff5\",\"version\":\"4.1.0\"}]"`},
		{SyncTime: "2024-05-02T12:04:25+08:00", CveId: "CVE-2020-19143", Binary: "['libtiff5']"},
	}
	for _, cve := range cves {
		CVEs[cve.CveId] = cve
		for _, binary := range parseBinaryPackages(cve.Binary) {
			manager.cvePkgs[binary] = append(manager.cvePkgs[binary], cve.CveId)
		}
	}

	logs := manager.GetCVEUpdateLogs([]string{"libtiff5"})

	if len(logs) != 2 {
		t.Fatalf("expected 2 CVE logs, got %d", len(logs))
	}
	if _, ok := logs["CVE-2024-7006"]; !ok {
		t.Fatal("expected CVE-2024-7006 in results")
	}
}
