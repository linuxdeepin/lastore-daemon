// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	FlushName = "/tmp/lastore_update_detail.log"
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

	ff *fileFlush
}

func (c *Command) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.Cmd.Args, " "))
}

func (c *Command) SetEnv(envVarMap map[string]string) {
	if envVarMap == nil {
		return
	}

	// Create a map from existing environment variables
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) == 2 {
			envMap[pair[0]] = pair[1]
		}
	}

	// Update with new values, overwriting existing keys
	for key, value := range envVarMap {
		envMap[key] = value
	}

	// Convert back to slice
	envVarSlice := make([]string, 0, len(envMap))
	for key, value := range envMap {
		envVarSlice = append(envVarSlice, key+"="+value)
	}

	c.Cmd.Env = envVarSlice
}

func (c *Command) Start() error {
	var err error
	c.ff, err = OpenFlush(FlushName)
	if err != nil {
		return err
	}

	rr, ww, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("aptCommand.Start pipe : %v", err)
	}

	defer func() {
		_ = ww.Close()
	}()

	// Print command start information
	cmdStr := strings.Join(c.Cmd.Args, " ")
	startMsg := fmt.Sprintf("=== Job %s running: %s ===\n", c.JobId, cmdStr)
	if c.ff != nil {
		c.ff.SetFlushCmd(c.Cmd)
		_, err := c.ff.WriteString(startMsg)
		if err != nil {
			logger.Warning("failed to write start message to log file:", err)
		} else {
			c.ff.Sync()
		}
	}

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

	// Print command end information with status
	var statusStr string
	switch c.ExitCode {
	case ExitSuccess:
		statusStr = "SUCCESS"
	case ExitFailure:
		statusStr = "FAILED"
	case ExitPause:
		statusStr = "PAUSED"
	default:
		statusStr = "UNKNOWN"
	}

	cmdStr := strings.Join(c.Cmd.Args, " ")
	endMsg := fmt.Sprintf("=== Job %s end: %s [Status: %s] ===\n", c.JobId, cmdStr, statusStr)
	logger.Info(endMsg)
	if c.ff != nil {
		_, err := c.ff.WriteString(endMsg)
		if err != nil {
			logger.Warning("failed to write end message to log file:", err)
		} else {
			c.ff.Sync()
		}
	}

	// Close log file when process exits
	if c.ff != nil {
		err := c.ff.Close()
		if err != nil {
			logger.Warning("failed to close log file:", err)
		}
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

	// Print command end information with failed status and close log file
	cmdStr := strings.Join(c.Cmd.Args, " ")
	endMsg := fmt.Sprintf("=== Job %s end: %s [Status: FAILED - %s] ===\n", c.JobId, cmdStr, errType)
	logger.Info(endMsg)
	if c.ff != nil {
		_, err := c.ff.WriteString(endMsg)
		if err != nil {
			logger.Warning("failed to write end message to log file:", err)
		} else {
			c.ff.Sync()
		}
		// Close log file when indicating failed
		err = c.ff.Close()
		if err != nil {
			logger.Warning("failed to close log file:", err)
		}
	}

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

		// Write pipe output to log file
		if c.ff != nil {
			_, err := c.ff.WriteString(line)
			if err != nil {
				logger.Warning("failed to write pipe output to log file:", err)
			} else {
				// Ensure data is written to disk immediately
				c.ff.Sync()
			}
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

type fileFlush struct {
	fileName string
	file     *os.File
	fileMu   sync.Mutex
}

func OpenFlush(file string) (*fileFlush, error) {
	if file == "" {
		return nil, fmt.Errorf("file name is empty")
	}

	ff := &fileFlush{fileName: file}
	ff.fileMu.Lock()
	defer ff.fileMu.Unlock()

	var err error
	ff.file, err = os.OpenFile(ff.fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return ff, nil
}

func (ff *fileFlush) Close() error {
	ff.fileMu.Lock()
	defer ff.fileMu.Unlock()
	if ff.file != nil {
		return ff.file.Close()
	}
	return nil
}

func (ff *fileFlush) SetFlushCmd(cmd *exec.Cmd) error {
	ff.fileMu.Lock()
	defer ff.fileMu.Unlock()
	if ff.file == nil {
		return fmt.Errorf("file is not open")
	}

	// Handle case where cmd.Stdout/Stderr might be nil
	if cmd.Stdout != nil {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, ff)
	} else {
		cmd.Stdout = ff
	}

	if cmd.Stderr != nil {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, ff)
	} else {
		cmd.Stderr = ff
	}

	return nil
}

func (ff *fileFlush) Write(data []byte) (int, error) {
	ff.fileMu.Lock()
	defer ff.fileMu.Unlock()
	if ff.file == nil {
		return 0, fmt.Errorf("file is not open")
	}

	// Add timestamp to each line
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	lines := strings.Split(string(data), "\n")
	var timestampedLines []string

	for _, line := range lines {
		if line != "" {
			timestampedLines = append(timestampedLines, fmt.Sprintf("[%s] %s", timestamp, line))
		} else {
			timestampedLines = append(timestampedLines, "")
		}
	}

	timestampedData := []byte(strings.Join(timestampedLines, "\n"))
	_, err := ff.file.Write(timestampedData)
	if err != nil {
		return 0, err
	}

	// 重要：必须返回原始数据的长度，不是时间戳数据的长度
	return len(data), nil
}

func (ff *fileFlush) WriteString(data string) (int, error) {
	return ff.Write([]byte(data))
}

func (ff *fileFlush) Sync() error {
	ff.fileMu.Lock()
	defer ff.fileMu.Unlock()
	if ff.file == nil {
		return fmt.Errorf("file is not open")
	}
	return ff.file.Sync()
}
