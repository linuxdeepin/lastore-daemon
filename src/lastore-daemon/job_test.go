// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"strconv"
	"testing"
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
