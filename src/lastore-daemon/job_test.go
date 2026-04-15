// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"strconv"
	"testing"
	"time"
)

func TestJob(t *testing.T) {
	tests := []string{system.DownloadJobType, system.InstallJobType, system.RemoveJobType, system.UpdateJobType, system.DistUpgradeJobType,
		system.PrepareDistUpgradeJobType, system.UpdateSourceJobType, system.CleanJobType, system.FixErrorJobType}

	info := system.JobProgressInfo{}
	info.Cancelable = true
	info.Status = system.ReadyStatus

	for i, jobType := range tests {
		t.Run("TestJob-Type-"+jobType, func(t *testing.T) {
			if jobType == system.DownloadJobType || jobType == system.PrepareDistUpgradeJobType {
				// 这两要调dpkg暂时不测
				return
			}
			job := NewJob(nil, "TestJob-Type-"+strconv.Itoa(i), "TestJob-Type-"+jobType, nil, jobType, "", nil)
			if job.String() == "" {
				t.Error("TestJob String() error,type=", jobType)
			}
			// 初始状态无改变
			change := job.updateInfo(info)
			if change {
				t.Error("TestJob changed,type=", jobType)
			}
			// 测钩子，保证能设置成功就行
			hooks := make(map[string]func() error)
			hooks["test"] = func() error {
				job.Description = "testhook"
				return nil
			}
			job.setPreHooks(hooks)
			job.getPreHook("test")()
			if job.Description != "testhook" {
				t.Error("TestJob Hook() error,type=", jobType)
			}
		})
	}
}

func TestJobDeliveryDownloadInfoUpdatesSpeedAndNormalizedProto(t *testing.T) {
	tests := []struct {
		name      string
		proto     string
		wantProto string
	}{
		{name: "http", proto: "http", wantProto: "http"},
		{name: "https", proto: "https", wantProto: "http"},
		{name: "p2p", proto: "p2p", wantProto: "p2p"},
		{name: "delivery", proto: "delivery", wantProto: "p2p"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := NewJob(nil, "test-job", "test-job", nil, system.DownloadJobType, "", nil)

			job.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
				JobId: "test-job",
				Speed: 1024,
				Proto: tt.proto,
			})

			if job.Speed != 1024 {
				t.Fatalf("Speed = %d, want 1024", job.Speed)
			}
			if job.Proto != tt.wantProto {
				t.Fatalf("Proto = %q, want %q", job.Proto, tt.wantProto)
			}
		})
	}
}

func TestJobIgnoresEmptyDeliveryDownloadInfo(t *testing.T) {
	job := NewJob(nil, "test-job", "test-job", nil, system.PrepareDistUpgradeJobType, "", nil)
	job.DownloadSize = 1024 * 1024
	job.Status = system.RunningStatus
	job.Progress = 0.5
	job.speedMeter.SetDownloadSize(job.DownloadSize)
	past := time.Now().Add(-10 * time.Second)
	job.speedMeter.startTime = past
	job.speedMeter.updateTime = past

	job.updateDeliveryDownloadInfo(system.JobDeliveryDownloadInfo{
		JobId: "test-job",
	})

	job.updateInfo(system.JobProgressInfo{
		JobId:      "test-job",
		Progress:   0.5,
		Status:     system.RunningStatus,
		Cancelable: true,
	})

	if job.Speed == 0 {
		t.Fatal("Speed = 0, want fallback progress speed")
	}
	if job.Proto != "http" {
		t.Fatalf("Proto = %q, want http", job.Proto)
	}
}
