package system

import (
	"errors"
)

const VarLibDir = "/var/lib/lastore"

type Status string

const (
	ReadyStatus   Status = "ready"
	RunningStatus        = "running"
	FailedStatus         = "failed"
	SucceedStatus        = "succeed"
	PausedStatus         = "paused"

	EndStatus = "end"
)

const (
	DownloadJobType     = "download"
	InstallJobType      = "install"
	RemoveJobType       = "remove"
	UpdateJobType       = "update"
	DistUpgradeJobType  = "dist_upgrade"
	UpdateSourceJobType = "update_source"
)

type JobProgressInfo struct {
	JobId       string
	Progress    float64
	Description string
	Status      Status
	Cancelable  bool
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
var NotSupportError = errors.New("not support operation")
var ResourceExitError = errors.New("resource exists")

type Indicator func(JobProgressInfo)

type System interface {
	CheckInstallable(pkgId string) bool
	CheckInstalled(packageId string) bool
	Download(jobId string, packages []string) error
	Install(jobId string, packages []string) error
	Remove(jobId string, packages []string) error

	DistUpgrade(jobId string) error
	UpdateSource(jobId string) error
	UpgradeInfo() []UpgradeInfo

	Abort(jobId string) error
	AttachIndicator(Indicator)
	SystemArchitectures() []Architecture
}
