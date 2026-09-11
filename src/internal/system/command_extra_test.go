// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandString(t *testing.T) {
	cmd := &Command{
		JobId:      "job123",
		Cancelable: true,
		Cmd:        exec.Command("apt-get", "install", "foo"),
	}
	s := cmd.String()
	assert.Contains(t, s, "job123")
	assert.Contains(t, s, "apt-get install foo")
	assert.Contains(t, s, "true")
}

func TestCommandStringEmptyArgs(t *testing.T) {
	cmd := &Command{
		JobId: "job0",
		Cmd:   &exec.Cmd{},
	}
	s := cmd.String()
	assert.Contains(t, s, "job0")
}

func TestCommandSetEnv(t *testing.T) {
	cmd := &Command{
		Cmd: exec.Command("echo", "test"),
	}
	cmd.SetEnv(map[string]string{
		"http_proxy":  "http://proxy:8080",
		"HTTPS_PROXY": "http://proxy:8443",
		"LD_PRELOAD":  "/malicious/lib.so",
		"DISPLAY":     ":0",
	})
	assert.NotNil(t, cmd.Cmd.Env)

	// SetEnv copies os.Environ() plus whitelisted keys from the map.
	// Verify whitelisted vars are set with the provided values
	var hasProxy, hasDisplay, hasHTTPSProxy bool
	for _, e := range cmd.Cmd.Env {
		if e == "http_proxy=http://proxy:8080" {
			hasProxy = true
		}
		if e == "HTTPS_PROXY=http://proxy:8443" {
			hasHTTPSProxy = true
		}
		if e == "DISPLAY=:0" {
			hasDisplay = true
		}
	}
	assert.True(t, hasProxy, "whitelisted http_proxy should be set")
	assert.True(t, hasHTTPSProxy, "whitelisted HTTPS_PROXY should be set")
	assert.True(t, hasDisplay, "whitelisted DISPLAY should be set")

	// Verify non-whitelisted LD_PRELOAD is NOT set from the map
	for _, e := range cmd.Cmd.Env {
		if e == "LD_PRELOAD=/malicious/lib.so" {
			t.Fatal("LD_PRELOAD should not be set (not in whitelist)")
		}
	}
}

func TestCommandSetEnvNil(t *testing.T) {
	cmd := &Command{
		Cmd: exec.Command("echo", "test"),
	}
	cmd.SetEnv(nil)
	// Env should remain nil (not set)
	assert.Nil(t, cmd.Cmd.Env)
}

func TestCommandAbortNotCancelable(t *testing.T) {
	c := &Command{Cancelable: false}
	assert.ErrorIs(t, c.Abort(), NotSupportError)
	assert.ErrorIs(t, c.AbortWithFailed(), NotSupportError)
}

func TestCommandAbortProcessNotStarted(t *testing.T) {
	c := &Command{
		Cancelable: true,
		Cmd:        exec.Command("sleep", "1"),
	}
	err := c.Abort()
	assert.EqualError(t, err, "the process has not yet started")

	c2 := &Command{
		Cancelable: true,
		Cmd:        exec.Command("sleep", "1"),
	}
	err = c2.AbortWithFailed()
	assert.EqualError(t, err, "the process has not yet started")
}

// newStartedSleepCommand starts a long-running child in its own process group
// so that abort() can signal it without affecting the test process itself.
func newStartedSleepCommand(t *testing.T) *Command {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return &Command{Cancelable: true, Cmd: cmd}
}

func TestCommandAbortStartedPause(t *testing.T) {
	c := newStartedSleepCommand(t)
	require.NoError(t, c.Abort())
	assert.Equal(t, ExitPause, c.ExitCode)
}

func TestCommandAbortStartedWithFailed(t *testing.T) {
	c := newStartedSleepCommand(t)
	require.NoError(t, c.AbortWithFailed())
	assert.Equal(t, ExitFailure, c.ExitCode)
}

func TestCommandAbortGetpgidError(t *testing.T) {
	cmd := exec.Command("true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())
	require.NoError(t, cmd.Wait())

	c := &Command{Cancelable: true, Cmd: cmd}
	err := c.Abort()
	assert.Error(t, err)
}
