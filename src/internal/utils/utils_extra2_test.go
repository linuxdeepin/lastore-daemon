// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeeToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "sub", "output.bin")
	input := []byte("hello tee test")

	var handlerRead bytes.Buffer
	err := TeeToFile(bytes.NewReader(input), outFile, func(r io.Reader) error {
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		handlerRead.Write(buf[:n])
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, input, handlerRead.Bytes())

	fileData, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Equal(t, input, fileData)
}

func TestSaveKeyFileSecurely(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "config.conf")

	kf := keyfile.NewKeyFile()
	kf.SetValue("Group1", "Key1", "Value1")

	err := SaveKeyFileSecurely(outPath, kf, 0644)
	require.NoError(t, err)

	info, err := os.Stat(outPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

	kf2 := keyfile.NewKeyFile()
	require.NoError(t, kf2.LoadFromFile(outPath))
	val, err := kf2.GetString("Group1", "Key1")
	require.NoError(t, err)
	assert.Equal(t, "Value1", val)
}

func TestFilterExecOutput(t *testing.T) {
	cmd := exec.Command("echo", "hello\nworld\nhello again")
	lines, err := FilterExecOutput(cmd, 5*time.Second, func(line string) bool {
		return len(line) > 0
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(lines), 1)
}

func TestFilterExecOutputWithFilter(t *testing.T) {
	cmd := exec.Command("printf", "apple\nbanana\napple pie\ncherry")
	lines, err := FilterExecOutput(cmd, 5*time.Second, func(line string) bool {
		return len(line) > 0 && line[0] == 'a'
	})
	require.NoError(t, err)
	for _, l := range lines {
		assert.True(t, l[0] == 'a')
	}
}

func TestUnsetEnv(t *testing.T) {
	os.Setenv("TEST_UNSET_ENV_VAR", "somevalue")
	assert.Equal(t, "somevalue", os.Getenv("TEST_UNSET_ENV_VAR"))

	err := UnsetEnv("TEST_UNSET_ENV_VAR")
	require.NoError(t, err)
	assert.Equal(t, "", os.Getenv("TEST_UNSET_ENV_VAR"))
}

func TestUnsetEnvNonExistent(t *testing.T) {
	err := UnsetEnv("DEFINITELY_NOT_SET_VAR_XYZ123")
	require.NoError(t, err)
	assert.Equal(t, "", os.Getenv("DEFINITELY_NOT_SET_VAR_XYZ123"))
}
