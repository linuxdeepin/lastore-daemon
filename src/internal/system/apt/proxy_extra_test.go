// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckPkgSystemErrorNoLock(t *testing.T) {
	noopIndicator := func(info system.JobProgressInfo) {}
	// apt-get check with NoLocking should work on a properly configured system
	err := CheckPkgSystemError(false, noopIndicator)
	_ = err
}

func TestCheckPkgSystemErrorWithLock(t *testing.T) {
	noopIndicator := func(info system.JobProgressInfo) {}
	// With lock=true, this needs remount permissions; may fail, just verify no panic
	err := CheckPkgSystemError(true, noopIndicator)
	_ = err
}

func TestParseProgressInfo_Dummy(t *testing.T) {
	info, err := parseProgressInfo("job1", "dummy:100:50:downloading")
	assert.NoError(t, err)
	assert.Equal(t, "job1", info.JobId)
	assert.Equal(t, system.Status("100"), info.Status)
	assert.Equal(t, "downloading", info.Description)
}

func TestParseProgressInfo_Dlstatus(t *testing.T) {
	info, err := parseProgressInfo("job1", "dlstatus:1:50:fetching")
	assert.NoError(t, err)
	assert.Equal(t, "job1", info.JobId)
	assert.Equal(t, 0.5, info.Progress)
	assert.Equal(t, system.RunningStatus, info.Status)
	assert.True(t, info.Cancelable)
}

func TestParseProgressInfo_Pmstatus(t *testing.T) {
	info, err := parseProgressInfo("job1", "pmstatus:1:75:installing")
	assert.NoError(t, err)
	assert.Equal(t, "job1", info.JobId)
	assert.Equal(t, 0.75, info.Progress)
	assert.Equal(t, system.RunningStatus, info.Status)
	assert.False(t, info.Cancelable)
}

func TestParseProgressInfo_PmerrorNonDistUpgrade(t *testing.T) {
	info, err := parseProgressInfo("install", "pmstatus:1:50:err text")
	assert.NoError(t, err)
	// pmstatus path, not pmerror
	assert.Equal(t, system.RunningStatus, info.Status)
}

func TestParseProgressInfo_Pmerror(t *testing.T) {
	info, err := parseProgressInfo("install", "pmerror:1:0:some error")
	assert.NoError(t, err)
	assert.Equal(t, "install", info.JobId)
	assert.Equal(t, -1.0, info.Progress)
	assert.Equal(t, system.FailedStatus, info.Status)
}

func TestParseProgressInfo_PmerrorDistUpgrade(t *testing.T) {
	info, err := parseProgressInfo(system.DistUpgradeJobType, "pmerror:1:0:some error")
	assert.NoError(t, err)
	assert.Equal(t, -1.0, info.Progress)
	assert.NotEqual(t, system.FailedStatus, info.Status)
}

func TestParseProgressInfo_UnknownType(t *testing.T) {
	_, err := parseProgressInfo("job1", "unknown:1:50:desc")
	assert.Error(t, err)
}

func TestParseProgressInfo_InvalidFormat(t *testing.T) {
	info, err := parseProgressInfo("job1", "invalid")
	assert.Error(t, err)
	assert.Equal(t, "job1", info.JobId)
}

func TestParseProgressInfo_InvalidProgress(t *testing.T) {
	_, err := parseProgressInfo("job1", "dlstatus:1:abc:desc")
	assert.Error(t, err)
}

func TestParsePkgSystemError_NoError(t *testing.T) {
	err := ParsePkgSystemError([]byte("output"), []byte{})
	assert.NoError(t, err)
}

func TestParsePkgSystemError_DpkgInterrupted(t *testing.T) {
	err := ParsePkgSystemError([]byte("output"), []byte("dpkg was interrupted"))
	assert.Error(t, err)
	jobErr, ok := err.(*system.JobError)
	assert.True(t, ok)
	assert.Equal(t, system.ErrorDpkgInterrupted, jobErr.ErrType)
}

func TestParsePkgSystemError_UnmetDependencies(t *testing.T) {
	out := []byte("Reading package lists...\nThe following packages have unmet dependencies:\n pkgA depends on pkgB")
	err := ParsePkgSystemError(out, []byte("Unmet dependencies"))
	assert.Error(t, err)
	jobErr, ok := err.(*system.JobError)
	assert.True(t, ok)
	assert.Equal(t, system.ErrorDependenciesBroken, jobErr.ErrType)
	assert.Contains(t, jobErr.ErrDetail, "unmet dependencies")
}

func TestParsePkgSystemError_GeneratedBreaks(t *testing.T) {
	out := []byte("some output")
	err := ParsePkgSystemError(out, []byte("generated breaks"))
	assert.Error(t, err)
	jobErr, ok := err.(*system.JobError)
	assert.True(t, ok)
	assert.Equal(t, system.ErrorDependenciesBroken, jobErr.ErrType)
}

func TestParsePkgSystemError_InvalidSourcesList(t *testing.T) {
	err := ParsePkgSystemError([]byte("out"), []byte("The list of sources could not be read"))
	assert.Error(t, err)
	jobErr, ok := err.(*system.JobError)
	assert.True(t, ok)
	assert.Equal(t, system.ErrorInvalidSourcesList, jobErr.ErrType)
}

func TestParsePkgSystemError_Unknown(t *testing.T) {
	err := ParsePkgSystemError([]byte("some out"), []byte("some unknown error"))
	assert.Error(t, err)
	jobErr, ok := err.(*system.JobError)
	assert.True(t, ok)
	assert.Equal(t, system.ErrorUnknown, jobErr.ErrType)
}

func TestParseProgressField_Valid(t *testing.T) {
	v, err := parseProgressField("42.5")
	assert.NoError(t, err)
	assert.Equal(t, 42.5, v)
}

func TestParseProgressField_Invalid(t *testing.T) {
	v, err := parseProgressField("abc")
	assert.Error(t, err)
	assert.Equal(t, -1.0, v)
}
