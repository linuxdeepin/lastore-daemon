// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	"github.com/stretchr/testify/assert"
)

func TestJobInitDownloadSize(t *testing.T) {
	j := &Job{service: newTestService()}
	j.initDownloadSize(1024.0)
	assert.Equal(t, int64(1024), j.DownloadSize)

	// second call should not overwrite existing non-zero size
	j.initDownloadSize(2048.0)
	assert.Equal(t, int64(1024), j.DownloadSize)
}

func TestJobInitDownloadSize_ZeroInitial(t *testing.T) {
	j := &Job{service: newTestService()}
	j.initDownloadSize(0)
	assert.Equal(t, int64(0), j.DownloadSize)
}

func TestJobSetUpdatePolicy(t *testing.T) {
	j := &Job{service: newTestService()}
	j.setUpdatePolicy(updateplatform.UpdateTp(2))
	assert.Equal(t, 2, j.PolicyTyp)

	// set same value — no change
	j.setUpdatePolicy(updateplatform.UpdateTp(2))
	assert.Equal(t, 2, j.PolicyTyp)

	j.setUpdatePolicy(updateplatform.UpdateTp(5))
	assert.Equal(t, 5, j.PolicyTyp)
}

func TestJobSetError(t *testing.T) {
	j := &Job{service: newTestService(), Description: "old"}
	je := &system.JobError{}
	j.setError(je)
	assert.NotEqual(t, "old", j.Description)
}
