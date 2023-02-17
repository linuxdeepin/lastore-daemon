// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"internal/system"
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
	err := TransitionJobState(j, system.RunningStatus, false)
	j.PropsMu.Unlock()
	if err != nil {
		return err
	}
	var args []string
	for key, value := range j.option { // apt 命令执行参数
		args = append(args, "-o")
		args = append(args, fmt.Sprintf("%v=%v", key, value))
	}
	switch j.Type {
	case system.DownloadJobType:
		return sys.DownloadPackages(j.Id, j.Packages, j.environ, args)

	case system.PrepareDistUpgradeJobType:
		return sys.DownloadSource(j.Id, j.environ, args)

	case system.InstallJobType:
		return sys.Install(j.Id, j.Packages, j.environ, args)

	case system.DistUpgradeJobType:
		return sys.DistUpgrade(j.Id, j.environ, args)

	case system.RemoveJobType:
		return sys.Remove(j.Id, j.Packages, j.environ)

	case system.UpdateSourceJobType:
		return sys.UpdateSource(j.Id, j.environ)

	case system.UpdateJobType:
		return sys.Install(j.Id, j.Packages, j.environ, args)

	case system.CleanJobType:
		return sys.Clean(j.Id)

	case system.FixErrorJobType:
		errType := j.Packages[0]
		return sys.FixError(j.Id, errType, j.environ, args)
	default:
		return system.NotFoundError("StartSystemJob unknown job type " + j.Type)
	}
}

func ValidTransitionJobState(from system.Status, to system.Status) bool {
	validation := map[system.Status][]system.Status{
		system.ReadyStatus: {
			system.RunningStatus,
			system.PausedStatus,
			system.EndStatus,
			system.ReloadStatus,
		},
		system.RunningStatus: {
			system.FailedStatus,
			system.SucceedStatus,
			system.PausedStatus,
		},
		system.FailedStatus: {
			system.ReadyStatus,
			system.EndStatus,
			system.ReloadStatus,
		},
		system.SucceedStatus: {
			system.EndStatus,
		},
		system.PausedStatus: {
			system.ReadyStatus,
			system.EndStatus,
			system.ReloadStatus,
		},
		system.ReloadStatus: {
			system.ReadyStatus,
			system.PausedStatus,
			system.FailedStatus,
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
func TransitionJobState(j *Job, to system.Status, inhibitSignalEmit bool) error {
	if !ValidTransitionJobState(j.Status, to) {
		return fmt.Errorf("can't transition the status of Job(id=%s) %q to %q", j.Id, j.Status, to)
	}
	logger.Infof("%q transition state from %q to %q (Cancelable:%v)\n", j.Id, j.Status, to, j.Cancelable)
	if to == system.FailedStatus && j.retry > 0 {
		j.Status = to
		return nil
	}

	hookFn := j.getHook(string(to))
	if hookFn != nil {
		j.PropsMu.Unlock()
		hookFn()
		j.PropsMu.Lock()
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

	if j.Status == system.SucceedStatus {
		return TransitionJobState(j, system.EndStatus, false)
	}
	return nil
}
