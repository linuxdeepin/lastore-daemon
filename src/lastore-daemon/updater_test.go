// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateSourceSupportsP2PFromDeliveryPrefix(t *testing.T) {
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

			got := updateSourceSupportsP2P(sourcePath)
			if got != tt.want {
				t.Fatalf("updateSourceSupportsP2P() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateSourceSupportsP2PReturnsFalseForMissingSource(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "missing.list")

	if updateSourceSupportsP2P(sourcePath) {
		t.Fatal("updateSourceSupportsP2P() = true, want false")
	}
}
