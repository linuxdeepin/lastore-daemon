// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"errors"
	"fmt"
	"os"
)

const VarLibDir = "/var/lib/lastore"
const DefaultMirrorsUrl = "http://packages.deepin.com/mirrors/community.json"

type Status string

const (
	ReadyStatus   Status = "ready"
	RunningStatus Status = "running"
	FailedStatus  Status = "failed"
	SucceedStatus Status = "succeed"
	PausedStatus  Status = "paused"
	EndStatus     Status = "end"
)

const (
	DownloadJobType           = "download"
	InstallJobType            = "install"
	OnlyInstallJobType        = "only_install"
	RemoveJobType             = "remove"
	UpdateJobType             = "update"
	DistUpgradeJobType        = "dist_upgrade"
	PrepareDistUpgradeJobType = "prepare_dist_upgrade"
	UpdateSourceJobType       = "update_source"
	CleanJobType              = "clean"
	FixErrorJobType           = "fix_error"
	CheckSystemJobType        = "check_system"
	CheckDependsJobType       = "check_depends"

	// UpgradeJobType 创建任务时会根据四种下载和安装类型,分别创建带有不同参数的下载和更新任务
	PrepareSystemUpgradeJobType   = "prepare_system_upgrade"
	PrepareAppStoreUpgradeJobType = "prepare_appstore_upgrade"
	PrepareSecurityUpgradeJobType = "prepare_security_upgrade"
	PrepareUnknownUpgradeJobType  = "prepare_unknown_upgrade"
	SystemUpgradeJobType          = "system_upgrade"
	AppStoreUpgradeJobType        = "appstore_upgrade"
	SecurityUpgradeJobType        = "security_upgrade"
	UnknownUpgradeJobType         = "unknown_upgrade"
	OfflineUpgradeJobType         = "offline_update"
	OtherUpgradeJobType           = "other_system_update"
)

const (
	ErrTypeDpkgInterrupted    = "dpkgInterrupted"
	ErrTypeDependenciesBroken = "dependenciesBroken"
	ErrTypeUnknown            = "unknown"
	ErrTypeInvalidSourcesList = "invalidSourceList"
)

const (
	NotifyExpireTimeoutDefault = -1
	NotifyExpireTimeoutNoHide  = 0
)

type JobProgressInfo struct {
	JobId       string
	Progress    float64
	Description string
	Status      Status
	Cancelable  bool
	Error       *JobError
	FatalError  bool
}

type UpgradeInfo struct {
	Package        string
	CurrentVersion string
	LastVersion    string
	ChangeLog      string
	Category       string
}

type UpdateInfoError struct {
	Type   string
	Detail string
}

func (err *UpdateInfoError) Error() string {
	return fmt.Sprintf("UpdateInfoError type: %s, detail: %s",
		err.Type, err.Detail)
}

type SourceUpgradeInfoMap map[string][]UpgradeInfo

type Architecture string

var _NotImplementError = errors.New("not implement")

type NotFoundErrorType string

func (e NotFoundErrorType) Error() string {
	return string(e)
}

const NotFoundErrorMsg = "not found resource: "

func NotFoundError(w string) NotFoundErrorType {
	return NotFoundErrorType(NotFoundErrorMsg + w)
}

var NotSupportError = errors.New("not support operation")
var ResourceExitError = errors.New("resource exists")

type Indicator func(JobProgressInfo)
type ParseProgressInfo func(id, line string) (JobProgressInfo, error)
type ParseJobError func(stdErrStr string, stdOutStr string) *JobError

type System interface {
	OptionToArgs(options map[string]string) []string
	DownloadPackages(jobId string, packages []string, environ map[string]string, cmdArgs []string) error
	DownloadSource(jobId string, environ map[string]string, cmdArgs []string) error
	Install(jobId string, packages []string, environ map[string]string, cmdArgs []string) error
	Remove(jobId string, packages []string, environ map[string]string) error
	DistUpgrade(jobId string, environ map[string]string, cmdArgs []string) error
	UpdateSource(jobId string, environ map[string]string, cmdArgs []string) error
	Clean(jobId string) error
	Abort(jobId string) error
	AbortWithFailed(jobId string) error
	AttachIndicator(Indicator)
	FixError(jobId string, errType string, environ map[string]string, cmdArgs []string) error
	CheckSystem(jobId string, checkType string, environ map[string]string, cmdArgs []string) error
	CheckDepends(jobId string, checkType string, environ map[string]string, cmdArgs []string) error
}

type PkgSystemError struct {
	Type   string
	Detail string
}

func (e *PkgSystemError) GetType() string {
	return "PkgSystemError::" + e.Type
}

func (e *PkgSystemError) GetDetail() string {
	return e.Detail
}

func (e *PkgSystemError) Error() string {
	return fmt.Sprintf("PkgSystemError Type:%s, Detail: %s", e.Type, e.Detail)
}

type JobError struct {
	Type   string
	Detail string
}

func (e *JobError) GetType() string {
	return "JobError::" + e.Type
}

func (e *JobError) GetDetail() string {
	return e.Detail
}

func GetAppStoreAppName() string {
	_, err := os.Stat("/usr/share/applications/deepin-app-store.desktop")
	if err == nil {
		return "deepin-app-store"
	}
	return "deepin-appstore"
}
