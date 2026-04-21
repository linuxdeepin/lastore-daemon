// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
	}

	if err := manager.updateTargetPkgMetaSync(); err == nil {
		t.Fatal("updateTargetPkgMetaSync() error = nil, want non-nil")
	}
}
