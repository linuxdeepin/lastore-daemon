// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

// exitPanic is a sentinel used by the stubbed osExit to halt execution once a
// command path calls osExit, instead of terminating the test process.
type exitPanic int

// stubExit replaces the osExit seam with a recorder that stores the exit code
// and panics. The returned restore func MUST be deferred.
func stubExit() (restore func(), exitCode *int) {
	old := osExit
	code := 0
	osExit = func(c int) {
		code = c
		panic(exitPanic(c))
	}
	return func() { osExit = old }, &code
}

// setPlatform points updatePlatform at url/token for run* command tests.
func setPlatform(url, token string) {
	updatePlatform = UpdatePlatformManager{requestURL: url, Token: token}
}
