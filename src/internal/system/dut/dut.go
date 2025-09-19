// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dut

import (
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	libCheck "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/cli"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"

	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/dut")

func GetErrorBitMap() map[system.JobErrorType]uint {
	return map[system.JobErrorType]uint{
		system.ErrorUnmetDependencies: ErrorUnmetDependenciesBit,
		system.ErrorInsufficientSpace: ErrorInsufficientSpaceBit,
		system.ErrorPkgNotFound:       ErrorPkgNotFoundBit,
		system.ErrorDpkgError:         ErrorDpkgErrorBit,
		system.ErrorMissCoreFile:      ErrorMissCoreFileBit,
		system.ErrorProgressCheck:     ErrorProgressCheckBit,
		system.ErrorScript:            ErrorScriptBit,
	}
}

const (
	ErrorUnmetDependenciesBit = ChkToolsDependError | ChkPkgDependError | ChkCorePkgDependError
	ErrorInsufficientSpaceBit = ChkDataDiskOutSpace | ChkSysDiskOutSpace
	ErrorPkgNotFoundBit       = ChkCorePkgNotfound | ChkOptionPkgNotfound
	ErrorDpkgErrorBit         = ChkDpkgVersionNotSupported | ChkAptStateError |
		ChkDpkgStateError | ChkPkgListErrState | UpdatePkgInstallFailed |
		ChkPkgListNonexistence | ChkPkgListErrVersion | ChkSysPkgInfoLoadErr |
		UpdateRulesCheckFailed
	ErrorMissCoreFileBit  = ChkCoreFileMiss
	ErrorProgressCheckBit = ChkImportantProgressNotRunning
	ErrorScriptBit        = ChkDynamicScriptErr | ChkPkgConfigError
)

// CheckSystem performs a system check of the specified type with given options
// and returns a JobError if any issues are found.
func CheckSystem(typ CheckType, options map[string]string) *system.JobError {
	logger.Debugf("CheckSystem check type: %s, options: %+v", typ.String(), options)

	libCheck.UpdateMetaConfigPath = system.DutOnlineMetaConfPath
	var checkRetMsg ecode.RetMsg
	switch typ {
	case PreCheck:
		checkRetMsg = libCheck.PreCheck()
	case MidCheck:
		checkRetMsg = libCheck.MidCheck()
	case PostCheck:
		if options[OptionFirstCheck] == "1" {
			libCheck.PostCheckStage1 = true
		} else {
			libCheck.PostCheckStage1 = false
		}

		checkRetMsg = libCheck.PostCheck()
	}

	if checkRetMsg.Code == int64(ChkSuccess) {
		return nil
	}

	errDetailJSON, err := checkRetMsg.ToJson()
	if err != nil {
		logger.Warning("failed to marshal checkRetMsg to json:", err)
	}
	if typ == PreCheck {
		// pre-check only handles hook script errors
		if checkRetMsg.Code == int64(ChkDynError) {
			logger.Warningf("hook script check failed: %v", errDetailJSON)
			return &system.JobError{
				ErrType:      system.ErrorScript,
				ErrDetail:    errDetailJSON,
				IsCheckError: true,
			}
		}
		return nil
	}

	for errType, extCode := range GetErrorBitMap() {
		if checkRetMsg.Ext.Code&int64(extCode) != 0 {
			logger.Warningf("short error msg:%v", strings.Join(checkRetMsg.Ext.Msg, ";"))
			return &system.JobError{
				ErrType:      errType,
				ErrDetail:    errDetailJSON,
				ErrorLog:     checkRetMsg.LogPath,
				IsCheckError: true,
			}
		}
	}
	// error not matched, should be a bug in this program.
	return &system.JobError{
		ErrType:      system.ErrorProgram,
		ErrDetail:    errDetailJSON,
		IsCheckError: true,
	}
}
