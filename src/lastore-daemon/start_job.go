package main

import (
	"fmt"
	"internal/system"
	"log"
	"pkg.deepin.io/lib/dbus"
)

// StartSystemJob start job
// 1. Dispatch Job by type
// 2. Check whether the work queue is empty
func StartSystemJob(sys system.System, j *Job) error {
	if j == nil {
		panic("StartSystemJob with nil")
	}

	if !TransitionJobState(j, system.RunningStatus) {
		return fmt.Errorf("Can't transition state from %q to %q",
			j.Status, system.RunningStatus)
	}

	switch j.Type {
	case system.DownloadJobType:
		return sys.Download(j.Id, j.PackageId)

	case system.InstallJobType:
		return sys.Install(j.Id, j.PackageId)

	case system.RemoveJobType:
		return sys.Remove(j.Id, j.PackageId)

	case system.DistUpgradeJobType:
		return sys.DistUpgrade()

	case system.UpdateJobType:
		return sys.Install(j.Id, j.PackageId)

	default:
		return system.NotFoundError
	}
}

func ValidTransitionJobState(from system.Status, to system.Status) bool {
	switch to {
	case system.ReadyStatus:
		switch from {
		case system.FailedStatus,
			system.PausedStatus,
			system.StartStatus:
		default:
			return false
		}
	case system.RunningStatus:
		switch from {
		case system.FailedStatus,
			system.ReadyStatus,
			system.PausedStatus:
		default:
			return false
		}
	case system.FailedStatus,
		system.SucceedStatus,
		system.PausedStatus:
		if from != system.RunningStatus {
			return false
		}
	case system.EndStatus:
		if from == system.RunningStatus {
			return false
		}
	}
	return true
}

func TransitionJobState(j *Job, to system.Status) bool {
	if j.Status == to {
		return true
	}

	if !ValidTransitionJobState(j.Status, to) {
		return false
	}
	log.Printf("%q transition state from %q to %q\n", j.Id, j.Status, to)
	j.Status = to
	dbus.NotifyChange(j, "Status")
	return true
}
