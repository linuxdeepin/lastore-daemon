// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"strings"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestParseJobErrorFetchFailed(t *testing.T) {
	err := parseJobError("E: Failed to fetch http://example.com/pkg.deb", "")
	assert.Equal(t, system.ErrorFetchFailed, err.ErrType)
	assert.Contains(t, err.ErrDetail, "Failed to fetch")
}

func TestParseJobErrorOperationNotPermitted(t *testing.T) {
	err := parseJobError("E: Failed to fetch file: rename failed, Operation not permitted", "")
	assert.Equal(t, system.ErrorOperationNotPermitted, err.ErrType)
}

func TestParseJobErrorInsufficientSpaceFetch(t *testing.T) {
	err := parseJobError("E: Failed to fetch http://example.com/pkg.deb: No space left on device", "")
	assert.Equal(t, system.ErrorInsufficientSpace, err.ErrType)
}

func TestParseJobErrorDpkgError(t *testing.T) {
	err := parseJobError("E: Sub-process /usr/bin/dpkg returned an error code (1)", "some output")
	assert.Equal(t, system.ErrorDpkgError, err.ErrType)
	assert.Equal(t, "some output", err.ErrDetail)
}

func TestParseJobErrorDpkgErrorWithDetail(t *testing.T) {
	stdout := "line1\ndpkg: error processing package foo"
	err := parseJobError("E: Sub-process /usr/bin/dpkg returned an error code (1)", stdout)
	assert.Equal(t, system.ErrorDpkgError, err.ErrType)
	assert.Contains(t, err.ErrDetail, "dpkg:")
}

func TestParseJobErrorPkgNotFound(t *testing.T) {
	err := parseJobError("E: Unable to locate package nonexistent", "")
	assert.Equal(t, system.ErrorPkgNotFound, err.ErrType)
}

func TestParseJobErrorUnmetDependencies(t *testing.T) {
	err := parseJobError("E: Unable to correct problems, you have held broken packages", "")
	assert.Equal(t, system.ErrorUnmetDependencies, err.ErrType)
}

func TestParseJobErrorNoInstallationCandidate(t *testing.T) {
	err := parseJobError("E: Package foo has no installation candidate", "")
	assert.Equal(t, system.ErrorNoInstallationCandidate, err.ErrType)
}

func TestParseJobErrorInsufficientSpaceGeneric(t *testing.T) {
	err := parseJobError("E: You don't have enough free space", "")
	assert.Equal(t, system.ErrorInsufficientSpace, err.ErrType)
}

func TestParseJobErrorUnauthenticatedPackages(t *testing.T) {
	err := parseJobError("E: There were unauthenticated packages and -y was used without --allow-unauthenticated", "")
	assert.Equal(t, system.ErrorUnauthenticatedPackages, err.ErrType)
}

func TestParseJobErrorIO(t *testing.T) {
	err := parseJobError("E: I/O error encountered", "")
	assert.Equal(t, system.ErrorIO, err.ErrType)
}

func TestParseJobErrorDamagePackageHash(t *testing.T) {
	err := parseJobError("E: Hash Sum mismatch", "")
	assert.Equal(t, system.ErrorDamagePackage, err.ErrType)
}

func TestParseJobErrorCorruptedFile(t *testing.T) {
	err := parseJobError("E: Corrupted file detected", "")
	assert.Equal(t, system.ErrorDamagePackage, err.ErrType)
}

func TestParseJobErrorInvalidSourcesList(t *testing.T) {
	err := parseJobError("E: The list of sources could not be read", "")
	assert.Equal(t, system.ErrorInvalidSourcesList, err.ErrType)
}

func TestParseJobErrorUnknown(t *testing.T) {
	err := parseJobError("some random error", "")
	assert.Equal(t, system.ErrorUnknown, err.ErrType)
}

func TestParseAptShowListBasic(t *testing.T) {
	input := "The following NEW packages will be installed:\n  foo\n  bar:i386\n\n"
	r := strings.NewReader(input)
	result := parseAptShowList(r, "The following NEW packages will be installed:\n")
	assert.Equal(t, []string{"foo", "bar"}, result)
}

func TestParseAptShowListEmpty(t *testing.T) {
	r := strings.NewReader("nothing here\n")
	result := parseAptShowList(r, "The following NEW packages will be installed:\n")
	assert.Nil(t, result)
}

func TestOptionToArgs(t *testing.T) {
	opts := map[string]string{"key1": "val1", "key2": "val2"}
	args := OptionToArgs(opts)
	assert.Len(t, args, 4)
	for i := 0; i < len(args); i += 2 {
		assert.Equal(t, "-o", args[i])
	}
}

func TestOptionToArgsEmpty(t *testing.T) {
	args := OptionToArgs(map[string]string{})
	assert.Nil(t, args)
}

func TestValidatePackageNamesValid(t *testing.T) {
	err := validatePackageNames([]string{"foo", "bar-baz", "lib123"})
	assert.NoError(t, err)
}

func TestValidatePackageNamesInvalid(t *testing.T) {
	err := validatePackageNames([]string{"FOO"})
	assert.Error(t, err)
}

func TestValidatePackageNamesEmpty(t *testing.T) {
	err := validatePackageNames([]string{})
	assert.NoError(t, err)
}

func TestParseDeliveryDownloadInfoStatus(t *testing.T) {
	line := "102 Status[{IsFinish false} {Speed 10240} {Proto https}]"
	info, err := parseDeliveryDownloadInfo("job1", line)
	assert.NoError(t, err)
	assert.Equal(t, "job1", info.JobId)
	assert.False(t, info.IsFinished)
	assert.Equal(t, int64(10240), info.Speed)
	assert.Equal(t, "https", info.Proto)
}

func TestParseDeliveryDownloadInfoFinished(t *testing.T) {
	line := "102 Status[{IsFinish true} {Speed 0} {Proto https}]"
	info, err := parseDeliveryDownloadInfo("job1", line)
	assert.NoError(t, err)
	assert.True(t, info.IsFinished)
	assert.Equal(t, int64(-1), info.Speed)
}

func TestParseDeliveryDownloadInfoNoStatus(t *testing.T) {
	info, err := parseDeliveryDownloadInfo("job1", "some other line")
	assert.NoError(t, err)
	assert.Equal(t, "job1", info.JobId)
}

func TestParseBackupJobErrorInvalidJSON(t *testing.T) {
	err := parseBackupJobError("stderr msg", "not json")
	assert.Equal(t, system.ErrorUnknown, err.ErrType)
	assert.Contains(t, err.ErrDetail, "invalid")
	assert.Equal(t, []string{"stderr msg"}, err.ErrorLog)
}

func TestParseBackupJobErrorValidJSON(t *testing.T) {
	stdout := `{"code":1,"message":"failed","error":{"code":"E001","message":["detail1"]}}`
	err := parseBackupJobError("stderr", stdout)
	assert.Equal(t, system.ErrorUnknown, err.ErrType)
	assert.Contains(t, err.ErrDetail, "E001")
}

func TestParseBackupJobErrorNoErrorField(t *testing.T) {
	stdout := `{"code":1,"message":"failed"}`
	err := parseBackupJobError("stderr", stdout)
	assert.Equal(t, system.ErrorUnknown, err.ErrType)
	assert.Contains(t, err.ErrDetail, "failed")
}
