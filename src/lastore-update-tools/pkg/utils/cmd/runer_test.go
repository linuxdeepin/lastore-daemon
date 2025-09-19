// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package runcmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecAndWait_Success(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		command string
		args    []string
		env     []string
		wantOut string
		wantErr bool
	}{
		{
			name:    "echo command",
			timeout: 5,
			command: "echo",
			args:    []string{"hello", "world"},
			env:     nil,
			wantOut: "hello world\n",
			wantErr: false,
		},
		{
			name:    "pwd command",
			timeout: 5,
			command: "pwd",
			args:    []string{},
			env:     nil,
			wantOut: "",
			wantErr: false,
		},
		{
			name:    "env variable test",
			timeout: 5,
			command: "sh",
			args:    []string{"-c", "echo $TEST_VAR"},
			env:     []string{"TEST_VAR=test_value"},
			wantOut: "test_value\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := execAndWait(tt.timeout, tt.command, tt.env, tt.args...)

			if tt.wantErr {
				assert.Error(t, err, "execAndWait() should return error")
			} else {
				assert.NoError(t, err, "execAndWait() should not return error")
			}

			switch tt.name {
			case "echo command":
				assert.Equal(t, tt.wantOut, stdout, "stdout should match expected output")
			case "env variable test":
				assert.Equal(t, tt.wantOut, stdout, "stdout should match expected output")
			case "pwd command":
				assert.Contains(t, stdout, "/", "pwd output should contain '/'")
			}

			// stderr should be empty for successful commands
			if !tt.wantErr && stderr != "" {
				t.Logf("execAndWait() stderr = %v (might be expected for some commands)", stderr)
			}
		})
	}
}

func TestExecAndWait_Timeout(t *testing.T) {
	// Test command that takes longer than timeout
	stdout, stderr, err := execAndWait(1, "sleep", nil, "3")

	assert.Error(t, err, "execAndWait() should return timeout error")
	assert.Contains(t, err.Error(), "timed out", "error message should contain 'timed out'")

	// stdout and stderr might be empty for timeout case
	t.Logf("Timeout test - stdout: %q, stderr: %q", stdout, stderr)
}

func TestExecAndWait_CommandFailure(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{
			name:    "non-existent command",
			command: "non_existent_command_12345",
			args:    []string{},
		},
		{
			name:    "invalid argument",
			command: "ls",
			args:    []string{"--invalid-option-12345"},
		},
		{
			name:    "false command",
			command: "false",
			args:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := execAndWait(5, tt.command, nil, tt.args...)

			assert.Error(t, err, "execAndWait() should return error for %s", tt.name)
			assert.Contains(t, err.Error(), "command failed", "error message should contain 'command failed'")

			t.Logf("Command failure test - stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		})
	}
}

func TestExecAndWait_EnvironmentVariables(t *testing.T) {
	// Test that environment variables are properly passed
	envVars := []string{
		"TEST_VAR1=value1",
		"TEST_VAR2=value2",
		"TEST_VAR3=value with spaces",
	}

	stdout, stderr, err := execAndWait(5, "sh", envVars, "-c", "echo \"$TEST_VAR1|$TEST_VAR2|$TEST_VAR3\"")

	assert.NoError(t, err, "execAndWait() should not return error")

	expected := "value1|value2|value with spaces\n"
	assert.Equal(t, expected, stdout, "stdout should match expected environment variable output")

	if stderr != "" {
		t.Logf("execAndWait() stderr = %v (might be expected)", stderr)
	}
}

func TestExecAndWait_StdoutStderr(t *testing.T) {
	// Test command that outputs to both stdout and stderr
	stdout, stderr, err := execAndWait(5, "sh", nil, "-c", "echo 'stdout message'; echo 'stderr message' >&2")

	assert.NoError(t, err, "execAndWait() should not return error")
	assert.Contains(t, stdout, "stdout message", "stdout should contain expected message")
	assert.Contains(t, stderr, "stderr message", "stderr should contain expected message")
}

func TestExecAndWait_NilEnvironment(t *testing.T) {
	// Test that nil environment works correctly
	stdout, stderr, err := execAndWait(5, "echo", nil, "test_nil_env")

	assert.NoError(t, err, "execAndWait() should not return error")
	assert.Equal(t, "test_nil_env\n", stdout, "stdout should match expected output")

	if stderr != "" {
		t.Logf("execAndWait() stderr = %v (might be expected)", stderr)
	}
}

func TestExecAndWait_EmptyArgs(t *testing.T) {
	// Test command with no arguments
	stdout, stderr, err := execAndWait(5, "echo", nil)

	assert.NoError(t, err, "execAndWait() should not return error")
	assert.Equal(t, "\n", stdout, "stdout should contain newline only")

	if stderr != "" {
		t.Logf("execAndWait() stderr = %v (might be expected)", stderr)
	}
}

// Test wrapper functions
func TestRunnerOutput(t *testing.T) {
	output, err := RunnerOutput(5, "echo", "test_runner_output")

	assert.NoError(t, err, "RunnerOutput() should not return error")
	assert.Equal(t, "test_runner_output\n", output, "output should match expected value")
}

func TestRunnerOutputEnv(t *testing.T) {
	env := []string{"TEST_VAR=test_value"}
	output, err := RunnerOutputEnv(5, "sh", env, "-c", "echo $TEST_VAR")

	assert.NoError(t, err, "RunnerOutputEnv() should not return error")
	assert.Equal(t, "test_value\n", output, "output should match expected environment variable value")
}

func TestRunnerNotOutput(t *testing.T) {
	err := RunnerNotOutput(5, "echo", "test_runner_not_output")

	assert.NoError(t, err, "RunnerNotOutput() should not return error")
}

// Benchmark tests
func BenchmarkExecAndWait_SimpleCommand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := execAndWait(5, "echo", nil, "benchmark_test")
		assert.NoError(b, err, "execAndWait() should not return error")
	}
}

func BenchmarkExecAndWait_WithEnvironment(b *testing.B) {
	env := []string{"BENCH_VAR=benchmark_value"}
	for i := 0; i < b.N; i++ {
		_, _, err := execAndWait(5, "sh", env, "-c", "echo $BENCH_VAR")
		assert.NoError(b, err, "execAndWait() should not return error")
	}
}

// Test context cancellation (manual test for understanding)
func TestExecAndWait_ContextBehavior(t *testing.T) {
	// This test demonstrates how context cancellation works
	start := time.Now()
	_, _, err := execAndWait(2, "sleep", nil, "5")
	duration := time.Since(start)

	assert.Error(t, err, "execAndWait() should return timeout error")
	assert.Contains(t, err.Error(), "timed out", "error should indicate timeout")

	// Should timeout around 2 seconds, not 5
	assert.Less(t, duration, 3*time.Second, "execution should timeout within expected time")

	t.Logf("Context cancellation worked correctly, duration: %v", duration)
}
