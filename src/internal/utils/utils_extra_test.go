// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"bytes"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteData(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "subdir", "data.json")
	data := map[string]string{"key": "value"}
	err := WriteData(fp, data)
	require.NoError(t, err)

	content, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"key":"value"`)
}

func TestValidURL(t *testing.T) {
	assert.True(t, ValidURL("http://example.com"))
	assert.True(t, ValidURL("https://example.com"))
	assert.False(t, ValidURL("ftp://example.com"))
	assert.False(t, ValidURL("example.com"))
	assert.False(t, ValidURL(""))
}

func TestWriteFileSecurely(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "secure.txt")
	content := []byte("secure content")

	err := WriteFileSecurely(fp, content, 0644)
	require.NoError(t, err)

	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	info, err := os.Stat(fp)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode())
}

func TestWriteFileSecurelyNestedDir(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "a", "b", "secure.txt")
	err := WriteFileSecurely(fp, []byte("nested"), 0600)
	require.NoError(t, err)
	data, err := os.ReadFile(fp)
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestEnsureBaseDir(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "subdir", "file.txt")
	err := EnsureBaseDir(fp)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "subdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Already exists
	err = EnsureBaseDir(fp)
	assert.NoError(t, err)
}

func TestRunCommand(t *testing.T) {
	out, err := RunCommand("echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)

	_, err = RunCommand("false")
	assert.Error(t, err)
}

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

func TestTeeToFileEnsureBaseDirError(t *testing.T) {
	// parent path is an existing file, so EnsureBaseDir -> os.MkdirAll fails
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "parent-is-file")
	require.NoError(t, os.WriteFile(parentFile, []byte("x"), 0644))

	err := TeeToFile(strings.NewReader("data"), filepath.Join(parentFile, "out.txt"), func(r io.Reader) error {
		return nil
	})
	assert.Error(t, err)
}

func TestTeeToFileCreateError(t *testing.T) {
	// fpath is an existing directory, so os.Create fails
	dir := t.TempDir()
	sub := filepath.Join(dir, "outdir")
	require.NoError(t, os.Mkdir(sub, 0755))

	err := TeeToFile(strings.NewReader("data"), sub, func(r io.Reader) error {
		return nil
	})
	assert.Error(t, err)
}

func TestWriteFileSecurelyMkdirAllError(t *testing.T) {
	// parent path is an existing file, so os.MkdirAll fails
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "parent-is-file")
	require.NoError(t, os.WriteFile(parentFile, []byte("x"), 0644))

	err := WriteFileSecurely(filepath.Join(parentFile, "child", "out.txt"), []byte("data"), 0644)
	assert.Error(t, err)
}

func TestWriteFileSecurelyRenameError(t *testing.T) {
	// Target path is an existing non-empty directory, so os.Rename fails.
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(targetDir, "child"), 0755))

	err := WriteFileSecurely(targetDir, []byte("data"), 0644)
	assert.Error(t, err)
}

func TestWriteFileSecurelyCreateTempError(t *testing.T) {
	// A base name longer than NAME_MAX (255) makes os.CreateTemp fail.
	dir := t.TempDir()
	longName := strings.Repeat("a", 300)

	err := WriteFileSecurely(filepath.Join(dir, longName), []byte("data"), 0644)
	assert.Error(t, err)
}

func TestUnsetEnvNonExistent(t *testing.T) {
	err := UnsetEnv("DEFINITELY_NOT_SET_VAR_XYZ123")
	require.NoError(t, err)
	assert.Equal(t, "", os.Getenv("DEFINITELY_NOT_SET_VAR_XYZ123"))
}
