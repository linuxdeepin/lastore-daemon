// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetControlField(t *testing.T) {
	tests := []struct {
		line    string
		key     string
		want    string
		wantErr bool
	}{
		{"Package: vim", "Package: ", "vim", false},
		{"Version: 2:8.1.1-1", "Version: ", "2:8.1.1-1", false},
		{"Architecture: amd64", "Architecture: ", "amd64", false},
		{"Wrong: field", "Package: ", "", true},
	}
	for _, tt := range tests {
		got, err := getControlField([]byte(tt.line), []byte(tt.key))
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		}
	}
}

func TestDebInfoPkgArch(t *testing.T) {
	di := &debInfo{pkg: "vim", version: "1.0", arch: "amd64"}
	assert.Equal(t, "vim:amd64", di.pkgArch())

	di2 := &debInfo{pkg: "bash", version: "5.0", arch: "all"}
	assert.Equal(t, "bash:all", di2.pkgArch())
}

func TestShouldDelete(t *testing.T) {
	tests := []struct {
		name       string
		debInfo    *debInfo
		cache      map[string]statusVersion
		wantPolicy DeletePolicy
		wantTestAg bool
	}{
		{
			name:       "not installed",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{},
			wantPolicy: DeleteExpired,
			wantTestAg: true,
		},
		{
			name:       "installed older deb",
			debInfo:    &debInfo{pkg: "vim", version: "2.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "ii", version: "1.0"}},
			wantPolicy: DeleteExpired,
			wantTestAg: true,
		},
		{
			name:       "installed same or older deb",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "ii", version: "2.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "removed",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "rc", version: "1.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "purged",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "pc", version: "1.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "held",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "hi", version: "1.0"}},
			wantPolicy: DeleteImmediately,
			wantTestAg: false,
		},
		{
			name:       "unknown status",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "ui", version: "1.0"}},
			wantPolicy: DeleteExpired,
			wantTestAg: false,
		},
		{
			name:       "empty status",
			debInfo:    &debInfo{pkg: "vim", version: "1.0", arch: "amd64"},
			cache:      map[string]statusVersion{"vim:amd64": {status: "", version: "1.0"}},
			wantPolicy: DeleteExpired,
			wantTestAg: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, testAgain := shouldDelete(tt.debInfo, tt.cache)
			assert.Equal(t, tt.wantPolicy, policy)
			assert.Equal(t, tt.wantTestAg, testAgain)
		})
	}
}

func TestShouldDeleteTestAgain(t *testing.T) {
	// Save and restore candidate cache
	origCache := _candidateCache
	defer func() { _candidateCache = origCache }()

	_candidateCache = map[string]string{
		"vim:amd64": "1.0",
		"vim":       "1.0",
	}

	// Candidate version matches
	di := &debInfo{pkg: "vim", version: "1.0", arch: "amd64"}
	assert.Equal(t, DeletePolicy(Keep), shouldDeleteTestAgain(di))

	// Candidate version different
	di2 := &debInfo{pkg: "vim", version: "2.0", arch: "amd64"}
	assert.Equal(t, DeletePolicy(DeleteImmediately), shouldDeleteTestAgain(di2))

	// No candidate version
	di3 := &debInfo{pkg: "nonexistent", version: "1.0", arch: "amd64"}
	assert.Equal(t, DeletePolicy(DeleteExpired), shouldDeleteTestAgain(di3))

	// arch all uses pkg only
	_candidateCache = map[string]string{"vim": "1.0"}
	di4 := &debInfo{pkg: "vim", version: "1.0", arch: "all"}
	assert.Equal(t, DeletePolicy(Keep), shouldDeleteTestAgain(di4))
}

func TestParseAptCachePolicyOutput(t *testing.T) {
	input := `vim:
  Installed: 2:8.1.0875-1
  Candidate: 2:8.1.0875-1
bash:
  Installed: 5.0-4
  Candidate: 5.0-5
`
	result := parseAptCachePolicyOutput(strings.NewReader(input))
	assert.Equal(t, "2:8.1.0875-1", result["vim"])
	assert.Equal(t, "5.0-5", result["bash"])
}

func TestParseAptCachePolicyOutputEmpty(t *testing.T) {
	result := parseAptCachePolicyOutput(strings.NewReader(""))
	assert.Empty(t, result)
}

func TestCompareVersionsGt(t *testing.T) {
	assert.True(t, compareVersionsGt("2.0", "1.0"))
	assert.False(t, compareVersionsGt("1.0", "2.0"))
	assert.False(t, compareVersionsGt("1.0", "1.0"))
}

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

func TestMustGetBin(t *testing.T) {
	// dpkg should exist in the test environment
	path := mustGetBin("dpkg")
	assert.NotEmpty(t, path)
}

func TestFindBins(t *testing.T) {
	findBins()
	assert.NotEmpty(t, binDpkg)
	assert.NotEmpty(t, binDpkgQuery)
	assert.NotEmpty(t, binDpkgDeb)
	assert.NotEmpty(t, binAptCache)
}

func TestCompareVersionsGtDpkg(t *testing.T) {
	findBins()
	// 2.0 > 1.0
	assert.True(t, compareVersionsGtDpkg("2.0", "1.0"))
	// 1.0 is not > 2.0
	assert.False(t, compareVersionsGtDpkg("1.0", "2.0"))
	// 1.0 is not > 1.0
	assert.False(t, compareVersionsGtDpkg("1.0", "1.0"))
}

func TestCompareVersionsGtFast_ValidVersions(t *testing.T) {
	gt, err := compareVersionsGtFast("2.0", "1.0")
	assert.NoError(t, err)
	assert.True(t, gt)
}

func TestCompareVersionsGtFast_EqualVersions(t *testing.T) {
	gt, err := compareVersionsGtFast("1.0", "1.0")
	assert.NoError(t, err)
	assert.False(t, gt)
}

func TestCompareVersionsGtFast_LessThan(t *testing.T) {
	gt, err := compareVersionsGtFast("1.0", "2.0")
	assert.NoError(t, err)
	assert.False(t, gt)
}

func TestCompareVersionsGtFast_InvalidVersion1(t *testing.T) {
	_, err := compareVersionsGtFast("invalid-version-!!!", "1.0")
	assert.Error(t, err)
}

func TestCompareVersionsGtFast_InvalidVersion2(t *testing.T) {
	_, err := compareVersionsGtFast("1.0", "invalid-version-!!!")
	assert.Error(t, err)
}

func TestCompareVersionsGt_WithFast(t *testing.T) {
	assert.True(t, compareVersionsGt("2.0", "1.0"))
	assert.False(t, compareVersionsGt("1.0", "2.0"))
	assert.False(t, compareVersionsGt("1.0", "1.0"))
}

func TestCompareVersionsGt_ComplexVersions(t *testing.T) {
	assert.True(t, compareVersionsGt("1.0.1-1", "1.0.0-1"))
	assert.True(t, compareVersionsGt("1:1.0", "1.0"))
	assert.False(t, compareVersionsGt("1.0-1", "1.0-2"))
}

func TestCompareVersionsGt_FallbackToDpkg(t *testing.T) {
	// When debVersion.Parse fails, compareVersionsGt falls back to dpkg --compare-versions
	// Just verify it doesn't panic and returns a bool
	assert.NotPanics(t, func() {
		_ = compareVersionsGt("\x00invalid", "1.0")
	})
}

func TestCompareVersionsGtFast_WithEpoch(t *testing.T) {
	gt, err := compareVersionsGtFast("2:1.0", "1:1.0")
	assert.NoError(t, err)
	assert.True(t, gt)
}

func TestCompareVersionsGtFast_WithRevision(t *testing.T) {
	gt, err := compareVersionsGtFast("1.0-2", "1.0-1")
	assert.NoError(t, err)
	assert.True(t, gt)
}

// writeFakeBin creates an executable shell script at a temp path and returns its path.
func writeFakeBin(t *testing.T, script string) string {
	t.Helper()
	fpath := filepath.Join(t.TempDir(), "fake-bin")
	if err := os.WriteFile(fpath, []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
	return fpath
}

func TestGetDebInfo(t *testing.T) {
	orig := binDpkgDeb
	defer func() { binDpkgDeb = orig }()

	t.Run("success", func(t *testing.T) {
		binDpkgDeb = writeFakeBin(t, `printf 'Package: testpkg\nVersion: 1.0\nArchitecture: amd64\n'`)
		info, err := getDebInfo("/nonexistent.deb")
		require.NoError(t, err)
		assert.Equal(t, "testpkg", info.pkg)
		assert.Equal(t, "1.0", info.version)
		assert.Equal(t, "amd64", info.arch)
	})

	t.Run("command error", func(t *testing.T) {
		binDpkgDeb = writeFakeBin(t, "exit 1")
		_, err := getDebInfo("/nonexistent.deb")
		assert.Error(t, err)
	})

	t.Run("too few lines", func(t *testing.T) {
		binDpkgDeb = writeFakeBin(t, `printf 'Package: testpkg\nVersion: 1.0'`)
		_, err := getDebInfo("/nonexistent.deb")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "len(lines) < 3")
	})

	t.Run("missing Package prefix", func(t *testing.T) {
		binDpkgDeb = writeFakeBin(t, `printf 'WrongPrefix: testpkg\nVersion: 1.0\nArchitecture: amd64\n'`)
		_, err := getDebInfo("/nonexistent.deb")
		assert.Error(t, err)
	})
}

func TestLoadPkgStatusVersion(t *testing.T) {
	orig := binDpkgQuery
	defer func() { binDpkgQuery = orig }()

	t.Run("success", func(t *testing.T) {
		binDpkgQuery = writeFakeBin(t, `printf 'pkg:arch ii 1.0\nbadline\nfoo bar\n'`)
		result, err := loadPkgStatusVersion()
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, statusVersion{status: "ii", version: "1.0"}, result["pkg:arch"])
	})

	t.Run("command error", func(t *testing.T) {
		binDpkgQuery = writeFakeBin(t, "exit 1")
		_, err := loadPkgStatusVersion()
		assert.Error(t, err)
	})
}

func TestLoadCandidateVersions(t *testing.T) {
	orig := binAptCache
	defer func() { binAptCache = orig }()
	origCache := _candidateCache
	defer func() { _candidateCache = origCache }()

	configPath := filepath.Join(t.TempDir(), "apt.conf")

	t.Run("success", func(t *testing.T) {
		binAptCache = writeFakeBin(t, `printf 'pkg1:amd64:\n  Candidate: 1.0\n'`)
		err := loadCandidateVersions([]*debInfo{{pkg: "pkg1", arch: "amd64"}}, configPath)
		require.NoError(t, err)
		assert.Equal(t, "1.0", _candidateCache["pkg1:amd64"])
	})

	t.Run("command error", func(t *testing.T) {
		binAptCache = writeFakeBin(t, "exit 1")
		err := loadCandidateVersions([]*debInfo{{pkg: "pkg1", arch: "amd64"}}, configPath)
		assert.Error(t, err)
	})
}

func TestAppendArchivesDirInfos(t *testing.T) {
	orig := _archivesDirInfos
	defer func() { _archivesDirInfos = orig }()

	t.Run("success", func(t *testing.T) {
		_archivesDirInfos = nil
		confPath := filepath.Join(t.TempDir(), "apt.conf")
		content := "Dir \"/\";\nDir::Cache \"var/cache/apt/\";\nDir::Cache::archives \"archives/\";\n"
		require.NoError(t, os.WriteFile(confPath, []byte(content), 0644))

		appendArchivesDirInfos(confPath)
		require.Len(t, _archivesDirInfos, 1)
		assert.Equal(t, filepath.Join("/", "var/cache/apt/", "archives/"), _archivesDirInfos[0].archivesDir)
		assert.Equal(t, confPath, _archivesDirInfos[0].configPath)
	})

	t.Run("empty Dir triggers error", func(t *testing.T) {
		_archivesDirInfos = nil
		confPath := filepath.Join(t.TempDir(), "apt.conf")
		// Empty Dir value makes system.GetArchivesDir return an error.
		require.NoError(t, os.WriteFile(confPath, []byte("Dir \"\";\n"), 0644))

		appendArchivesDirInfos(confPath)
		assert.Empty(t, _archivesDirInfos)
	})
}
