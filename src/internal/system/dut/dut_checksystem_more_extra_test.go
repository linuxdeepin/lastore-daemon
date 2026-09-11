// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"errors"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	libCheck "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAllChecksNil swaps every libCheck seam for a stub returning nil so no
// real system check runs. The original implementations are restored on cleanup.
func stubAllChecksNil(t *testing.T) {
	t.Helper()
	nilFn := func() error { return nil }
	old := []func() error{
		preUpdateCheckFn, postUpdateCheckFn, preDownloadCheckFn,
		postDownloadCheckFn, preBackupCheckFn, postBackupCheckFn,
		preUpgradeCheckFn, midUpgradeCheckFn, postUpgradeCheckFn,
	}
	t.Cleanup(func() {
		preUpdateCheckFn = old[0]
		postUpdateCheckFn = old[1]
		preDownloadCheckFn = old[2]
		postDownloadCheckFn = old[3]
		preBackupCheckFn = old[4]
		postBackupCheckFn = old[5]
		preUpgradeCheckFn = old[6]
		midUpgradeCheckFn = old[7]
		postUpgradeCheckFn = old[8]
	})
	preUpdateCheckFn = nilFn
	postUpdateCheckFn = nilFn
	preDownloadCheckFn = nilFn
	postDownloadCheckFn = nilFn
	preBackupCheckFn = nilFn
	postBackupCheckFn = nilFn
	preUpgradeCheckFn = nilFn
	midUpgradeCheckFn = nilFn
	postUpgradeCheckFn = nilFn
}

func TestCheckSystemAllKnownTypesNilExtra(t *testing.T) {
	stubAllChecksNil(t)

	types := []CheckType{
		PreUpdateCheck, PostUpdateCheck, PreDownloadCheck,
		PostDownloadCheck, PreBackupCheck, PostBackupCheck,
		PreUpgradeCheck, MidUpgradeCheck, PostUpgradeCheck,
	}
	for _, typ := range types {
		// nil checkError triggers the early-return branch.
		assert.Nil(t, CheckSystem(typ, nil, nil), "type %s", typ.String())
	}
}

func TestCheckSystemPostUpgradeFirstCheckOneExtra(t *testing.T) {
	stubAllChecksNil(t)
	// FirstCheck=="1" must set the libCheck stage flag to true before running.
	jobErr := CheckSystem(PostUpgradeCheck, map[string]string{OptionFirstCheck: "1"}, nil)
	assert.Nil(t, jobErr)
	assert.True(t, libCheck.PostCheckStage1)
}

func TestCheckSystemPostUpgradeFirstCheckOtherExtra(t *testing.T) {
	stubAllChecksNil(t)
	// Any value other than "1" clears the stage flag.
	jobErr := CheckSystem(PostUpgradeCheck, map[string]string{OptionFirstCheck: "0"}, nil)
	assert.Nil(t, jobErr)
	assert.False(t, libCheck.PostCheckStage1)
}

func TestCheckSystemNonJobErrorWrapExtra(t *testing.T) {
	stubAllChecksNil(t)
	postUpgradeCheckFn = func() error { return errors.New("plain failure") }
	t.Cleanup(func() { postUpgradeCheckFn = libCheck.PostUpgradeCheck })

	var got []system.JobProgressInfo
	indicator := func(info system.JobProgressInfo) {
		got = append(got, info)
	}

	jobErr := CheckSystem(PostUpgradeCheck, nil, indicator)
	require.NotNil(t, jobErr)
	assert.Equal(t, system.JobErrorType("UNKNOWN_ERROR"), jobErr.ErrType)
	assert.Equal(t, "plain failure", jobErr.ErrDetail)
	assert.True(t, jobErr.IsCheckError)

	require.NotEmpty(t, got, "indicator should record the original error log")
	assert.True(t, got[0].OnlyLog)
	assert.Equal(t, "plain failure", got[0].OriginalLog)
}

func TestCheckSystemUnknownTypeErrDetailExtra(t *testing.T) {
	jobErr := CheckSystem(CheckType(42), nil, nil)
	require.NotNil(t, jobErr)
	assert.Equal(t, system.JobErrorType("INVALID_CHECK_TYPE"), jobErr.ErrType)
	assert.Contains(t, jobErr.ErrDetail, "Unknown check type:")
	assert.True(t, jobErr.IsCheckError)
}
