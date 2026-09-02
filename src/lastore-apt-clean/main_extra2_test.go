// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArchivesInfo(t *testing.T) {
	ai := newArchivesInfo("/tmp/test")
	assert.Equal(t, "/tmp/test", ai.dir)
	assert.Nil(t, ai.Files)
	assert.Equal(t, uint64(0), ai.TotalSize)
}

func TestArchivesInfoAddFileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test content"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	ai := newArchivesInfo(tmpDir)
	ai.addFileInfo(info)

	require.Len(t, ai.Files, 1)
	assert.Equal(t, "test.deb", ai.Files[0].Name)
	assert.Equal(t, info.Size(), ai.Files[0].Size)
	assert.Equal(t, uint64(info.Size()), ai.TotalSize)
}

func TestDeleteDeb(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "to-delete.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))
	require.FileExists(t, tmpFile)

	deleteDeb(tmpFile)
	assert.NoFileExists(t, tmpFile)
}

func TestDeleteDebNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nope.deb")
	deleteDeb(nonExistent)
}

func TestGetChangeTime(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	ct := getChangeTime(info)
	assert.WithinDuration(t, time.Now(), ct, 10*time.Second)
}

func TestActWithPolicyDeleteImmediately(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	origForce := options.forceDelete
	defer func() { options.forceDelete = origForce }()
	options.forceDelete = false

	actWithPolicy(DeleteImmediately, info, tmpFile, nil)
	assert.NoFileExists(t, tmpFile)
}

func TestActWithPolicyDeleteImmediatelyWithArchivesInfo(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	ai := newArchivesInfo(tmpDir)
	actWithPolicy(DeleteImmediately, info, tmpFile, ai)
	assert.FileExists(t, tmpFile)
	require.Len(t, ai.Files, 1)
}

func TestActWithPolicyKeepNoForce(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	origForce := options.forceDelete
	defer func() { options.forceDelete = origForce }()
	options.forceDelete = false

	actWithPolicy(Keep, info, tmpFile, nil)
	assert.FileExists(t, tmpFile)
}

func TestActWithPolicyKeepWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	origForce := options.forceDelete
	defer func() { options.forceDelete = origForce }()
	options.forceDelete = true

	actWithPolicy(Keep, info, tmpFile, nil)
	assert.NoFileExists(t, tmpFile)
}

func TestActWithPolicyDeleteExpiredForce(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	origForce := options.forceDelete
	defer func() { options.forceDelete = origForce }()
	options.forceDelete = true

	actWithPolicy(DeleteExpired, info, tmpFile, nil)
	assert.NoFileExists(t, tmpFile)
}

func TestActWithPolicyDeleteExpiredRecent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.deb")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	origForce := options.forceDelete
	defer func() { options.forceDelete = origForce }()
	options.forceDelete = false

	actWithPolicy(DeleteExpired, info, tmpFile, nil)
	assert.FileExists(t, tmpFile)
}

func TestGetControlFieldNoMatch(t *testing.T) {
	line := []byte("Version: 1.0")
	key := []byte("Package: ")
	_, err := getControlField(line, key)
	assert.Error(t, err)
}

func TestDebInfoPkgArchAll(t *testing.T) {
	di := &debInfo{pkg: "testpkg", arch: "all"}
	assert.Equal(t, "testpkg:all", di.pkgArch())
}

func TestCompareVersionsGtWithEpoch(t *testing.T) {
	assert.True(t, compareVersionsGt("1:1.0", "0:9.0"))
	assert.False(t, compareVersionsGt("1.0", "1:0.1"))
}

func TestShouldDeleteNotInstalled(t *testing.T) {
	cache := map[string]statusVersion{}
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	policy, testAgain := shouldDelete(di, cache)
	assert.Equal(t, DeletePolicy(DeleteExpired), policy)
	assert.True(t, testAgain)
}

func TestShouldDeleteInstalledNewer(t *testing.T) {
	cache := map[string]statusVersion{
		"testpkg:amd64": {status: "ii ", version: "1.0"},
	}
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "2.0"}
	policy, testAgain := shouldDelete(di, cache)
	assert.Equal(t, DeletePolicy(DeleteExpired), policy)
	assert.True(t, testAgain)
}

func TestShouldDeleteInstalledSameOrOlder(t *testing.T) {
	cache := map[string]statusVersion{
		"testpkg:amd64": {status: "ii ", version: "2.0"},
	}
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	policy, testAgain := shouldDelete(di, cache)
	assert.Equal(t, DeletePolicy(DeleteImmediately), policy)
	assert.False(t, testAgain)
}

func TestShouldDeleteRemoved(t *testing.T) {
	cache := map[string]statusVersion{
		"testpkg:amd64": {status: "rc ", version: "1.0"},
	}
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	policy, testAgain := shouldDelete(di, cache)
	assert.Equal(t, DeletePolicy(DeleteImmediately), policy)
	assert.False(t, testAgain)
}

func TestShouldDeleteUnknownStatus(t *testing.T) {
	cache := map[string]statusVersion{
		"testpkg:amd64": {status: "u ", version: "1.0"},
	}
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	policy, testAgain := shouldDelete(di, cache)
	assert.Equal(t, DeletePolicy(DeleteExpired), policy)
	assert.False(t, testAgain)
}

func TestShouldDeleteEmptyStatus(t *testing.T) {
	cache := map[string]statusVersion{
		"testpkg:amd64": {status: "", version: "1.0"},
	}
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	policy, testAgain := shouldDelete(di, cache)
	assert.Equal(t, DeletePolicy(DeleteExpired), policy)
	assert.False(t, testAgain)
}

func TestGetCandidateVersionArchAll(t *testing.T) {
	_candidateCache["testpkg"] = "1.0"
	defer delete(_candidateCache, "testpkg")

	di := &debInfo{pkg: "testpkg", arch: "all"}
	assert.Equal(t, "1.0", getCandidateVersion(di))
}

func TestGetCandidateVersionWithArch(t *testing.T) {
	_candidateCache["testpkg:amd64"] = "2.0"
	_candidateCache["testpkg"] = "1.0"
	defer delete(_candidateCache, "testpkg:amd64")
	defer delete(_candidateCache, "testpkg")

	di := &debInfo{pkg: "testpkg", arch: "amd64"}
	assert.Equal(t, "2.0", getCandidateVersion(di))
}

func TestGetCandidateVersionFallbackToPkgName(t *testing.T) {
	_candidateCache["testpkg"] = "1.0"
	defer delete(_candidateCache, "testpkg")

	di := &debInfo{pkg: "testpkg", arch: "amd64"}
	assert.Equal(t, "1.0", getCandidateVersion(di))
}

func TestGetCandidateVersionNotFound(t *testing.T) {
	di := &debInfo{pkg: "notfound", arch: "amd64"}
	assert.Equal(t, "", getCandidateVersion(di))
}

func TestShouldDeleteTestAgainNoCandidate(t *testing.T) {
	_candidateCache = make(map[string]string)
	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	assert.Equal(t, DeletePolicy(DeleteExpired), shouldDeleteTestAgain(di))
}

func TestShouldDeleteTestAgainVersionMismatch(t *testing.T) {
	_candidateCache["testpkg:amd64"] = "2.0"
	defer delete(_candidateCache, "testpkg:amd64")

	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	assert.Equal(t, DeletePolicy(DeleteImmediately), shouldDeleteTestAgain(di))
}

func TestShouldDeleteTestAgainVersionMatch(t *testing.T) {
	_candidateCache["testpkg:amd64"] = "1.0"
	defer delete(_candidateCache, "testpkg:amd64")

	di := &debInfo{pkg: "testpkg", arch: "amd64", version: "1.0"}
	assert.Equal(t, DeletePolicy(Keep), shouldDeleteTestAgain(di))
}
