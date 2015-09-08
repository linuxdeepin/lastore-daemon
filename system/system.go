package system

import (
	"errors"
)

type Status string

const (
	ReadyStatus     Status = "ready"
	RunningStatus          = "running"
	FailedStatus           = "failed"
	SuccessedStatus        = "success"
)

type ProgressInfo struct {
	JobId       string
	Progress    float64
	Description string
	Status      Status
}

var NotImplementError = errors.New("not implement")
var NotFoundError = errors.New("not found resource")

type Indicator func(ProgressInfo)

type System interface {
	CheckInstalled(packageId string) bool
	Download(jobId string, packageId string) error
	Install(jobId string, packageId string) error
	Remove(jobId string, packageId string) error
	SystemUpgrade()

	Abort(jobId string) error
	Pause(jobId string) error
	Start(jobId string) error
	AttachIndicator(Indicator)
}
