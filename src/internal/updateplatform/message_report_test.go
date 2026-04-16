// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import "testing"

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
