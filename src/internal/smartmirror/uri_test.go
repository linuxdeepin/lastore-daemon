// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import "testing"

func TestRewritePreservesSuitePath(t *testing.T) {
	original := "https://cdn-community-packages.deepin.com/deepin/beige/pool/main/a/a.deb"
	official := "https://cdn-community-packages.deepin.com/deepin/"
	mirror := "https://mirrors.kernel.org/deepin/"

	got, ok := Rewrite(original, official, mirror)
	if !ok {
		t.Fatal("Rewrite rejected valid URL")
	}
	want := "https://mirrors.kernel.org/deepin/beige/pool/main/a/a.deb"
	if got != want {
		t.Fatalf("Rewrite() = %q, want %q", got, want)
	}
}

func TestRewriteRejectsMismatchedOfficialBase(t *testing.T) {
	if _, ok := Rewrite("https://other.example/deepin/beige/pool/a.deb",
		"https://cdn-community-packages.deepin.com/deepin/",
		"https://mirrors.kernel.org/deepin/"); ok {
		t.Fatal("Rewrite accepted URL outside official base")
	}
}
