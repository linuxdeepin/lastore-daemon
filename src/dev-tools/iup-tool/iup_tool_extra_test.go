// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitUpdatePlatform(t *testing.T) {
	initUpdatePlatform()

	assert.Equal(t, extractMachineIDFromToken(updatePlatform.Token), updatePlatform.machineID)
}

func TestGetPlatformURLFromDSettings(t *testing.T) {
	// Returns "" when no D-Bus system bus is available; must not panic.
	assert.NotPanics(t, func() { _ = getPlatformURLFromDSettings() })
}

func TestRootCmdPersistentPreRun(t *testing.T) {
	origDebug := globalDebug
	defer func() { globalDebug = origDebug }()

	// debug disabled -> Info log level
	globalDebug = false
	assert.NotPanics(t, func() { rootCmd.PersistentPreRun(rootCmd, nil) })

	// debug enabled -> Debug log level
	globalDebug = true
	assert.NotPanics(t, func() { rootCmd.PersistentPreRun(rootCmd, nil) })
}

func TestMainErrorPath(t *testing.T) {
	if os.Getenv("IUP_TOOL_MAIN_HELPER") == "1" {
		os.Args = []string{"iup-tool", "__nonexistent_command__"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainErrorPath$")
	cmd.Env = append(os.Environ(), "IUP_TOOL_MAIN_HELPER=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	var exitErr *exec.ExitError
	if assert.ErrorAs(t, err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode())
	}
}
