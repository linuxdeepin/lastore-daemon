// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSmartMirrorUsesConfiguredArchiveRoots(t *testing.T) {
	oldStateDirectory := stateDirectory
	stateDirectory = t.TempDir()
	t.Cleanup(func() { stateDirectory = oldStateDirectory })

	if err := os.WriteFile(filepath.Join(stateDirectory, "mirrors.json"), []byte(`[
  {"id":"Kernel","url":"https://mirrors.kernel.org/deepin/","weight":90000}
]`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDirectory, "smartmirror_config.json"), []byte(`{"Enable":true}`), 0644); err != nil {
		t.Fatal(err)
	}

	s := newSmartMirror(nil)
	if len(s.sourcesURL) != 1 {
		t.Fatalf("candidate source count = %d, want 1: %v", len(s.sourcesURL), s.sourcesURL)
	}
	if got := s.sourcesURL[0]; got != "https://mirrors.kernel.org/deepin/" {
		t.Fatalf("candidate source = %q, want upstream base root without suite path", got)
	}
}
