// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
)

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
