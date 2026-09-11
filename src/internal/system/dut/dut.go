// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"fmt"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	libCheck "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/cli"

	"github.com/linuxdeepin/go-lib/log"
)

const (
	OptionFirstCheck = "firstCheck"
)

var logger = log.NewLogger("lastore/dut")

// CheckSystem performs a system check of the specified type with given options
// and returns a JobError if any issues are found.
func CheckSystem(typ CheckType, options map[string]string, indicator system.Indicator) *system.JobError {
	logger.Debugf("CheckSystem check type: %s, options: %+v", typ.String(), options)

	// 参数验证
	if options == nil {
		options = make(map[string]string)
	}

	libCheck.UpdateMetaConfigPath = system.DutOnlineMetaConfPath
	var checkError error
	switch typ {
	case PreUpdateCheck:
		checkError = preUpdateCheckFn()
	case PostUpdateCheck:
		checkError = postUpdateCheckFn()
	case PreDownloadCheck:
		checkError = preDownloadCheckFn()
	case PostDownloadCheck:
		checkError = postDownloadCheckFn()
	case PreBackupCheck:
		checkError = preBackupCheckFn()
	case PostBackupCheck:
		checkError = postBackupCheckFn()
	case PreUpgradeCheck:
		checkError = preUpgradeCheckFn()
	case MidUpgradeCheck:
		checkError = midUpgradeCheckFn()
	case PostUpgradeCheck:
		if options[OptionFirstCheck] == "1" {
			libCheck.PostCheckStage1 = true
		} else {
			libCheck.PostCheckStage1 = false
		}

		checkError = postUpgradeCheckFn()
	default:
		logger.Errorf("Unknown check type: %s", typ.String())
		checkError = &system.JobError{
			ErrType:      system.JobErrorType("INVALID_CHECK_TYPE"),
			ErrDetail:    fmt.Sprintf("Unknown check type: %s", typ.String()),
			IsCheckError: true,
		}
	}

	if checkError == nil {
		logger.Info("checkError is nil")
		return nil
	}

	// 通过 indicator 记录错误日志，使 processLogFds 能被调用
	if indicator != nil {
		indicator(system.JobProgressInfo{
			OnlyLog:     true,
			OriginalLog: checkError.Error(),
		})
	}

	if jobErr, ok := checkError.(*system.JobError); ok {
		return jobErr
	}

	// 如果不是 JobError 类型，创建一个新的 JobError 包装原始错误
	return &system.JobError{
		ErrType:      system.JobErrorType("UNKNOWN_ERROR"),
		ErrDetail:    checkError.Error(),
		IsCheckError: true,
	}
}
