package system

import (
	"errors"
)

type Status string

const (
	ReadyStatus   Status = "ready"
	RunningStatus        = "running"
	FailedStatus         = "failed"
	SucceedStatus        = "succeed"
	PausedStatus         = "paused"
)

const (
	DownloadJobType    = "download"
	InstallJobType     = "install"
	RemoveJobType      = "remove"
	UpdateJobType      = "update"
	DistUpgradeJobType = "dist_upgrade"
)

type ProgressInfo struct {
	JobId       string
	Progress    float64
	Description string
	Status      Status
}

type UpgradeInfo struct {
	Package        string
	CurrentVersion string
	LastVersion    string
	ChangeLog      string
}

type Architecture string

var NotImplementError = errors.New("not implement")
var NotFoundError = errors.New("not found resource")

type Indicator func(ProgressInfo)

type System interface {
	CheckInstalled(packageId string) bool
	Download(jobId string, packageId string) error
	Install(jobId string, packageId string) error
	Remove(jobId string, packageId string) error

	DistUpgrade() error
	UpgradeInfo() []UpgradeInfo

	Abort(jobId string) error
	AttachIndicator(Indicator)
	SystemArchitectures() []Architecture
}
