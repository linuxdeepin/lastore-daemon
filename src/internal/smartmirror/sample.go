// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import "time"

// Sample is one observation of an APT method transfer. Bytes and Body are the
// body bytes and the time spent on them, Wait is the time before the body
// started, and Capped reports that a probe stopped at its byte limit.
// ExpectedBytes is the size of the object reported by the method - in the start
// message when the server announces it, otherwise in the done message - and is
// 0 when the size is unknown.
type Sample struct {
	Endpoint      string
	Bytes         int64
	ExpectedBytes int64
	Wait, Body    time.Duration
	At            time.Time
	Capped        bool
}
