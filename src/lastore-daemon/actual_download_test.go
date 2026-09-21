// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackDownloadSizeUsesRegularFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "pkg.deb")
	require.NoError(t, os.WriteFile(filename, []byte("123456"), 0600))

	bytes, known, reason := fallbackDownloadSize(filename, 6)
	assert.True(t, known)
	assert.Equal(t, int64(6), bytes)
	assert.Empty(t, reason)
}

func TestFallbackDownloadSizeUsesAptPartialFinalPath(t *testing.T) {
	archives := t.TempDir()
	partial := filepath.Join(archives, "partial", "pkg.deb")
	require.NoError(t, os.Mkdir(filepath.Dir(partial), 0700))
	final := filepath.Join(archives, "pkg.deb")
	require.NoError(t, os.WriteFile(final, []byte("123456"), 0600))

	bytes, known, reason := fallbackDownloadSize(partial, 6)
	assert.True(t, known)
	assert.Equal(t, int64(6), bytes)
	assert.Empty(t, reason)
}

func TestFallbackDownloadSizeRejectsMismatchAndSymlink(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "pkg.deb")
	require.NoError(t, os.WriteFile(filename, []byte("123456"), 0600))
	_, known, reason := fallbackDownloadSize(filename, 7)
	assert.False(t, known)
	assert.Contains(t, reason, "does not match")

	link := filepath.Join(dir, "link.deb")
	require.NoError(t, os.Symlink(filename, link))
	_, known, reason = fallbackDownloadSize(link, 6)
	assert.False(t, known)
	assert.Contains(t, reason, "regular file")
}

func TestJobCompletesActualDownloadSession(t *testing.T) {
	j := NewJob(nil, "job-1", "download", nil, system.DownloadJobType, DownloadQueue, nil)
	uri := "https://mirror.example/pool/pkg.deb"
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		URI: uri, UsedMirror: "https://mirror.example/", FileName: "/var/cache/apt/archives/pkg.deb",
		DownloadSize: 100, DownloadedSize: 40, Speed: 1000, Proto: "https",
	})
	time.Sleep(time.Millisecond)
	completed := j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		URI: uri, DownloadSize: 100, DownloadedSize: 100, IsFinished: true,
	})
	require.NotNil(t, completed)
	assert.Equal(t, uri, completed.URI)
	assert.Equal(t, int64(100), completed.Bytes)
	assert.True(t, completed.BytesKnown)
	assert.False(t, completed.Failed)
}

func TestJobUsesFirstProgressAsBodyStart(t *testing.T) {
	j := NewJob(nil, "job-1", "download", nil, system.DownloadJobType, DownloadQueue, nil)
	uri := "https://mirror.example/pool/pkg.deb"
	start := time.Now().Add(-2 * time.Second)
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		MethodEvent: 200, EventAt: start, URI: uri, UsedMirror: "https://mirror.example/",
		FileName: "/var/cache/apt/archives/pkg.deb", DownloadSize: 2 << 20,
	})
	progress := start.Add(time.Second)
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		MethodEvent: 102, EventAt: progress, URI: uri, DownloadedSize: 1 << 20, Speed: 1,
	})
	completed := j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		MethodEvent: 201, EventAt: start.Add(3 * time.Second), URI: uri,
		DownloadSize: 2 << 20, DownloadedSize: 2 << 20, IsFinished: true,
	})
	require.NotNil(t, completed)
	assert.WithinDuration(t, progress, completed.FirstByteAt, time.Millisecond)
}

func TestJobCarriesTransferTimestamps(t *testing.T) {
	j := NewJob(nil, "job-1", "download", nil, system.DownloadJobType, DownloadQueue, nil)
	uri := "https://mirror.example/pool/pkg.deb"
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		MethodEvent: 200, URI: uri, UsedMirror: "https://mirror.example/", TransferStartUs: 1000,
		DownloadSize: 2 << 20,
	})
	completed := j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		MethodEvent: 201, URI: uri, TransferEndUs: 2000,
		DownloadSize: 2 << 20, DownloadedSize: 2 << 20, IsFinished: true,
	})
	require.NotNil(t, completed)
	assert.Equal(t, int64(1000), completed.TransferStartUs)
	assert.Equal(t, int64(2000), completed.TransferEndUs)
}

func TestCompleteDownloadReportPrefersMethodElapsed(t *testing.T) {
	now := time.Now()
	report := (&JobManager{}).completeDownloadReport(&completedDownload{
		URI:             "https://mirror.example/pool/pkg.deb",
		StartedAt:       now.Add(-10 * time.Second),
		FirstByteAt:     now.Add(-9 * time.Second),
		Size:            100,
		Bytes:           100,
		BytesKnown:      true,
		TransferStartUs: 1000,
		TransferEndUs:   2000,
	})
	assert.Equal(t, int64(1), report.ElapsedMs)
	assert.Equal(t, int64(1_000), report.WaitMs)
}

func TestCompleteDownloadReportWithoutMethodTimeHasNoElapsed(t *testing.T) {
	now := time.Now()
	report := (&JobManager{}).completeDownloadReport(&completedDownload{
		URI:         "https://mirror.example/pool/pkg.deb",
		StartedAt:   now.Add(-10 * time.Second),
		FirstByteAt: now.Add(-9 * time.Second),
		Size:        100,
		Bytes:       100,
		BytesKnown:  true,
	})
	assert.Equal(t, int64(0), report.ElapsedMs)
}

func TestCompleteDownloadReportRejectsInvalidMethodTime(t *testing.T) {
	cases := []struct {
		name  string
		start int64
		end   int64
	}{
		{name: "zero start", start: 0, end: 2000},
		{name: "zero end", start: 1000, end: 0},
		{name: "equal", start: 1000, end: 1000},
		{name: "reversed", start: 2000, end: 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			report := (&JobManager{}).completeDownloadReport(&completedDownload{
				URI:             "https://mirror.example/pool/pkg.deb",
				StartedAt:       now.Add(-10 * time.Second),
				FirstByteAt:     now.Add(-9 * time.Second),
				Size:            100,
				Bytes:           100,
				BytesKnown:      true,
				TransferStartUs: tc.start,
				TransferEndUs:   tc.end,
			})
			assert.Equal(t, int64(0), report.ElapsedMs)
		})
	}
}

func TestJobCompletesCompactFinishAndFallsBackToFileSize(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "pkg.deb")
	require.NoError(t, os.WriteFile(filename, []byte("123456"), 0600))

	j := NewJob(nil, "job-1", "download", nil, system.DownloadJobType, DownloadQueue, nil)
	uri := "https://mirror.example/pool/pkg.deb"
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		URI: uri, UsedMirror: "https://mirror.example/", FileName: filename,
		DownloadSize: 6, Speed: 1000, Proto: "https",
	})
	completed := j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		IsFinished: true, Speed: -1, Proto: "https",
	})
	require.NotNil(t, completed)
	assert.Equal(t, uri, completed.URI)
	assert.False(t, completed.BytesKnown)

	report := (&JobManager{}).completeDownloadReport(completed)
	assert.True(t, report.BytesKnown)
	assert.Equal(t, int64(6), report.Bytes)
}

func TestJobFailureFlushesActiveDownloads(t *testing.T) {
	j := NewJob(nil, "job-1", "download", nil, system.DownloadJobType, DownloadQueue, nil)
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		URI: "https://mirror.example/pool/pkg.deb", DownloadedSize: 10, Speed: 1,
	})
	failed := j.finishActiveDownloads(true)
	require.Len(t, failed, 1)
	assert.True(t, failed[0].Failed)
	assert.Equal(t, int64(10), failed[0].Bytes)
}

func TestJobSuccessFlushesActiveDownloads(t *testing.T) {
	j := NewJob(nil, "job-1", "download", nil, system.DownloadJobType, DownloadQueue, nil)
	j.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		URI: "https://mirror.example/pool/pkg.deb", DownloadedSize: 10, Speed: 1,
	})
	completed := j.finishActiveDownloads(false)
	require.Len(t, completed, 1)
	assert.False(t, completed[0].Failed)
	assert.Equal(t, int64(10), completed[0].Bytes)
}

func TestCompleteDownloadReportKeepsUnknownSize(t *testing.T) {
	now := time.Now()
	report := (&JobManager{}).completeDownloadReport(&completedDownload{
		URI:       "https://mirror.example/pool/pkg.deb",
		StartedAt: now.Add(-time.Second),
		Size:      100,
	})
	assert.False(t, report.BytesKnown)
	assert.Equal(t, int64(0), report.Bytes)
	assert.False(t, report.Failed)
}

func TestCompleteDownloadReportSubtractsResumePointFromFallback(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "pkg.deb")
	require.NoError(t, os.WriteFile(filename, []byte("123456"), 0600))
	now := time.Now()
	report := (&JobManager{}).completeDownloadReport(&completedDownload{
		URI:         "https://mirror.example/pool/pkg.deb",
		FileName:    filename,
		StartedAt:   now.Add(-time.Second),
		Size:        6,
		ResumePoint: 2,
	})
	assert.True(t, report.BytesKnown)
	assert.Equal(t, int64(4), report.Bytes)
}
