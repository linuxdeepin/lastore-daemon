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
	err := TransitionJobState(j, system.RunningStatus)
	j.PropsMu.Unlock()
	if err != nil {
		return err
	}

	switch j.Type {
	case system.DownloadJobType:
		return sys.Download(j.Id, j.Packages)

	case system.InstallJobType:
		return sys.Install(j.Id, j.Packages, j.environ)

	case system.DistUpgradeJobType:
		var args []string
		for key, value := range j.option { // upgradeJobInfo结构体中指定的更新参数
			args = append(args, "-o")
			args = append(args, fmt.Sprintf("%v=%v", key, value))
		}
		return sys.DistUpgrade(j.Id, j.environ, args)

	case system.RemoveJobType:
		return sys.Remove(j.Id, j.Packages, j.environ)

	case system.UpdateSourceJobType:
		return sys.UpdateSource(j.Id)

	case system.UpdateJobType:
		return sys.Install(j.Id, j.Packages, j.environ)

	case system.CleanJobType:
		return sys.Clean(j.Id)

	case system.FixErrorJobType:
		errType := j.Packages[0]
		return sys.FixError(j.Id, errType, j.environ)
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
	if !ValidTransitionJobState(j.Status, to) {
		return fmt.Errorf("can't transition the status of Job(id=%s) %q to %q", j.Id, j.Status, to)
	}
	logger.Infof("%q transition state from %q to %q (Cancelable:%v)\n", j.Id, j.Status, to, j.Cancelable)

	j.Status = to

	if j.Status == system.FailedStatus && j.retry > 0 {
		return nil
	}

	hookFn := j.getHook(string(to))
	if hookFn != nil {
		j.PropsMu.Unlock()
		hookFn()
		j.PropsMu.Lock()
	}
	if NotUseDBus {
		return nil
	}
	err := j.emitPropChangedStatus(to)
	if err != nil {
		logger.Warning(err)
	}

	if j.Status == system.SucceedStatus {
		return TransitionJobState(j, system.EndStatus)
	}
	return nil
}
