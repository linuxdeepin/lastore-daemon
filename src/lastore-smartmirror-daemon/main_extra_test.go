// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"
)

func TestMainFunction(t *testing.T) {
	// main() blocks in service.Wait() after taking the bus name; run it in a
	// goroutine and give it enough time to reach Wait() so the earlier lines
	// are exercised. If the name is already owned, main() returns immediately.
	done := make(chan struct{})
	go func() {
		defer close(done)
		main()
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Blocked in service.Wait(); expected, and harmless for the test.
	}
}
