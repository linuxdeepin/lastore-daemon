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

package system

import (
	"errors"
)

const VarLibDir = "/var/lib/lastore"

type Status string

const (
	ReadyStatus   Status = "ready"
	RunningStatus Status = "running"
	FailedStatus  Status = "failed"
	SucceedStatus Status = "succeed"
	PausedStatus  Status = "paused"

	EndStatus = "end"
)

const (
	DownloadJobType           = "download"
	InstallJobType            = "install"
	RemoveJobType             = "remove"
	UpdateJobType             = "update"
	DistUpgradeJobType        = "dist_upgrade"
	PrepareDistUpgradeJobType = "prepare_dist_upgrade"
	UpdateSourceJobType       = "update_source"
	CleanJobType              = "clean"
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

type NotFoundErrorType string

func (e NotFoundErrorType) Error() string {
	return string(e)
}

func NotFoundError(w string) NotFoundErrorType {
	return NotFoundErrorType("not found resource: " + w)
}

var NotSupportError = errors.New("not support operation")
var ResourceExitError = errors.New("resource exists")

type Indicator func(JobProgressInfo)

type System interface {
	Download(jobId string, packages []string) error
	Install(jobId string, packages []string) error
	Remove(jobId string, packages []string) error

	DistUpgrade(jobId string) error

	UpdateSource(jobId string) error
	Clean(jobId string) error

	Abort(jobId string) error

	AttachIndicator(Indicator)
}
