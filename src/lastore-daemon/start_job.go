/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"internal/system"

	log "github.com/cihub/seelog"
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
		return sys.DistUpgrade(j.Id, j.environ)

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
	validtion := map[system.Status][]system.Status{
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

	tos, ok := validtion[from]
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

func TransitionJobState(j *Job, to system.Status) error {
	if !ValidTransitionJobState(j.Status, to) {
		return fmt.Errorf("can't transition the status of Job %v to %q", j, to)
	}
	log.Infof("%q transition state from %q to %q (Cancelable:%v)\n", j.Id, j.Status, to, j.Cancelable)

	j.Status = to

	if j.Status == system.FailedStatus && j.retry > 0 {
		return nil
	}

	j.emitPropChangedStatus(to)

	if j.Status == system.SucceedStatus {
		return TransitionJobState(j, system.EndStatus)
	}
	return nil
}
