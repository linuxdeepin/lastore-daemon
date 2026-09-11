// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overridePathVars 将包级路径变量临时指向临时目录,并在测试结束后恢复。
func overridePathVars(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldCV := CacheVersion
	oldCB := cacheBaseline
	oldRB := realBaseline
	oldRV := realVersion
	oldCT := cacheTaskInfo
	oldCL := cveLocalInfo
	oldAA := aptAuthConfFile

	CacheVersion = filepath.Join(dir, "os-version.b")
	cacheBaseline = filepath.Join(dir, "os-baseline.b")
	realBaseline = filepath.Join(dir, "os-baseline")
	realVersion = filepath.Join(dir, "os-version")
	cacheTaskInfo = filepath.Join(dir, "os-task-info")
	cveLocalInfo = filepath.Join(dir, "cve_local_info.json")
	aptAuthConfFile = filepath.Join(dir, "uos.conf")

	t.Cleanup(func() {
		CacheVersion = oldCV
		cacheBaseline = oldCB
		realBaseline = oldRB
		realVersion = oldRV
		cacheTaskInfo = oldCT
		cveLocalInfo = oldCL
		aptAuthConfFile = oldAA
	})
}

func TestGetAptAuthConf(t *testing.T) {
	overridePathVars(t)

	// 命中 domain 的分支
	content := "machine example.com login user1 password secret1\n" +
		"machine other.com login user2 password secret2\n"
	require.NoError(t, os.WriteFile(aptAuthConfFile, []byte(content), 0644))

	user, pass := getAptAuthConf("example.com")
	assert.Equal(t, "user1", user)
	assert.Equal(t, "secret1", pass)
}

func TestGetAptAuthConfShortLine(t *testing.T) {
	overridePathVars(t)

	// 短行会被 len(line) < 6 跳过,最终返回空
	content := "short line\nmachine example.com login\n"
	require.NoError(t, os.WriteFile(aptAuthConfFile, []byte(content), 0644))

	user, pass := getAptAuthConf("example.com")
	assert.Empty(t, user)
	assert.Empty(t, pass)
}

func TestGetAptAuthConfNotExist(t *testing.T) {
	overridePathVars(t)
	// 不写文件,走 os.Open 失败分支
	user, pass := getAptAuthConf("example.com")
	assert.Empty(t, user)
	assert.Empty(t, pass)
}

func TestSaveCEVDataAndLoadLocal(t *testing.T) {
	overridePathVars(t)

	meta := CVEMeta{
		DateTime: "2026-01-01",
		Cves: []CEVInfo{
			{CveId: "CVE-2026-0001", Score: "7.5", Source: "pkg1"},
		},
	}
	saveCEVData(meta)

	data := loadLocalCVEData()
	require.NotNil(t, data)

	var got CVEMeta
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, meta, got)
}

func TestGetSystemMeta(t *testing.T) {
	m := &UpdatePlatformManager{
		TargetCorePkgs: map[string]system.PackageInfo{
			"core1": {Name: "core1", Version: "1.0", Need: "strict"},
			"core2": {Name: "core2", Version: "2.0", Need: "skipstate"},
		},
	}

	got := m.GetSystemMeta()
	assert.Len(t, got, 2)
	assert.Equal(t, system.PackageInfo{Name: "core1", Version: "1.0", Need: "strict"}, got["core1"])
	assert.Equal(t, system.PackageInfo{Name: "core2", Version: "2.0", Need: "skipstate"}, got["core2"])

	// 返回的是复制 map,修改返回结果不应影响原始数据
	got["core1"] = system.PackageInfo{Name: "mutated"}
	assert.Equal(t, "core1", m.TargetCorePkgs["core1"].Name)
}

func TestUpdateBaselineManager(t *testing.T) {
	overridePathVars(t)

	m := &UpdatePlatformManager{
		targetBaseline:         "2500",
		targetVersion:          "2510",
		systemTypeFromPlatform: "Professional",
	}
	m.UpdateBaseline()

	assert.Equal(t, "2500", getGeneralValueFromKeyFile(cacheBaseline, "Baseline"))
	assert.Equal(t, "Professional", getGeneralValueFromKeyFile(cacheBaseline, "SystemType"))
	assert.Equal(t, "2510", getGeneralValueFromKeyFile(cacheBaseline, "Version"))

	// realBaseline 被 copyFile 同步,preBaseline 应等于 realBaseline 的 Baseline
	assert.Equal(t, "2500", getGeneralValueFromKeyFile(realBaseline, "Baseline"))
	assert.Equal(t, "2500", m.preBaseline)
}

func TestUpdateBaselineCacheManager(t *testing.T) {
	overridePathVars(t)

	m := &UpdatePlatformManager{
		targetBaseline:         "2600",
		targetVersion:          "2610",
		systemTypeFromPlatform: "Professional",
	}
	m.UpdateBaselineCache()

	assert.Equal(t, "2600", getGeneralValueFromKeyFile(cacheBaseline, "Baseline"))
	assert.Equal(t, "Professional", getGeneralValueFromKeyFile(cacheBaseline, "SystemType"))
	assert.Equal(t, "2610", getGeneralValueFromKeyFile(cacheBaseline, "Version"))
}

func TestReplaceVersionCache(t *testing.T) {
	overridePathVars(t)

	// 准备 realVersion 文件并创建软链接到 CacheVersion
	require.NoError(t, os.WriteFile(realVersion, []byte("fake version content"), 0644))
	require.NoError(t, os.Symlink(realVersion, CacheVersion))

	m := &UpdatePlatformManager{}
	m.ReplaceVersionCache()

	// 软链接被移除,替换为 realVersion 的内容副本
	assert.False(t, isSymlink(CacheVersion))
	data, err := os.ReadFile(CacheVersion)
	require.NoError(t, err)
	assert.Equal(t, "fake version content", string(data))
}

func TestRecoverVersionLink(t *testing.T) {
	overridePathVars(t)

	// 先放置一个普通文件,验证会被软链接替换
	require.NoError(t, os.WriteFile(CacheVersion, []byte("old file"), 0644))
	require.NoError(t, os.WriteFile(realVersion, []byte("real version"), 0644))

	m := &UpdatePlatformManager{}
	m.RecoverVersionLink()

	assert.True(t, isSymlink(CacheVersion))
	target, err := os.Readlink(CacheVersion)
	require.NoError(t, err)
	assert.Equal(t, realVersion, target)
}

func TestSaveTaskId(t *testing.T) {
	overridePathVars(t)

	m := &UpdatePlatformManager{taskID: 42}
	m.saveTaskId()

	assert.Equal(t, 42, getTaskId())
}

func TestUpdateTokenConfigFile(t *testing.T) {
	// 仅覆盖 tokenConfigFile 路径注入,避免写 /etc/apt
	dir := t.TempDir()
	old := tokenConfigFile
	tokenConfigFile = filepath.Join(dir, "99lastore-token.conf")
	t.Cleanup(func() { tokenConfigFile = old })

	token := UpdateTokenConfigFile(false, false)
	assert.NotEmpty(t, token)

	data, err := os.ReadFile(tokenConfigFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Acquire::SmartMirrors::Token")
}

// isSymlink 判断路径是否为软链接。
func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}
