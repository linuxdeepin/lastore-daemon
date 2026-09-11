// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationInfosExtra(t *testing.T) {
	// applications.json typically doesn't exist at /var/lib/lastore/applications.json
	// Should return empty map without panic
	result := applicationInfos()
	assert.NotNil(t, result, "applicationInfos should return non-nil map")
}

func TestPackageIconInfos(t *testing.T) {
	// package_icon.json typically doesn't exist
	// Should return empty map without panic
	result := packageIconInfos()
	assert.NotNil(t, result, "packageIconInfos should return non-nil map")
}

func TestGetArchiveInfoNonExistentBinary(t *testing.T) {
	// /usr/bin/lastore-apt-clean may not be installed
	_, err := getArchiveInfo()
	// Just verify it doesn't panic
	_ = err
}

func TestGetNeedCleanCacheSize(t *testing.T) {
	_, err := getNeedCleanCacheSize()
	// Just verify it doesn't panic
	_ = err
}

func TestListPackageDesktopFilesBash(t *testing.T) {
	// bash is installed; listPackageDesktopFiles should return desktop files if any
	result := listPackageDesktopFiles("bash")
	// bash may or may not have desktop files; just verify it doesn't panic
	_ = result
}

func TestListPackageDesktopFilesNonExistent(t *testing.T) {
	result := listPackageDesktopFiles("nonexistent-pkg-xyz123")
	assert.Nil(t, result, "listPackageDesktopFiles should return nil for non-existent package")
}

func TestContainsPathTraversalClean(t *testing.T) {
	result := containsPathTraversal("Unpacking foo (1.0) over (2.0) ...\nSetting up foo (1.0) ...\n")
	assert.False(t, result)
}

func TestContainsPathTraversalEmpty(t *testing.T) {
	result := containsPathTraversal("")
	assert.False(t, result)
}

func TestContainsPathTraversalAbsolute(t *testing.T) {
	output := "drwxr-xr-x root/root         0 2024-01-01 12:00 /usr/bin/foo"
	result := containsPathTraversal(output)
	assert.True(t, result)
}

func TestContainsPathTraversalTraversal(t *testing.T) {
	output := "some line with enough fields and ../../../etc/passwd"
	result := containsPathTraversal(output)
	assert.True(t, result)
}

func TestFormatSize(t *testing.T) {
	assert.Equal(t, "0B", formatSize(0))
	assert.Equal(t, "1023B", formatSize(1023))
	assert.Equal(t, "1.00KB", formatSize(1024))
	assert.Equal(t, "1.00MB", formatSize(1024*1024))
	assert.Equal(t, "1.00GB", formatSize(1024*1024*1024))
}

func TestFormatSizeTB(t *testing.T) {
	assert.Equal(t, "1.00TB", formatSize(1024*1024*1024*1024))
}

func TestGetContentSha256(t *testing.T) {
	result := getContentSha256("hello")
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", result)
}

func TestGetContentSha256Empty(t *testing.T) {
	result := getContentSha256("")
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", result)
}

func TestGetFileSha256Valid(t *testing.T) {
	tmpDir := t.TempDir()
	fp := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(fp, []byte("hello"), 0644))
	result, err := getFileSha256(fp)
	assert.NoError(t, err)
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", result)
}

func TestGetFileSha256EmptyPath(t *testing.T) {
	_, err := getFileSha256("")
	assert.Error(t, err)
}

func TestGetFileSha256NonExistent(t *testing.T) {
	_, err := getFileSha256("/nonexistent/path/file.txt")
	assert.Error(t, err)
}

func TestNewTimeRangeNormal(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	assert.Equal(t, start, tr.Start)
	assert.Equal(t, end, tr.End)
}

func TestNewTimeRangeSwapped(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	assert.Equal(t, end, tr.Start)
	assert.Equal(t, start, tr.End)
}

func TestTimeRangeContainsInside(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	mid := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	assert.True(t, tr.Contains(mid))
}

func TestTimeRangeContainsBoundary(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	assert.True(t, tr.Contains(start))
	assert.True(t, tr.Contains(end))
}

func TestTimeRangeContainsOutside(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	before := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	after := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)
	assert.False(t, tr.Contains(before))
	assert.False(t, tr.Contains(after))
}

func TestTimeRangeString(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	s := tr.String()
	assert.Contains(t, s, "~")
	assert.Contains(t, s, "2024-01-01T10:00:00Z")
	assert.Contains(t, s, "2024-01-01T12:00:00Z")
}

func TestGetFilterPackages_SingleType(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType: {"pkg1", "pkg2"},
	}
	result := getFilterPackages(infosMap, system.SystemUpdate)
	assert.Equal(t, []string{"pkg1", "pkg2"}, result)
}

func TestGetFilterPackages_MultipleTypes(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType:   {"pkg1"},
		system.SecurityUpgradeJobType: {"pkg2", "pkg3"},
	}
	result := getFilterPackages(infosMap, system.SystemUpdate|system.SecurityUpdate)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2", "pkg3"}, result)
}

func TestGetFilterPackages_NoMatch(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType: {"pkg1"},
	}
	result := getFilterPackages(infosMap, system.SecurityUpdate)
	assert.Empty(t, result)
}

func TestGetFilterPackages_EmptyMap(t *testing.T) {
	result := getFilterPackages(map[string][]string{}, system.SystemUpdate)
	assert.Empty(t, result)
}

func TestGetFilterPackages_AllInstallUpdate(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType:   {"pkg1"},
		system.SecurityUpgradeJobType: {"pkg2"},
		system.UnknownUpgradeJobType:  {"pkg3"},
	}
	result := getFilterPackages(infosMap, system.AllInstallUpdate)
	assert.ElementsMatch(t, []string{"pkg1", "pkg2", "pkg3"}, result)
}

func TestInitConfig_PopulatesAllRepoTypes(t *testing.T) {
	sc := UpdateSourceConfig{}
	oemCfg := config.OemRepoConfig{
		RepoShowNameZh: "测试仓库",
		RepoShowNameEn: "Test Repo",
		RepoUrl:        []string{"deb http://example.com/repo stable main"},
	}
	customRepo := []string{"deb http://custom.com/repo stable main"}

	InitConfig(sc, oemCfg, customRepo)

	assert.NotNil(t, sc[config.OSDefaultRepo])
	assert.NotNil(t, sc[config.OemDefaultRepo])
	assert.NotNil(t, sc[config.CustomRepo])

	oemInfo := sc[config.OemDefaultRepo]
	assert.Equal(t, "测试仓库", oemInfo.RepoShowNameZh)
	assert.Equal(t, "Test Repo", oemInfo.RepoShowNameEn)
	assert.Equal(t, []string{"deb http://example.com/repo stable main"}, oemInfo.RepoConfig)

	customInfo := sc[config.CustomRepo]
	assert.Equal(t, []string{"deb http://custom.com/repo stable main"}, customInfo.RepoConfig)

	osInfo := sc[config.OSDefaultRepo]
	assert.Nil(t, osInfo.RepoConfig)
}

func TestSetUsingRepoType_SetsCorrectFlag(t *testing.T) {
	sc := UpdateSourceConfig{
		config.OSDefaultRepo:  &RepoInfo{IsUsing: true},
		config.OemDefaultRepo: &RepoInfo{IsUsing: false},
		config.CustomRepo:     &RepoInfo{IsUsing: false},
	}

	SetUsingRepoType(sc, config.OemDefaultRepo)

	assert.False(t, sc[config.OSDefaultRepo].IsUsing)
	assert.True(t, sc[config.OemDefaultRepo].IsUsing)
	assert.False(t, sc[config.CustomRepo].IsUsing)
}

func TestSetUsingRepoType_AllFalseExceptTarget(t *testing.T) {
	sc := UpdateSourceConfig{
		config.OSDefaultRepo:  &RepoInfo{IsUsing: true},
		config.OemDefaultRepo: &RepoInfo{IsUsing: true},
		config.CustomRepo:     &RepoInfo{IsUsing: true},
	}

	SetUsingRepoType(sc, config.CustomRepo)

	assert.False(t, sc[config.OSDefaultRepo].IsUsing)
	assert.False(t, sc[config.OemDefaultRepo].IsUsing)
	assert.True(t, sc[config.CustomRepo].IsUsing)
}

func TestCheckSupportDpkgScriptIgnore(t *testing.T) {
	// Value depends on the host dpkg; only assert a bool is returned without panic.
	result := checkSupportDpkgScriptIgnore()
	assert.IsType(t, false, result)
}

func TestRecordUpgradeLogPrepends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade_record.json")

	// First record writes a one-element array.
	recordUpgradeLog("uuid-1", system.SystemUpdate, map[string]string{"a": "b"}, path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var records []recordInfo
	require.NoError(t, json.Unmarshal(data, &records))
	require.Len(t, records, 1)
	assert.Equal(t, "uuid-1", records[0].UUID)
	assert.Equal(t, system.SystemUpdate, records[0].UpgradeMode)

	// Second record is PREPENDED to the existing array.
	recordUpgradeLog("uuid-2", system.SecurityUpdate, nil, path)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &records))
	require.Len(t, records, 2)
	assert.Equal(t, "uuid-2", records[0].UUID)
	assert.Equal(t, "uuid-1", records[1].UUID)
}

func TestRecordUpgradeLogInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade_record.json")
	require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0644))

	// Malformed existing content must not be overwritten.
	recordUpgradeLog("uuid-1", system.SystemUpdate, nil, path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "{not-json", string(data))
}

func TestGetHistoryChangelog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changelog.txt")
	require.NoError(t, os.WriteFile(path, []byte("changelog content"), 0644))
	assert.Equal(t, "changelog content", getHistoryChangelog(path))

	assert.Equal(t, "", getHistoryChangelog(filepath.Join(t.TempDir(), "nope.txt")))
}

func TestGetCoreListFromCacheValid(t *testing.T) {
	orig := coreListVarPath
	coreListVarPath = filepath.Join(t.TempDir(), "corelist")
	t.Cleanup(func() { coreListVarPath = orig })

	data, err := json.Marshal(PackageList{PkgList: []Package{{PkgName: "pkg-a"}, {PkgName: "pkg-b"}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(coreListVarPath, data, 0644))

	assert.Equal(t, []string{"pkg-a", "pkg-b"}, getCoreListFromCache())
}

func TestGetCoreListFromCacheMalformed(t *testing.T) {
	orig := coreListVarPath
	coreListVarPath = filepath.Join(t.TempDir(), "corelist")
	t.Cleanup(func() { coreListVarPath = orig })
	require.NoError(t, os.WriteFile(coreListVarPath, []byte("{bad"), 0644))

	assert.Nil(t, getCoreListFromCache())
}

func TestCleanAllCacheWarning(t *testing.T) {
	orig := aptGetBin
	aptGetBin = filepath.Join(t.TempDir(), "no-such-apt-get")
	t.Cleanup(func() { aptGetBin = orig })

	// apt-get missing -> logs a warning; must not panic or touch the real cache.
	cleanAllCache()
}
