// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package fs

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileHashSha1(t *testing.T) {
	file, err := ioutil.TempFile("", "sha1_test_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	content := []byte("hello world")
	_, err = file.Write(content)
	assert.NoError(t, err)
	file.Close()

	hasher := sha1.New()
	hasher.Write(content)
	expected := hex.EncodeToString(hasher.Sum(nil))

	got, err := FileHashSha1(file.Name())
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestFileHashSha1NonExistent(t *testing.T) {
	_, err := FileHashSha1("/nonexistent/path/file.txt")
	assert.Error(t, err)
}

func TestCheckFileHashSha1Match(t *testing.T) {
	file, err := ioutil.TempFile("", "checksha1_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	content := []byte("test content")
	file.Write(content)
	file.Close()

	hasher := sha1.New()
	hasher.Write(content)
	correctHash := hex.EncodeToString(hasher.Sum(nil))

	err = CheckFileHashSha1(file.Name(), correctHash)
	assert.NoError(t, err)
}

func TestCheckFileHashSha1Mismatch(t *testing.T) {
	file, err := ioutil.TempFile("", "checksha1m_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	file.Write([]byte("test content"))
	file.Close()

	err = CheckFileHashSha1(file.Name(), "wronghash")
	assert.Error(t, err)
}

func TestCheckFileHashSha1NonExistent(t *testing.T) {
	err := CheckFileHashSha1("/nonexistent/path/file.txt", "somewidth")
	assert.Error(t, err)
}

func TestCheckFileHashSha256Match(t *testing.T) {
	file, err := ioutil.TempFile("", "checksha256_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	content := []byte("test content 256")
	file.Write(content)
	file.Close()

	hasher := sha256.New()
	hasher.Write(content)
	correctHash := hex.EncodeToString(hasher.Sum(nil))

	err = CheckFileHashSha256(file.Name(), correctHash)
	assert.NoError(t, err)
}

func TestCheckFileHashSha256Mismatch(t *testing.T) {
	file, err := ioutil.TempFile("", "checksha256m_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	file.Write([]byte("test content 256"))
	file.Close()

	err = CheckFileHashSha256(file.Name(), "wronghash")
	assert.Error(t, err)
}

func TestCheckFileHashSha256NonExistent(t *testing.T) {
	err := CheckFileHashSha256("/nonexistent/path/file.txt", "somewidth")
	assert.Error(t, err)
}

func TestCheckRepoInfoHashSha256Match(t *testing.T) {
	file, err := ioutil.TempFile("", "checkrepo_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	content := []byte("repo info content")
	file.Write(content)
	file.Close()

	hasher := sha256.New()
	hasher.Write(content)
	correctHash := hex.EncodeToString(hasher.Sum(nil))

	err = CheckRepoInfoHashSha256(file.Name(), correctHash)
	assert.NoError(t, err)
}

func TestCheckRepoInfoHashSha256Mismatch(t *testing.T) {
	file, err := ioutil.TempFile("", "checkrepom_")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	file.Write([]byte("repo info content"))
	file.Close()

	// CheckRepoInfoHashSha256 only logs a warning on mismatch, does NOT return error
	err = CheckRepoInfoHashSha256(file.Name(), "wronghash")
	assert.NoError(t, err)
}

func TestCheckRepoInfoHashSha256NonExistent(t *testing.T) {
	err := CheckRepoInfoHashSha256("/nonexistent/path/file.txt", "somewidth")
	assert.Error(t, err)
}
