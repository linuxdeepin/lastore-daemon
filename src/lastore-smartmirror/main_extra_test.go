// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"testing"
)

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// 4 args bypasses the os.Exit(-1) usage path; the Query call then fails
	// against the (absent) Smartmirror service and prints the raw URL.
	os.Args = []string{"smartmirror", "http://example.com/a.deb", "official.example.com", "mirror.example.com"}
	main()
}
