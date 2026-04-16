// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
)

func TestShouldEnableUpgradeDeliveryService(t *testing.T) {
	tests := []struct {
		name                string
		cfg                 *config.Config
		platformHasDelivery bool
		want                bool
	}{
		{
			name: "public network follows user switch",
			cfg: &config.Config{
				UpgradeDeliveryEnabled: false,
				IntranetUpdate:         false,
				PlatformUpdate:         false,
			},
			platformHasDelivery: true,
			want:                false,
		},
		{
			name: "intranet platform delivery repo keeps service available",
			cfg: &config.Config{
				UpgradeDeliveryEnabled: false,
				IntranetUpdate:         true,
				PlatformUpdate:         true,
			},
			platformHasDelivery: true,
			want:                true,
		},
		{
			name: "intranet non delivery repo keeps service closed",
			cfg: &config.Config{
				UpgradeDeliveryEnabled: false,
				IntranetUpdate:         true,
				PlatformUpdate:         true,
			},
			platformHasDelivery: false,
			want:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldEnableUpgradeDeliveryService(tt.cfg, tt.platformHasDelivery); got != tt.want {
				t.Fatalf("shouldEnableUpgradeDeliveryService() = %v, want %v", got, tt.want)
			}
		})
	}
}
