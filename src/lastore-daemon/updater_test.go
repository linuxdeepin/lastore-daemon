// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
)

func TestUpdaterExportedMethodsIncludeSetMirrorSource(t *testing.T) {
	methods := (&Updater{}).GetExportedMethods()
	for _, method := range methods {
		if method.Name != "SetMirrorSource" {
			continue
		}
		if len(method.InArgs) != 1 || method.InArgs[0] != "id" {
			t.Fatalf("SetMirrorSource InArgs = %v, want [id]", method.InArgs)
		}
		return
	}
	t.Fatal("SetMirrorSource is not exported")
}

func TestUpdaterExportedMethodsIncludeListMirrorSources(t *testing.T) {
	methods := (&Updater{}).GetExportedMethods()
	for _, method := range methods {
		if method.Name != "ListMirrorSources" {
			continue
		}
		if len(method.InArgs) != 1 || method.InArgs[0] != "lang" {
			t.Fatalf("ListMirrorSources InArgs = %v, want [lang]", method.InArgs)
		}
		if len(method.OutArgs) != 1 || method.OutArgs[0] != "mirrorSources" {
			t.Fatalf("ListMirrorSources OutArgs = %v, want [mirrorSources]", method.OutArgs)
		}
		return
	}
	t.Fatal("ListMirrorSources is not exported")
}

func TestSourceFileHasDeliveryProtocol(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "delivery repo",
			content: "deb delivery://professional-packages.chinauos.com/desktop-professional eagle main\n",
			want:    true,
		},
		{
			name:    "delivery repo with apt options",
			content: "deb [trusted=yes] delivery://professional-packages.chinauos.com/desktop-professional eagle main\n",
			want:    true,
		},
		{
			name:    "http repo",
			content: "deb https://professional-packages.chinauos.com/desktop-professional eagle main\n",
			want:    false,
		},
		{
			name:    "commented delivery repo",
			content: "# deb delivery://professional-packages.chinauos.com/desktop-professional eagle main\ndeb https://professional-packages.chinauos.com/desktop-professional eagle main\n",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath := filepath.Join(t.TempDir(), "platform.list")
			if err := os.WriteFile(sourcePath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := sourceFileHasDeliveryProtocol(sourcePath)
			if got != tt.want {
				t.Fatalf("sourceFileHasDeliveryProtocol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSourceFileHasDeliveryProtocolReturnsFalseForMissingSource(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "missing.list")

	if sourceFileHasDeliveryProtocol(sourcePath) {
		t.Fatal("sourceFileHasDeliveryProtocol() = true, want false")
	}
}

func TestGetStartupDownloadSpeedLimitConfig(t *testing.T) {
	localConfig := `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"2048","IsOnlineSpeedLimit":false}`
	remoteConfig := `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"4096","IsOnlineSpeedLimit":true}`
	defaultConfig := `{"DownloadSpeedLimitEnabled":false,"LimitSpeed":"1024","IsOnlineSpeedLimit":false}`

	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "prefer local persisted config when startup config is not online speed limit",
			cfg: config.Config{
				DownloadSpeedLimitConfig:      defaultConfig,
				LocalDownloadSpeedLimitConfig: localConfig,
			},
			want: localConfig,
		},
		{
			name: "keep remote config when online speed limit is active",
			cfg: config.Config{
				DownloadSpeedLimitConfig:      remoteConfig,
				LocalDownloadSpeedLimitConfig: localConfig,
			},
			want: remoteConfig,
		},
		{
			name: "fallback to local config when startup config is invalid",
			cfg: config.Config{
				DownloadSpeedLimitConfig:      "{invalid json}",
				LocalDownloadSpeedLimitConfig: localConfig,
			},
			want: localConfig,
		},
		{
			name: "fallback to startup config when local config is empty",
			cfg: config.Config{
				DownloadSpeedLimitConfig: defaultConfig,
			},
			want: defaultConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStartupDownloadSpeedLimitConfig(&tt.cfg)
			if got != tt.want {
				t.Fatalf("getStartupDownloadSpeedLimitConfig() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisableLocalSpeedLimitConfigSyncsPlatformSpeedOnly(t *testing.T) {
	manager := &Manager{
		config: &config.Config{
			LocalDownloadSpeedLimitConfig: `{"DownloadSpeedLimitEnabled":true,"LimitSpeed":"888","IsOnlineSpeedLimit":false}`,
		},
	}

	_ = manager.disableLocalSpeedLimitConfig("666")

	var got downloadSpeedLimitConfig
	if err := json.Unmarshal([]byte(manager.config.LocalDownloadSpeedLimitConfig), &got); err != nil {
		t.Fatalf("LocalDownloadSpeedLimitConfig unmarshal error = %v", err)
	}
	want := downloadSpeedLimitConfig{
		DownloadSpeedLimitEnabled: false,
		LimitSpeed:                "666",
		IsOnlineSpeedLimit:        false,
	}
	if got != want {
		t.Fatalf("LocalDownloadSpeedLimitConfig = %+v, want %+v", got, want)
	}
}
