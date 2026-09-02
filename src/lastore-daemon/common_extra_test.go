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

	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

func TestNormalizePackageNamesExtra(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"normal", "vim emacs", []string{"vim", "emacs"}, false},
		{"single", "vim", []string{"vim"}, false},
		{"uppercase invalid", "Vim", nil, true},
		{"starts with dash", "-vim", nil, true},
		{"empty string", "", nil, true},
		{"only spaces", "   ", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePackageNames(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"empty", "", false},
		{"normal relative", "-rw-r--r-- root/root 100 2025-01-01 00:00 ./usr/bin/test", false},
		{"absolute path", "-rw-r--r-- root/root 100 2025-01-01 00:00 /etc/passwd", true},
		{"traversal", "-rw-r--r-- root/root 100 2025-01-01 00:00 ../../../etc/shadow", true},
		{"dotdot only", "-rw-r--r-- root/root 100 2025-01-01 00:00 ..", true},
		{"short line skipped", "abc", false},
		{"blank lines", "\n\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, containsPathTraversal(tt.input))
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes float64
		want  string
	}{
		{"bytes", 500, "500B"},
		{"kb", 2048, "2.00KB"},
		{"mb", 5 * 1024 * 1024, "5.00MB"},
		{"gb", 3 * 1024 * 1024 * 1024, "3.00GB"},
		{"tb", 2 * 1024 * 1024 * 1024 * 1024, "2.00TB"},
		{"zero", 0, "0B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatSize(tt.bytes))
		})
	}
}

func TestGetContentSha256(t *testing.T) {
	assert.Equal(t,
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		getContentSha256(""))
	assert.Equal(t,
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		getContentSha256("abc"))
}

func TestGetFileSha256(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "testfile.txt")
	require.NoError(t, os.WriteFile(fp, []byte("hello world"), 0644))

	hash, err := getFileSha256(fp)
	assert.NoError(t, err)
	assert.Equal(t,
		"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		hash)

	_, err = getFileSha256("")
	assert.Error(t, err)

	_, err = getFileSha256(filepath.Join(dir, "nonexistent"))
	assert.Error(t, err)
}

func TestNewTimeRange(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tr := NewTimeRange(t1, t2)
	assert.Equal(t, t1, tr.Start)
	assert.Equal(t, t2, tr.End)

	trSwapped := NewTimeRange(t2, t1)
	assert.Equal(t, t1, trSwapped.Start)
	assert.Equal(t, t2, trSwapped.End)
}

func TestTimeRangeContains(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)

	assert.True(t, tr.Contains(start))
	assert.True(t, tr.Contains(end))
	assert.True(t, tr.Contains(start.Add(time.Hour)))
	assert.False(t, tr.Contains(start.Add(-time.Hour)))
	assert.False(t, tr.Contains(end.Add(time.Hour)))
}

func TestTimeRangeString(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := NewTimeRange(start, end)
	s := tr.String()
	assert.Contains(t, s, "~")
	assert.Contains(t, s, "2025-01-01T10:00:00Z")
	assert.Contains(t, s, "2025-01-01T12:00:00Z")
}

func TestGetFilterPackages(t *testing.T) {
	infosMap := map[string][]string{
		system.SystemUpgradeJobType: {"vim", "emacs"},
		system.SecurityUpgradeJobType: {"openssl"},
		system.UnknownUpgradeJobType:  {"unknown-pkg"},
	}

	result := getFilterPackages(infosMap, system.SystemUpdate)
	assert.Equal(t, []string{"vim", "emacs"}, result)

	result = getFilterPackages(infosMap, system.SecurityUpdate)
	assert.Equal(t, []string{"openssl"}, result)

	result = getFilterPackages(infosMap, system.SystemUpdate|system.SecurityUpdate)
	assert.Equal(t, []string{"vim", "emacs", "openssl"}, result)

	result = getFilterPackages(infosMap, system.AppStoreUpdate)
	assert.Nil(t, result)
}

func TestGetUpgradeUrls(t *testing.T) {
	dir := t.TempDir()
	listFile := filepath.Join(dir, "test.list")
	content := "deb http://example.com/debian sid main\ndeb-src https://example2.com/debian sid main\n# comment\ndeb ftp://ftp.example.com/debian stable main\n"
	require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))

	urls := getUpgradeUrls(listFile)
	assert.Len(t, urls, 2) // deb-src line should NOT match (starts with "deb-src" not "deb ")
	assert.Contains(t, urls, "http://example.com/debian")
	assert.Contains(t, urls, "ftp://ftp.example.com/debian")

	urls = getUpgradeUrls(filepath.Join(dir, "nonexistent"))
	assert.Nil(t, urls)
}

func TestGetUpgradeUrlsDir(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.list")
	f2 := filepath.Join(dir, "b.list")
	require.NoError(t, os.WriteFile(f1, []byte("deb http://a.com/debian sid main\n"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("deb http://b.com/debian sid main\n"), 0644))

	urls := getUpgradeUrls(dir)
	assert.Len(t, urls, 2)
}

func TestInitConfig(t *testing.T) {
	sc := make(UpdateSourceConfig)
	oem := config.OemRepoConfig{
		RepoShowNameZh: "测试",
		RepoShowNameEn: "test",
		RepoUrl:        []string{"deb http://oem.com/repo stable main"},
	}
	custom := []string{"deb http://custom.com/repo stable main"}
	InitConfig(sc, oem, custom)

	assert.NotNil(t, sc[config.OSDefaultRepo])
	assert.Equal(t, "测试", sc[config.OemDefaultRepo].RepoShowNameZh)
	assert.Equal(t, "test", sc[config.OemDefaultRepo].RepoShowNameEn)
	assert.Equal(t, oem.RepoUrl, sc[config.OemDefaultRepo].RepoConfig)
	assert.Equal(t, custom, sc[config.CustomRepo].RepoConfig)
}

func TestSetUsingRepoType(t *testing.T) {
	sc := make(UpdateSourceConfig)
	InitConfig(sc, config.OemRepoConfig{}, nil)

	SetUsingRepoType(sc, config.OemDefaultRepo)
	assert.True(t, sc[config.OemDefaultRepo].IsUsing)
	assert.False(t, sc[config.OSDefaultRepo].IsUsing)
	assert.False(t, sc[config.CustomRepo].IsUsing)

	SetUsingRepoType(sc, config.CustomRepo)
	assert.True(t, sc[config.CustomRepo].IsUsing)
	assert.False(t, sc[config.OemDefaultRepo].IsUsing)
}

func TestRecordUpgradeLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "upgrade_record.json")

	recordUpgradeLog("uuid-1", system.SystemUpdate, "changelog1", logPath)

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "uuid-1")
	assert.Contains(t, string(data), "changelog1")

	recordUpgradeLog("uuid-2", system.SecurityUpdate, "changelog2", logPath)
	data, err = os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "uuid-1")
	assert.Contains(t, string(data), "uuid-2")
}

func TestGetHistoryChangelog(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "changelog.txt")
	require.NoError(t, os.WriteFile(fp, []byte("some changelog content"), 0644))

	result := getHistoryChangelog(fp)
	assert.Equal(t, "some changelog content", result)

	result = getHistoryChangelog(filepath.Join(dir, "nonexistent"))
	assert.Empty(t, result)
}

func TestRecordUpgradeLogInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "upgrade_record.json")
	require.NoError(t, os.WriteFile(logPath, []byte("invalid json"), 0644))

	recordUpgradeLog("uuid-1", system.SystemUpdate, "changelog", logPath)
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "invalid json")
}
