// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateInfoErrorError(t *testing.T) {
	err := &UpdateInfoError{Type: "network", Detail: "timeout"}
	assert.Equal(t, "UpdateInfoError type: network, detail: timeout", err.Error())
}

func TestNotFoundErrorTypeError(t *testing.T) {
	e := NotFoundErrorType("not found resource: foo")
	assert.Equal(t, "not found resource: foo", e.Error())
}

func TestNotFoundError(t *testing.T) {
	e := NotFoundError("packages")
	assert.Equal(t, NotFoundErrorMsg+"packages", string(e))
	assert.Equal(t, NotFoundErrorMsg+"packages", e.Error())
}

func TestJobErrorGetType(t *testing.T) {
	je := &JobError{ErrType: JobErrorType("dpkg_error"), ErrDetail: "broken package"}
	assert.Equal(t, "dpkg_error", je.GetType())
}

func TestJobErrorGetDetail(t *testing.T) {
	je := &JobError{ErrType: JobErrorType("dpkg_error"), ErrDetail: "broken package"}
	assert.Equal(t, "broken package", je.GetDetail())
}

func TestJobErrorError(t *testing.T) {
	je := &JobError{ErrType: JobErrorType("dpkg_error"), ErrDetail: "broken package"}
	assert.Equal(t, "JobError ErrType:dpkg_error, ErrDetail: broken package", je.Error())
}

func TestGetAppStoreAppName(t *testing.T) {
	name := GetAppStoreAppName()
	assert.Contains(t, []string{"deepin-app-store", "deepin-appstore"}, name)
}
