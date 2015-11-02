package main

import (
	"fmt"
	"internal/system"
	"pkg.deepin.io/lib/dbus"
)

func (m *Manager) StartJob(jobId string) error {
	j, err := m.JobList.Find(jobId)
	if err != nil {
		return err
	}
	return StartSystemJob(m.b, j)
}

// StartSystemJob start job
// 1. Dispatch Job by type
// 2. Check whether the work queue is empty
func StartSystemJob(sys system.System, j *Job) error {
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

	default:
		return system.NotFoundError
	}
}

func TransitionJobState(j *Job, end system.Status) bool {
	if j.Status == end {
		return true
	}

	switch end {
	case system.ReadyStatus:
		switch j.Status {
		case system.FailedStatus,
			system.PausedStatus,
			system.StartStatus:
		default:
			return false
		}
	case system.RunningStatus:
		switch j.Status {
		case system.FailedStatus,
			system.ReadyStatus,
			system.PausedStatus:
		default:
			return false
		}
	case system.FailedStatus,
		system.SucceedStatus,
		system.PausedStatus:
		if j.Status != system.RunningStatus {
			return false
		}
	case system.EndStatus:
		if j.Status == system.RunningStatus {
			return false
		}
	}

	j.Status = end
	dbus.NotifyChange(j, "Status")
	return true
}
