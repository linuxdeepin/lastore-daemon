// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"internal/system"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/linuxdeepin/go-lib/log"
)

type CommandSet interface {
	AddCMD(cmd *aptCommand)
	RemoveCMD(id string)
	FindCMD(id string) *aptCommand
}

var logger = log.NewLogger("lastore")

func (p *APTSystem) AddCMD(cmd *aptCommand) {
	if _, ok := p.cmdSet[cmd.JobId]; ok {
		logger.Warningf("APTSystem AddCMD: exist cmd %q\n", cmd.JobId)
		return
	}
	logger.Infof("APTSystem AddCMD: %v\n", cmd)
	p.cmdSet[cmd.JobId] = cmd
}
func (p *APTSystem) RemoveCMD(id string) {
	c, ok := p.cmdSet[id]
	if !ok {
		logger.Warningf("APTSystem RemoveCMD with invalid Id=%q\n", id)
		return
	}
	logger.Infof("APTSystem RemoveCMD: %v (exitCode:%d)\n", c, c.exitCode)
	delete(p.cmdSet, id)
}
func (p *APTSystem) FindCMD(id string) *aptCommand {
	return p.cmdSet[id]
}

type aptCommand struct {
	JobId      string
	Cancelable bool

	cmdSet CommandSet

	apt      *exec.Cmd
	aptMu    sync.Mutex
	exitCode int

	aptPipe *os.File

	indicator system.Indicator

	stdout   bytes.Buffer
	stderr   bytes.Buffer
	atExitFn func() bool
}

func (c *aptCommand) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.apt.Args, " "))
}

func createCommandLine(cmdType string, cmdArgs []string) *exec.Cmd {
	var args = []string{"-y"}

	options := map[string]string{
		"APT::Status-Fd": "3",
	}

	if cmdType == system.DownloadJobType {
		options["Debug::NoLocking"] = "1"
		options["Acquire::Retries"] = "1"
		args = append(args, "-m")
	}

	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}
	switch cmdType {
	case system.InstallJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "install")
		args = append(args, "--")
		args = append(args, cmdArgs...)
	case system.DistUpgradeJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "--allow-downgrades", "--allow-change-held-packages")
		args = append(args, "dist-upgrade")
		args = append(args, cmdArgs...)
	case system.RemoveJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "autoremove", "--allow-change-held-packages")
		args = append(args, "--")
		args = append(args, cmdArgs...)
	case system.DownloadJobType:
		args = append(args, "-c", system.LastoreAptV2CommonConfPath)
		args = append(args, "install", "-d", "--allow-change-held-packages")
		args = append(args, "--")
		args = append(args, cmdArgs...)
	case system.UpdateSourceJobType:
		sh := "apt-get -y -o APT::Status-Fd=3 update && /var/lib/lastore/scripts/build_system_info -now"
		return exec.Command("/bin/sh", "-c", sh)
	case system.CleanJobType:
		return exec.Command("/usr/bin/lastore-apt-clean")

	case system.FixErrorJobType:
		errType := cmdArgs[0]
		switch errType {
		case system.ErrTypeDpkgInterrupted:
			sh := "dpkg --force-confold --configure -a;" +
				fmt.Sprintf("apt-get -y -c %s -f install;", system.LastoreAptV2CommonConfPath)
			return exec.Command("/bin/sh", "-c", sh) // #nosec G204
		case system.ErrTypeDependenciesBroken:
			args = append(args, "-c", system.LastoreAptV2CommonConfPath)
			args = append(args, "-f", "install")
		default:
			panic("invalid error type " + errType)
		}
	}

	return exec.Command("apt-get", args...)
}

func newAPTCommand(cmdSet CommandSet, jobId string, cmdType string, fn system.Indicator, cmdArgs []string) *aptCommand {
	cmd := createCommandLine(cmdType, cmdArgs)

	// See aptCommand.Abort
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	r := &aptCommand{
		JobId:      jobId,
		cmdSet:     cmdSet,
		indicator:  fn,
		apt:        cmd,
		Cancelable: true,
	}
	cmd.Stdout = &r.stdout
	cmd.Stderr = &r.stderr

	cmdSet.AddCMD(r)
	return r
}

func (c *aptCommand) setEnv(envVarMap map[string]string) {
	if envVarMap == nil {
		return
	}

	envVarSlice := os.Environ()
	for key, value := range envVarMap {
		envVarSlice = append(envVarSlice, key+"="+value)
	}
	c.apt.Env = envVarSlice
}

func (c *aptCommand) Start() error {
	rr, ww, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("aptCommand.Start pipe : %v", err)
	}

	// It must be closed after c.osCMD.Start
	defer func() {
		_ = ww.Close()
	}()

	c.apt.ExtraFiles = append(c.apt.ExtraFiles, ww)

	c.aptMu.Lock()
	err = c.apt.Start()
	c.aptMu.Unlock()
	if err != nil {
		_ = rr.Close()
		return err
	}

	c.aptPipe = rr

	go c.updateProgress()

	go func() {
		_ = c.Wait()
	}()

	return nil
}

func (c *aptCommand) Wait() (err error) {
	err = c.apt.Wait()
	if c.exitCode != ExitPause {
		if err != nil {
			c.exitCode = ExitFailure
			logger.Infof("aptCommand.Wait: %v\n", err)
		} else {
			c.exitCode = ExitSuccess
		}
	}
	c.atExit()
	return err
}

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitPause   = 2
)

func (c *aptCommand) atExit() {
	err := c.aptPipe.Close()
	if err != nil {
		logger.Warning("failed to close pipe:", err)
	}

	logger.Infof("job %s stdout: %s", c.JobId, c.stdout.Bytes())
	logger.Infof("job %s stderr: %s", c.JobId, c.stderr.Bytes())

	c.cmdSet.RemoveCMD(c.JobId)

	if c.atExitFn != nil {
		shouldReturn := c.atExitFn()
		if shouldReturn {
			return
		}
	}

	switch c.exitCode {
	case ExitSuccess:
		c.indicator(system.JobProgressInfo{
			JobId:      c.JobId,
			Status:     system.SucceedStatus,
			Progress:   1.0,
			Cancelable: false,
		})
	case ExitFailure:
		err := parseJobError(c.stderr.String(), c.stdout.String())
		c.indicator(system.JobProgressInfo{
			JobId:      c.JobId,
			Status:     system.FailedStatus,
			Progress:   -1.0,
			Cancelable: true,
			Error:      err,
		})
	case ExitPause:
		c.indicator(system.JobProgressInfo{
			JobId:      c.JobId,
			Status:     system.PausedStatus,
			Progress:   -1.0,
			Cancelable: true,
		})
	}
}

func parseJobError(stdErrStr string, stdOutStr string) *system.JobError {
	switch {
	case strings.Contains(stdErrStr, "Failed to fetch"):
		return &system.JobError{
			Type:   "fetchFailed",
			Detail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "Sub-process /usr/bin/dpkg"+
		" returned an error code"):
		idx := strings.Index(stdOutStr, "\ndpkg:")
		var detail string
		if idx == -1 {
			detail = stdOutStr
		} else {
			detail = stdOutStr[idx+1:]
		}

		return &system.JobError{
			Type:   "dpkgError",
			Detail: detail,
		}

	case strings.Contains(stdErrStr, "Unable to locate package"):
		return &system.JobError{
			Type:   "pkgNotFound",
			Detail: stdErrStr,
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
			Type:   "unmetDependencies",
			Detail: detail,
		}

	case strings.Contains(stdErrStr, "has no installation candidate"):
		return &system.JobError{
			Type:   "noInstallationCandidate",
			Detail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "You don't have enough free space"):
		return &system.JobError{
			Type:   "insufficientSpace",
			Detail: stdErrStr,
		}

	case strings.Contains(stdErrStr, "There were unauthenticated packages"):
		return &system.JobError{
			Type:   "unauthenticatedPackages",
			Detail: stdErrStr,
		}

	default:
		return &system.JobError{
			Type:   "unknown",
			Detail: stdErrStr,
		}
	}
}

func (c *aptCommand) indicateFailed(errType, errDetail string, isFatalErr bool) {
	logger.Warningf("indicateFailed: type: %s, detail: %s", errType, errDetail)
	progressInfo := system.JobProgressInfo{
		JobId:      c.JobId,
		Progress:   -1.0,
		Status:     system.FailedStatus,
		Cancelable: true,
		Error: &system.JobError{
			Type:   errType,
			Detail: errDetail,
		},
		FatalError: isFatalErr,
	}
	c.cmdSet.RemoveCMD(c.JobId)
	c.indicator(progressInfo)
}

func (c *aptCommand) Abort() error {
	if c.Cancelable {
		c.aptMu.Lock()
		defer c.aptMu.Unlock()
		if c.apt.Process == nil {
			return errors.New("the process has not yet started")
		}

		logger.Debugf("Abort Command: %v\n", c)
		c.exitCode = ExitPause
		var err error
		pgid, err := syscall.Getpgid(c.apt.Process.Pid)
		if err != nil {
			return err
		}
		return syscall.Kill(-pgid, 2)
	}
	return system.NotSupportError
}

func (c *aptCommand) updateProgress() {
	b := bufio.NewReader(c.aptPipe)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, err := ParseProgressInfo(c.JobId, line)
		if err != nil {
			logger.Errorf("aptCommand.updateProgress %v -> %v\n", info, err)
			continue
		}

		c.Cancelable = info.Cancelable
		c.indicator(info)
	}
}
