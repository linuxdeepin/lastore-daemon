// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/apt")

func (p *APTSystem) AddCMD(cmd *system.Command) {
	if _, ok := p.CmdSet[cmd.JobId]; ok {
		logger.Warningf("APTSystem AddCMD: exist cmd %q\n", cmd.JobId)
		return
	}
	logger.Infof("APTSystem AddCMD: %v\n", cmd)
	p.CmdSet[cmd.JobId] = cmd
}
func (p *APTSystem) RemoveCMD(id string) {
	c, ok := p.CmdSet[id]
	if !ok {
		logger.Warningf("APTSystem RemoveCMD with invalid Id=%q\n", id)
		return
	}
	logger.Infof("APTSystem RemoveCMD: %v (exitCode:%d)\n", c, c.ExitCode)
	delete(p.CmdSet, id)
}
func (p *APTSystem) FindCMD(id string) *system.Command {
	return p.CmdSet[id]
}

// extractDownloadLimit extracts the value of Acquire::http::Dl-Limit from the command arguments.
// Example input: []string{"package1", "package2", "-o", "Acquire::http::Dl-Limit=300", "-o", "Dir::Etc::SourceList=/dev/null"}
// Returns: The extracted limit (KB/s), or 0 if not found.
func extractDownloadLimit(cmdArgs []string) int {
	// Use a regular expression to match Acquire::http::Dl-Limit=<number>
	dlLimitRegex := regexp.MustCompile(`^Acquire::http::Dl-Limit=(\d+)$`)

	for i, arg := range cmdArgs {
		// Look for the "-o" option
		if arg == "-o" && i+1 < len(cmdArgs) {
			nextArg := cmdArgs[i+1]
			// Check if the next argument matches the Dl-Limit pattern
			if matches := dlLimitRegex.FindStringSubmatch(nextArg); len(matches) > 1 {
				if limit, err := strconv.Atoi(matches[1]); err == nil {
					return limit
				}
			}
		}
	}

	return 0
}

// createIncrementalCommandLine creates the command line for incremental update/download operations.
// It supports extracting download limit from cmdArgs and uses the trickle tool for bandwidth control if possible.
func createIncrementalCommandLine(cmdType string, cmdArgs []string) *exec.Cmd {
	// Extract download limit setting
	downloadLimit := extractDownloadLimit(cmdArgs)
	logger.Debugf("Download limit extracted: %d KB/s", downloadLimit)

	// Build base command arguments
	baseArgs := []string{"upgrade", "--status-fd", "3"}
	if cmdType == system.IncrementalDownloadJobType {
		baseArgs = append(baseArgs, "--download-only")
	}

	// Apply download limit with trickle if set and available
	if downloadLimit > 0 {
		if cmd := buildTrickleCommand(downloadLimit, system.DeepinImmutableCtlPath, baseArgs); cmd != nil {
			return cmd
		}
	}

	// Default command (no bandwidth restriction)
	return exec.Command(system.DeepinImmutableCtlPath, baseArgs...)
}

// buildTrickleCommand constructs a command using trickle for bandwidth control.
// Returns nil if trickle is unavailable, so that the caller can fall back to the default command.
func buildTrickleCommand(downloadLimit int, targetCmd string, targetArgs []string) *exec.Cmd {
	tricklePath, err := exec.LookPath("trickle")
	if err != nil {
		logger.Warningf("trickle binary not found, will not apply download limit: %v", err)
		return nil
	}

	// Run a small test: trickle -s -d 100 echo.
	testCmd := exec.Command(tricklePath, "-s", "-d", "100", "echo")
	err = testCmd.Run()
	if err != nil {
		logger.Warningf("trickle test failed, will not apply download limit: %v", err)
		return nil
	}

	// Build the trickle command: trickle -s -d <limit> <target_cmd> <target_args...>
	trickleArgs := []string{"-s", "-d", strconv.Itoa(downloadLimit), targetCmd}
	trickleArgs = append(trickleArgs, targetArgs...)
	trickleArgs = append(trickleArgs, "--limit")

	logger.Debugf("Applying download limit with trickle: %d KB/s", downloadLimit)
	return exec.Command(tricklePath, trickleArgs...)
}

func createCommandLine(cmdType string, cmdArgs []string) *exec.Cmd {
	var args = []string{"-y"}

	options := map[string]string{
		"APT::Status-Fd": "3",
	}

	if cmdType == system.DownloadJobType || cmdType == system.PrepareDistUpgradeJobType {
		options["Debug::NoLocking"] = "1"
		args = append(args, "-m")
	}

	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}
	switch cmdType {
	case system.InstallJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "install")
		args = append(args, cmdArgs...)
	case system.PrepareDistUpgradeJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "dist-upgrade", "-d", "--allow-change-held-packages")
		args = append(args, cmdArgs...)
	case system.DistUpgradeJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "--allow-downgrades", "--allow-change-held-packages")
		args = append(args, "dist-upgrade")
		args = append(args, cmdArgs...)
	case system.RemoveJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "autoremove", "--allow-change-held-packages")
		args = append(args, cmdArgs...)
	case system.DownloadJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "install", "-d", "--allow-change-held-packages")
		args = append(args, cmdArgs...)
	case system.UpdateSourceJobType:
		args = append(args, cmdArgs...)
		argString := strings.Join(args, " ")
		sh := fmt.Sprintf("apt-get %s -o APT::Status-Fd=3 update --fix-missing && /var/lib/lastore/scripts/build_system_info -now", argString)
		return exec.Command("/bin/sh", "-c", sh)
	case system.CleanJobType:
		return exec.Command("/usr/bin/lastore-apt-clean")
	case system.BackupJobType:
		return exec.Command(system.DeepinImmutableCtlPath, "admin", "deploy", "--backup", "-j")
	case system.IncrementalDownloadJobType, system.IncrementalUpdateJobType:
		return createIncrementalCommandLine(cmdType, cmdArgs)
	case system.FixErrorJobType:
		var errType system.JobErrorType
		if len(cmdArgs) >= 1 {
			errType = system.JobErrorType(cmdArgs[0])
		}
		// FixError 需要加上apt参数项
		var aptOption []string
		var aptOptionString string
		if len(cmdArgs) > 1 {
			aptOption = cmdArgs[1:]
			aptOptionString = strings.Join(aptOption, " ")
		}
		switch errType {
		case system.ErrorDpkgInterrupted:
			sh := "dpkg --force-confold --configure -a;" +
				fmt.Sprintf("apt-get -y -c %s -f install %s;", system.LastoreAptV2CommonConfPath, aptOptionString)
			return exec.Command("/bin/sh", "-c", sh) // #nosec G204
		case system.ErrorDependenciesBroken:
			args = append(args, "-c", system.LastoreAptV2CommonConfPath)
			args = append(args, "-f", "install")
			args = append(args, aptOption...)
		default:
			panic("invalid error type " + errType)
		}
	}

	return exec.Command("apt-get", args...)
}

func newAPTCommand(cmdSet system.CommandSet, jobId string, cmdType string, fn system.Indicator, cmdArgs []string) *system.Command {
	cmd := createCommandLine(cmdType, cmdArgs)

	// See aptCommand.Abort
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

func parseJobError(stdErrStr string, stdOutStr string) *system.JobError {
	switch {
	case strings.Contains(stdErrStr, "Failed to fetch"):
		if strings.Contains(stdErrStr, "rename failed, Operation not permitted") {
			return &system.JobError{
				ErrType:   system.ErrorOperationNotPermitted,
				ErrDetail: stdErrStr,
			}
		}
		if strings.Contains(stdErrStr, "No space left on device") {
			return &system.JobError{
				ErrType:   system.ErrorInsufficientSpace,
				ErrDetail: stdErrStr,
			}
		}
		return &system.JobError{
			ErrType:   system.ErrorFetchFailed,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "Sub-process /usr/bin/dpkg returned an error code"),
		strings.Contains(stdErrStr, "Sub-process /usr/bin/dpkg received a segmentation fault."),
		strings.Contains(stdErrStr, "Sub-process /usr/bin/dpkg exited unexpectedly"):
		idx := strings.Index(stdOutStr, "\ndpkg:")
		var detail string
		if idx == -1 {
			detail = stdOutStr
		} else {
			detail = stdOutStr[idx+1:]
		}

		return &system.JobError{
			ErrType:   system.ErrorDpkgError,
			ErrDetail: detail,
		}

	case strings.Contains(stdErrStr, "Unable to locate package"):
		return &system.JobError{
			ErrType:   system.ErrorPkgNotFound,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "Unable to correct problems,"+
		" you have held broken packages"):

		idx := strings.Index(stdOutStr,
			"The following packages have unmet dependencies:")
		var detail string
		if idx == -1 {
			detail = stdOutStr
		} else {
			detail = stdOutStr[idx:]
		}
		return &system.JobError{
			ErrType:   system.ErrorUnmetDependencies,
			ErrDetail: detail,
		}

	case strings.Contains(stdErrStr, "has no installation candidate"):
		return &system.JobError{
			ErrType:   system.ErrorNoInstallationCandidate,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "You don't have enough free space") || strings.Contains(stdErrStr, "No space left on device"):
		return &system.JobError{
			ErrType:   system.ErrorInsufficientSpace,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "There were unauthenticated packages"):
		return &system.JobError{
			ErrType:   system.ErrorUnauthenticatedPackages,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "I/O error"):
		return &system.JobError{
			ErrType:   system.ErrorIO,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "don't have permission to access"):
		return &system.JobError{
			ErrType:   system.ErrorOperationNotPermitted,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "dpkg: error processing") && strings.Contains(stdErrStr, "--unpack"):
		return &system.JobError{
			ErrType:   system.ErrorDamagePackage,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "Hash Sum mismatch"):
		return &system.JobError{
			ErrType:   system.ErrorDamagePackage,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "Corrupted file"):
		return &system.JobError{
			ErrType:   system.ErrorDamagePackage,
			ErrDetail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "The list of sources could not be read"):
		detail := stdErrStr
		return &system.JobError{
			ErrType:   system.ErrorInvalidSourcesList,
			ErrDetail: detail,
		}

	default:
		return &system.JobError{
			ErrType:   system.ErrorUnknown,
			ErrDetail: stdErrStr,
		}
	}
}

func DownloadPackages(packages []string, environ map[string]string, options map[string]string) (string, error) {
	var args = []string{}
	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}

	args = append(args, "-c", system.LastoreAptV2CommonConfPath)
	args = append(args, "download")
	args = append(args, packages...)
	logger.Debug("download package with args:", args)
	cmd := exec.Command("apt-get", args...)
	tmpPath, err := os.MkdirTemp("/tmp", "apt-download-")
	if err != nil {
		return "", err
	}
	cmd.Dir = tmpPath
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		return "", parseJobError(errBuf.String(), "")
	}
	return tmpPath, nil
}

// In incremental update mode, returns true if all packages are cached, otherwise returns false.
func IsIncrementalUpdateCached() bool {
	cmd := exec.Command("/usr/sbin/deepin-immutable-ctl", "upgrade", "check")
	// Need download count: xxx
	output, err := cmd.Output()
	if err == nil {
		matchFlag := "Need download count: "
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			index := strings.Index(line, matchFlag)
			if index >= 0 {
				count, err := strconv.Atoi(strings.TrimSpace(line[index+len(matchFlag):]))
				if err == nil {
					if count == 0 {
						return true
					}
				}
			}
		}
	}
	return false
}
