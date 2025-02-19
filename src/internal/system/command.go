// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type CommandSet interface {
	AddCMD(cmd *Command)
	RemoveCMD(id string)
	FindCMD(id string) *Command
}

type Command struct {
	JobId      string
	Cancelable bool

	CmdSet CommandSet

	Cmd      *exec.Cmd
	cmdMu    sync.Mutex
	ExitCode int

	pipe *os.File

	Indicator         Indicator
	ParseProgressInfo ParseProgressInfo
	ParseJobError     ParseJobError

	Stdout   bytes.Buffer
	Stderr   bytes.Buffer
	AtExitFn func() bool
}

func (c *Command) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.Cmd.Args, " "))
}

func (c *Command) SetEnv(envVarMap map[string]string) {
	if envVarMap == nil {
		return
	}

	envVarSlice := os.Environ()
	for key, value := range envVarMap {
		envVarSlice = append(envVarSlice, key+"="+value)
	}
	c.Cmd.Env = envVarSlice
}

func (c *Command) Start() error {
	rr, ww, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("aptCommand.Start pipe : %v", err)
	}

	// It must be closed after c.osCMD.Start
	defer func() {
		_ = ww.Close()
	}()

	c.Cmd.ExtraFiles = append(c.Cmd.ExtraFiles, ww)

	c.cmdMu.Lock()
	err = c.Cmd.Start()
	c.cmdMu.Unlock()
	if err != nil {
		_ = rr.Close()
		return err
	}

	c.pipe = rr

	go c.updateProgress()

	go func() {
		_ = c.Wait()
	}()

	return nil
}

func (c *Command) Wait() (err error) {
	err = c.Cmd.Wait()
	if c.ExitCode != ExitPause {
		if err != nil {
			c.ExitCode = ExitFailure
			logger.Infof("aptCommand.Wait: %v\n", err)
		} else {
			c.ExitCode = ExitSuccess
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

func (c *Command) atExit() {
	err := c.pipe.Close()
	if err != nil {
		logger.Warning("failed to close pipe:", err)
	}

	logger.Infof("job %s Stdout: %s", c.JobId, c.Stdout.Bytes())
	logger.Infof("job %s Stderr: %s", c.JobId, c.Stderr.Bytes())

	c.CmdSet.RemoveCMD(c.JobId)

	if c.AtExitFn != nil {
		shouldReturn := c.AtExitFn()
		if shouldReturn {
			return
		}
	}

	switch c.ExitCode {
	case ExitSuccess:
		c.Indicator(JobProgressInfo{
			JobId:      c.JobId,
			Status:     SucceedStatus,
			Progress:   1.0,
			Cancelable: false,
		})
	case ExitFailure:
		err := c.ParseJobError(c.Stderr.String(), c.Stdout.String())
		if err != nil {
			c.Indicator(JobProgressInfo{
				JobId:      c.JobId,
				Status:     FailedStatus,
				Progress:   -1.0,
				Cancelable: true,
				Error:      err,
			})
		} else {
			// 解析错误后，确定错误为非阻塞性错误，那么认为是成功
			c.Indicator(JobProgressInfo{
				JobId:      c.JobId,
				Status:     SucceedStatus,
				Progress:   1.0,
				Cancelable: false,
				Error:      nil,
			})
		}

	case ExitPause:
		c.Indicator(JobProgressInfo{
			JobId:      c.JobId,
			Status:     PausedStatus,
			Progress:   -1.0,
			Cancelable: true,
		})
	}
}

func (c *Command) IndicateFailed(errType JobErrorType, errDetail string, isFatalErr bool) {
	logger.Warningf("IndicateFailed: type: %s, detail: %s", errType, errDetail)
	progressInfo := JobProgressInfo{
		JobId:      c.JobId,
		Progress:   -1.0,
		Status:     FailedStatus,
		Cancelable: true,
		Error: &JobError{
			ErrType:   errType,
			ErrDetail: errDetail,
		},
		FatalError: isFatalErr,
	}
	c.CmdSet.RemoveCMD(c.JobId)
	c.Indicator(progressInfo)
}

func (c *Command) Abort() error {
	return c.abort(false)
}

func (c *Command) AbortWithFailed() error {
	return c.abort(true)
}

func (c *Command) abort(withFailed bool) error {
	if c.Cancelable {
		c.cmdMu.Lock()
		defer c.cmdMu.Unlock()
		if c.Cmd.Process == nil {
			return errors.New("the process has not yet started")
		}

		logger.Debugf("Abort Command: %v\n", c)
		if withFailed {
			c.ExitCode = ExitFailure
		} else {
			c.ExitCode = ExitPause
		}
		var err error
		pgid, err := syscall.Getpgid(c.Cmd.Process.Pid)
		if err != nil {
			return err
		}
		return syscall.Kill(-pgid, 2)
	}
	return NotSupportError
}

func (c *Command) updateProgress() {
	b := bufio.NewReader(c.pipe)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, err := c.ParseProgressInfo(c.JobId, line)
		if err != nil {
			logger.Errorf("aptCommand.updateProgress %v -> %v\n", info, err)
			continue
		}

		c.Cancelable = info.Cancelable
		c.Indicator(info)
	}
}
