package main

const (
	ReadyStatus   = "ready"
	RunningStatus = "running"
	SucceedStatus = "succeed"
	FailedStatus  = "failed"
	PausedStatus  = "paused"
	StartStatus   = "start"
	EndStatus     = "end"
)

func ValidTransitionJobState(from string, to string) bool {
	switch to {
	case ReadyStatus:
		switch from {
		case FailedStatus,
			PausedStatus,
			StartStatus:
		default:
			return false
		}
	case RunningStatus:
		switch from {
		case FailedStatus,
			ReadyStatus,
			PausedStatus:
		default:
			return false
		}
	case FailedStatus,
		SucceedStatus,
		PausedStatus:
		if from != RunningStatus {
			return false
		}
	case EndStatus:
		if from == RunningStatus {
			return false
		}
	}
	return true
}
