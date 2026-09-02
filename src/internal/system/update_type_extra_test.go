// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateTypeBitToArray(t *testing.T) {
	tests := []struct {
		name string
		mode UpdateType
		want []UpdateType
	}{
		{"single system", SystemUpdate, []UpdateType{SystemUpdate}},
		{"single security", SecurityUpdate, []UpdateType{SecurityUpdate}},
		{"system + security", SystemUpdate | SecurityUpdate, []UpdateType{SystemUpdate, SecurityUpdate}},
		{"none", 0, nil},
		{"all install", AllInstallUpdate, []UpdateType{SystemUpdate, SecurityUpdate, UnknownUpdate}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpdateTypeBitToArray(tt.mode)
			assert.Equal(t, len(tt.want), len(got))
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				assert.True(t, found, "expected %v in result", w)
			}
		})
	}
}

func TestAllUpdateType(t *testing.T) {
	result := AllUpdateType()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, SystemUpdate)
	assert.Contains(t, result, SecurityUpdate)
	assert.Contains(t, result, AppStoreUpdate)
	assert.Contains(t, result, UnknownUpdate)
	assert.Contains(t, result, OtherSystemUpdate)
	assert.Contains(t, result, AppendUpdate)
}

func TestAllCheckUpdateType(t *testing.T) {
	result := AllCheckUpdateType()
	assert.NotEmpty(t, result)
}

func TestAllInstallUpdateType(t *testing.T) {
	result := AllInstallUpdateType()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, SystemUpdate)
	assert.Contains(t, result, SecurityUpdate)
	assert.Contains(t, result, UnknownUpdate)
}

func TestSetTempSourceDirAndClear(t *testing.T) {
	SetTempSourceDir("/tmp/test-temp-source")
	ClearTempSourceDir()
}

func TestJobTypeAllTypes(t *testing.T) {
	tests := []struct {
		ut   UpdateType
		want string
	}{
		{SystemUpdate, SystemUpgradeJobType},
		{AppStoreUpdate, AppStoreUpgradeJobType},
		{SecurityUpdate, SecurityUpgradeJobType},
		{OnlySecurityUpdate, SecurityUpgradeJobType},
		{UnknownUpdate, UnknownUpgradeJobType},
		{OtherSystemUpdate, OtherUpgradeJobType},
		{AppendUpdate, AppendUpgradeJobTye},
		{UpdateType(999), ""},
	}

	for _, tt := range tests {
		t.Run(tt.ut.JobType(), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ut.JobType())
		})
	}
}

func TestEncodeJsonAndDecodeJson(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "test.json")

	data := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	err := EncodeJson(fpath, data)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = DecodeJson(fpath, &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["name"])
}

func TestEncodeJsonError(t *testing.T) {
	err := EncodeJson("/nonexistent/path/file.json", map[string]string{"a": "b"})
	assert.Error(t, err)
}

func TestDecodeJsonError(t *testing.T) {
	var result map[string]interface{}
	err := DecodeJson("/nonexistent/file.json", &result)
	assert.Error(t, err)
}

func TestNormalFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "testfile")
	err := os.WriteFile(fpath, []byte("test"), 0644)
	assert.NoError(t, err)

	assert.True(t, NormalFileExists(fpath))
	assert.False(t, NormalFileExists(filepath.Join(tmpDir, "nonexistent")))
	assert.False(t, NormalFileExists(tmpDir))
}

func TestJobErrorTypeString(t *testing.T) {
	assert.Equal(t, "NoError", string(NoError))
	assert.Equal(t, "NoError", NoError.String())
	assert.Equal(t, "fetchFailed", string(ErrorFetchFailed))
	assert.Equal(t, "fetchFailed", ErrorFetchFailed.String())
}

func TestGetCategorySourceMap(t *testing.T) {
	m := GetCategorySourceMap()
	assert.NotEmpty(t, m)
	assert.Contains(t, m, SystemUpdate)
	assert.Contains(t, m, AppStoreUpdate)
	assert.Contains(t, m, SecurityUpdate)
	assert.Contains(t, m, UnknownUpdate)
	assert.Contains(t, m, OtherSystemUpdate)
	assert.Contains(t, m, AppendUpdate)
}

func TestSetSystemUpdate(t *testing.T) {
	SetSystemUpdate(true)
	assert.Equal(t, PlatFormSourceFile, SystemUpdateSource)

	SetSystemUpdate(false)
	assert.Equal(t, SoftLinkSystemSourceDir, SystemUpdateSource)
}

func TestRefreshSymlinksForSourceDirNoTempDir(t *testing.T) {
	ClearTempSourceDir()
	RefreshSymlinksForSourceDir("/tmp/nonexistent")
}

