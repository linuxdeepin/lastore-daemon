// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package runcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("lastore/update-tools")

// run command output
func RunnerOutput(timeout int, cmd string, args ...string) (string, error) {
	msg, out, err := execAndWait(timeout, cmd, nil, args...)
	if err != nil {
		logger.Debugf("run command %+v\n %s failed\n", err, out)
		return msg, fmt.Errorf("%v\n %s failed", err, out)
	}
	return msg, nil
}

// run command output
func RunnerOutputEnv(timeout int, cmd string, env []string, args ...string) (string, error) {
	msg, out, err := execAndWait(timeout, cmd, env, args...)
	if err != nil {
		logger.Debugf("run command %+v\n %s failed\n", err, out)
		return msg, fmt.Errorf("%v\n %s failed", err, out)
	}
	return msg, nil
}

// run command output
func RunnerNotOutput(timeout int, cmd string, args ...string) error {
	_, out, err := execAndWait(timeout, cmd, nil, args...)
	if err != nil {
		logger.Debugf("run command %+v\n %s failed\n", err, out)
		return err
	}
	return nil
}

// exec and wait for command
func execAndWait(timeout int, name string, env []string, arg ...string) (stdout, stderr string, err error) {
	logger.Debugf("cmd: name=%s, arg=%+v, env=%+v\n", name, arg, env)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, arg...)
	var bufStdout, bufStderr bytes.Buffer
	cmd.Stdout = &bufStdout
	cmd.Stderr = &bufStderr
	cmd.Env = append(os.Environ(), env...)

	err = cmd.Run()
	stdout = bufStdout.String()
	stderr = bufStderr.String()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("command timed out after %d seconds", timeout)
		} else {
			err = fmt.Errorf("command failed: %w", err)
		}
	}

	return
}
