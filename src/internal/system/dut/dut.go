package dut

import (
	"internal/system"
	"os/exec"
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
	case system.CheckDependsJobType:
		bin = "deepin-system-fixpkg"
		args = append(args, "check")

	case system.CheckSystemJobType:
		args = append(args, "check")
		// precheck --before-download ;precheck --after-download; midcheck ;postcheck --check-succeed;postcheck --check-failed
		args = append(args, cmdArgs...)
		args = append(args, "--ignore-warning")
		args = append(args, []string{
			"--meta-cfg",
			system.DutMetaConfPath,
		}...)
	case system.DistUpgradeJobType:
		args = append(args, "update")
		args = append(args, []string{
			"--meta-cfg",
			system.DutMetaConfPath,
		}...)
	case system.FixErrorJobType:
		bin = "deepin-system-fixpkg"
		args = append(args, "fix")
	default:
		panic("invalid cmd type " + cmdType)
	}
	logger.Info("cmd final args is:", bin, args)
	return exec.Command(bin, args...)
}

func parseJobError(stdErrStr string, stdOutStr string) *system.JobError {
	// TODO
	return nil
}
func parseProgressInfo(id, line string) (system.JobProgressInfo, error) {
	// TODO
	return system.JobProgressInfo{}, nil
}
