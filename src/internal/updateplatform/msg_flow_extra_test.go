// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOSVersionContent = `[Version]
SystemName=Deepin
ProductType=Desktop
EditionName=Community
MajorVersion=25
MinorVersion=0
OsBuild=100
`

func writeTestOSVersion(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "os-version.b")
	require.NoError(t, os.WriteFile(path, []byte(testOSVersionContent), 0644))
	return path
}

func TestNeedPostSystemUpgradeMessage(t *testing.T) {
	oldCV := CacheVersion
	CacheVersion = writeTestOSVersion(t)
	t.Cleanup(func() { CacheVersion = oldCV })

	m := &UpdatePlatformManager{
		config:                            &Cfg.Config{AllowPostSystemUpgradeMessageVersion: []string{"Community"}},
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
	}
	assert.True(t, m.needPostSystemUpgradeMessage(system.SystemUpdate))

	// editionName 不匹配
	m.config.AllowPostSystemUpgradeMessageVersion = []string{"Professional"}
	assert.False(t, m.needPostSystemUpgradeMessage(system.SystemUpdate))

	// mode 不匹配
	m.config.AllowPostSystemUpgradeMessageVersion = []string{"Community"}
	assert.False(t, m.needPostSystemUpgradeMessage(system.SecurityUpdate))
}

func TestCreateJobPostMsgInfo(t *testing.T) {
	oldCV := CacheVersion
	oldDir := postContentCacheDir
	CacheVersion = writeTestOSVersion(t)
	postContentCacheDir = filepath.Join(t.TempDir(), "post_msg_cache")
	t.Cleanup(func() {
		CacheVersion = oldCV
		postContentCacheDir = oldDir
	})

	m := &UpdatePlatformManager{
		config:                            &Cfg.Config{AllowPostSystemUpgradeMessageVersion: []string{"Community"}},
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
		jobPostMsgMap:                     make(map[string]*UpgradePostMsg),
		taskID:                            42,
	}
	m.CreateJobPostMsgInfo("uuid-1", system.SystemUpdate)
	require.Contains(t, m.jobPostMsgMap, "uuid-1")
	msg := m.jobPostMsgMap["uuid-1"]
	assert.Equal(t, "uuid-1", msg.Uuid)
	assert.Equal(t, 42, msg.TaskId)
	assert.Equal(t, NotReady, msg.PostStatus)
}

func TestSaveJobPostMsgByUUID(t *testing.T) {
	oldDir := postContentCacheDir
	postContentCacheDir = filepath.Join(t.TempDir(), "post_msg_cache")
	t.Cleanup(func() { postContentCacheDir = oldDir })

	m := &UpdatePlatformManager{
		config:         &Cfg.Config{IncludeDiskInfo: false, GetHardwareIdByHelper: false},
		jobPostMsgMap:  make(map[string]*UpgradePostMsg),
		targetVersion:  "v2",
		targetBaseline: "b2",
		preBaseline:    "b1",
		preBuild:       "v1",
	}
	m.jobPostMsgMap["uuid-1"] = &UpgradePostMsg{Uuid: "uuid-1"}
	m.SaveJobPostMsgByUUID("uuid-1", UpgradeFailed, "some error")

	msg := m.jobPostMsgMap["uuid-1"]
	assert.Equal(t, UpgradeFailed, msg.UpgradeStatus)
	assert.Equal(t, "some error", msg.UpgradeErrorMsg)
	assert.Equal(t, "b1", msg.PreBaseline)
	assert.Equal(t, "b2", msg.NextBaseline)
	assert.Equal(t, "v1", msg.PreBuild)
	assert.Equal(t, "v2", msg.NextShowVersion)
	assert.Equal(t, WaitPost, msg.PostStatus)
}

func TestGenRepositoryFromPlatform(t *testing.T) {
	oldPF := system.PlatFormSourceFile
	system.PlatFormSourceFile = filepath.Join(t.TempDir(), "platform.list")
	t.Cleanup(func() { system.PlatFormSourceFile = oldPF })

	m := &UpdatePlatformManager{
		config: &Cfg.Config{PlatformRepoComponents: "main", IntranetUpdate: true},
		repoInfos: []repoInfo{
			{Uri: "https://example.com/repo", CodeName: "deepin", Source: "deb https://example.com/repo deepin main"},
		},
	}
	m.genRepositoryFromPlatform(false)

	content, err := os.ReadFile(system.PlatFormSourceFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "deb https://example.com/repo deepin main")
}

func TestGenRepositoryFromPlatformDelivery(t *testing.T) {
	oldPF := system.PlatFormSourceFile
	system.PlatFormSourceFile = filepath.Join(t.TempDir(), "platform.list")
	t.Cleanup(func() { system.PlatFormSourceFile = oldPF })

	m := &UpdatePlatformManager{
		config: &Cfg.Config{PlatformRepoComponents: "main", IntranetUpdate: false},
		repoInfos: []repoInfo{
			{Uri: "http://example.com/repo", CodeName: "deepin"},
		},
	}
	m.genRepositoryFromPlatform(true)

	content, err := os.ReadFile(system.PlatFormSourceFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "delivery://example.com/repo")
}

func TestPrepareCheckScripts(t *testing.T) {
	oldBase := check.CheckBaseDir
	check.CheckBaseDir = filepath.Join(t.TempDir(), "check") + "/"
	t.Cleanup(func() { check.CheckBaseDir = oldBase })

	shell := base64.RawStdEncoding.EncodeToString([]byte("#!/bin/sh\necho hello\n"))
	m := &UpdatePlatformManager{
		PreUpgradeCheck: []ShellCheck{{Name: "test.sh", Shell: shell}},
	}
	m.PrepareCheckScripts()

	written, err := os.ReadFile(filepath.Join(check.CheckBaseDir, "pre_upgrade_check", "test.sh"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho hello\n", string(written))
}

func TestRetryPostHistory(t *testing.T) {
	m := &UpdatePlatformManager{jobPostMsgMap: make(map[string]*UpgradePostMsg)}
	m.RetryPostHistory() // 空 map,遍历无动作
}
