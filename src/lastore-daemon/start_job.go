// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// StartSystemJob start job
// 1. Dispatch Job by type
// 2. Check whether the work queue is empty
func StartSystemJob(sys system.System, j *Job) error {
	if j == nil {
		panic("StartSystemJob with nil")
	}
	j.PropsMu.Lock()
	j.setPropDescription("")
	err := TransitionJobState(j, system.RunningStatus)
	j.PropsMu.Unlock()
	if err != nil {
		return err
	}
	switch j.Type {
	case system.DownloadJobType:
		return sys.DownloadPackages(j.Id, j.Packages, j.environ, j.option)

	case system.PrepareDistUpgradeJobType:
		return sys.DownloadSource(j.Id, j.Packages, j.environ, j.option)

	case system.InstallJobType:
		return sys.Install(j.Id, j.Packages, j.environ, j.option)

	case system.DistUpgradeJobType:
		return sys.DistUpgrade(j.Id, j.Packages, j.environ, j.option)

	case system.RemoveJobType:
		return sys.Remove(j.Id, j.Packages, j.environ)

	case system.UpdateSourceJobType:
		return sys.UpdateSource(j.Id, j.environ, j.option)

	case system.UpdateJobType:
		return sys.Install(j.Id, j.Packages, j.environ, j.option)

	case system.CleanJobType:
		return sys.Clean(j.Id)

	case system.FixErrorJobType:
		var errType string
		if len(j.Packages) != 0 {
			errType = j.Packages[0]
		}
		return sys.FixError(j.Id, errType, j.environ, j.option)

	case system.CheckSystemJobType:
		var pkg string
		if len(j.Packages) != 0 {
			pkg = j.Packages[0]
		}
		return sys.CheckSystem(j.Id, pkg, j.environ, j.option)
	case system.BackupJobType:
		return sys.OsBackup(j.Id)
	default:
		return system.NotFoundError("StartSystemJob unknown job type " + j.Type)
	}
}

func ValidTransitionJobState(from system.Status, to system.Status) bool {
	validation := map[system.Status][]system.Status{
		system.ReadyStatus: {
			system.FailedStatus,
			system.RunningStatus,
			system.PausedStatus,
			system.EndStatus,
		},
		system.RunningStatus: {
			system.FailedStatus,
			system.SucceedStatus,
			system.PausedStatus,
		},
		system.FailedStatus: {
			system.ReadyStatus,
			system.EndStatus,
		},
		system.SucceedStatus: {
			system.EndStatus,
		},
		system.PausedStatus: {
			system.ReadyStatus,
			system.EndStatus,
		},
	}

	tos, ok := validation[from]
	if !ok {
		return false
	}
	for _, v := range tos {
		if v == to {
			return true
		}
	}
	return false
}

// TransitionJobState 需要保证，调用此方法时，必然对 j.PropsMu 加写锁了。因为在调用 hookFn 时会先解锁 j.PropsMu 。
func TransitionJobState(j *Job, to system.Status) error {
	var inhibitSignalEmit = false
	if !ValidTransitionJobState(j.Status, to) {
		return fmt.Errorf("can't transition the status of Job(id=%s) %q to %q", j.Id, j.Status, to)
	}
	// 如果是连续下载的job,那么running->succeed或succeed->end不需要发信号
	if j.next != nil && ((j.Status == system.SucceedStatus && to == system.EndStatus) || (j.Status == system.RunningStatus && to == system.SucceedStatus)) {
		inhibitSignalEmit = true
	}
	logger.Infof("%q transition state from %q to %q (Cancelable:%v)\n", j.Id, j.Status, to, j.Cancelable)
	if to == system.FailedStatus && j.retry > 0 {
		j.Status = to
		return nil
	}
	hookFn := j.getPreHook(string(to))
	if hookFn != nil {
		j.PropsMu.Unlock()
		err := hookFn()
		j.PropsMu.Lock()
		// 在切换running和success状态时触发一些检查，如果出错，需要终止并返回error
		if (to == system.RunningStatus || to == system.SucceedStatus) && err != nil {
			return err
		}
	}
	j.Status = to
	if NotUseDBus {
		return nil
	}
	if !inhibitSignalEmit {
		err := j.emitPropChangedStatus(to)
		if err != nil {
			logger.Warning(err)
		}
	}
	hookFn = j.getAfterHook(string(to))
	if hookFn != nil {
		j.PropsMu.Unlock()
		err := hookFn()
		j.PropsMu.Lock()
		// 在切换running和success状态时触发一些检查，如果出错，需要终止并返回error,不过通常不会有success的after hook返回error
		if (to == system.RunningStatus || to == system.SucceedStatus) && err != nil {
			return err
		}
	}
	if j.Status == system.SucceedStatus {
		return TransitionJobState(j, system.EndStatus)
	}
	return nil
}
