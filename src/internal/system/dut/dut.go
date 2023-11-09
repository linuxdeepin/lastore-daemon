package dut

import (
	"bytes"
	"encoding/json"
	"fmt"
	"internal/system"
	"os/exec"
	"strings"
	"syscall"

	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/dut")

func newDUTCommand(cmdSet system.CommandSet, jobId string, cmdType string, fn system.Indicator, cmdArgs []string) *system.Command {
	cmd := createCommandLine(cmdType, cmdArgs)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	r := &system.Command{
		JobId:             jobId,
		CmdSet:            cmdSet,
		Indicator:         fn,
		ParseJobError:     parseJobError,
		ParseProgressInfo: parseProgressInfo,
		Cmd:               cmd,
		Cancelable:        true,
	}
	cmd.Stdout = &r.Stdout
	cmd.Stderr = &r.Stderr
	cmdSet.AddCMD(r)
	return r
}

func createCommandLine(cmdType string, cmdArgs []string) *exec.Cmd {
	bin := "deepin-system-update"
	var args []string
	logger.Info("cmdArgs is:", cmdArgs)
	switch cmdType {
	case system.CheckSystemJobType:
		args = append(args, "check")
		args = append(args, cmdArgs...)
		args = append(args, "--ignore-warning")
	case system.DistUpgradeJobType:
		args = append(args, "update")
		args = append(args, cmdArgs...)
	case system.FixErrorJobType:
		bin = "deepin-system-fixpkg"
		args = append(args, "fix")
	default:
		panic("invalid cmd type " + cmdType)
	}
	args = append(args, "-d")
	logger.Info("cmd final args is:", bin, args)
	return exec.Command(bin, args...)
}

type ErrorContent struct {
	Code ErrorCode
	Msg  []string
	Ext  DetailErrorMsg
}

type DetailErrorMsg struct {
	Code ExtCode
	Msg  []string
}

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

func parsePkgSystemError(stdErrStr string, stdOutStr string) error {
	err := parseJobError(stdErrStr, stdOutStr)
	if err != nil {
		return &system.JobError{
			Type:   err.Type,
			Detail: err.Detail,
		}
	}
	return nil
}

func parseJobError(stdErrStr string, stdOutStr string) *system.JobError {
	logger.Info("error message form dut is:", stdErrStr)
	var content ErrorContent
	err := json.Unmarshal([]byte(stdErrStr), &content)
	if err != nil {
		return &system.JobError{
			Type:   system.ErrorUnknown,
			Detail: err.Error(),
		}
	}
	if content.Code != ChkSuccess {
		for k, v := range GetErrorBitMap() {
			if content.Ext.Code&ExtCode(v) != 0 {
				logger.Warningf("short error msg:%v", strings.Join(content.Ext.Msg, ";"))
				return &system.JobError{
					Type:   k,
					Detail: strings.Join(content.Ext.Msg, ";"), // TODO 获取详细的更新错误信息
				}
			}
		}
		// 错误未匹配上，应该是调用者程序错误
		return &system.JobError{
			Type:   system.ErrorProgram,
			Detail: strings.Join(content.Ext.Msg, ";"),
		}
	}
	return nil
}
func parseProgressInfo(id, line string) (system.JobProgressInfo, error) {
	// TODO
	logger.Info("progress message form dut is:", line)
	var content ErrorContent
	err := json.Unmarshal([]byte(line), &content)
	if err != nil {
		return system.JobProgressInfo{JobId: id}, fmt.Errorf("Invlaid Progress line:%q", line)
	}
	if content.Code == ChkSuccess {
		return system.JobProgressInfo{
			JobId:       id,
			Progress:    1,
			Description: "",
			Status:      system.SucceedStatus,
			Cancelable:  false,
		}, nil
	} else {
		return system.JobProgressInfo{
			JobId:       id,
			Progress:    1,
			Description: "",
			Status:      system.FailedStatus,
			Cancelable:  false,
		}, nil
	}
}

func CheckSystem(typ checkType, ifOffline bool, cmdArgs []string) error {
	bin := "deepin-system-update"
	var args []string
	args = append(args, "check")
	args = append(args, typ.String())
	if ifOffline {
		args = append(args, "--meta-cfg")
		args = append(args, system.DutOfflineMetaConfPath)
		// 离线更新暂不涉及precheck和midcheck
		// return nil
	} else {
		args = append(args, "--meta-cfg")
		args = append(args, system.DutOnlineMetaConfPath)
	}
	args = append(args, cmdArgs...)
	args = append(args, "--ignore-warning")
	cmd := exec.Command(bin, args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		return parsePkgSystemError(errBuf.String(), "")
	}
	return nil
}
