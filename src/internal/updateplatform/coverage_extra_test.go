// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasDeliveryRepoFalse(t *testing.T) {
	m := &UpdatePlatformManager{
		repoInfos: []repoInfo{
			{Source: "deb https://packages.example.com/desktop beige main"},
			{Source: "deb http://cdn.example.com/apps beige main"},
		},
	}
	assert.False(t, m.HasDeliveryRepo())

	assert.False(t, (&UpdatePlatformManager{}).HasDeliveryRepo())
}

func TestCopyFileDstError(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	// dst parent does not exist -> os.WriteFile fails; must not panic
	dst := filepath.Join(tmpDir, "nonexistent-dir", "dst.txt")
	copyFile(src, dst)
	assert.NoFileExists(t, dst)
}

func TestUpdateKeyFileInvalidIni(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.ini")
	require.NoError(t, os.WriteFile(path, []byte("this is not a key value pair\n"), 0644))

	assert.False(t, updateKeyFile(path, "Baseline", "25.1"))
}

func TestUpdateKeyFileSaveError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent-dir", "baseline.conf")
	assert.False(t, updateKeyFile(path, "Baseline", "25.1"))
}

func TestTarFilesDirectoryInput(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "adir")
	require.NoError(t, os.Mkdir(subDir, 0755))

	outFile := filepath.Join(tmpDir, "out.tar")
	err := tarFiles([]string{subDir}, outFile)
	assert.Error(t, err)
}

func TestGetUpdateMessageSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":true,"code":0,"data":{"systemType":"desktop","version":{"version":"2.0","baseline":"25","taskID":1},"policy":{"tp":0,"data":{"updateTime":""}},"repoInfos":[],"clientPollSetting":{"checkPolicyInterval":0}}}`))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}

	msg, err := m.getUpdateMessage()
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "2.0", msg.Version.Version)

	msg2, err := m.getUpdateMessageWithRetry(0)
	require.NoError(t, err)
	require.NotNil(t, msg2)
	assert.Equal(t, "desktop", msg2.SystemType)
}

func TestGetUpdateMessageWithRetryMaxRetry(t *testing.T) {
	m := &UpdatePlatformManager{}
	_, err := m.getUpdateMessageWithRetry(4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retry count exceeded")
}

func TestGetUpdateMessageWithRetryBadURL(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	_, err := m.getUpdateMessageWithRetry(0)
	assert.Error(t, err)
}

func TestGetUpdateMessageWithRetryNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	_, err := m.getUpdateMessageWithRetry(0)
	assert.Error(t, err)
}

func TestGetUpdateMessageWithRetryDataNull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":true,"code":0,"data":null}`))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	_, err := m.getUpdateMessageWithRetry(0)
	assert.Error(t, err)
}

func TestGetUpdateMessageWithRetryCode416(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":false,"code":416,"msg":"uninstall required"}`))
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	_, err := m.getUpdateMessageWithRetry(0)
	assert.Error(t, err)
}

func TestGenUpdatePolicyByTokenError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	err := m.genUpdatePolicyByToken()
	assert.Error(t, err)
}

func TestGetProcessorInfoLoongArch(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "cpuinfo")
	require.NoError(t, os.WriteFile(tmpFile, []byte("Model Name: LoongArch 3A5000\n"), 0644))

	cpu, err := getProcessorInfo(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "LoongArch 3A5000", cpu)
}

func TestGetProcessorInfoSWCPU(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "cpuinfo")
	require.NoError(t, os.WriteFile(tmpFile, []byte("cpu: Phytium FT-2000\n"), 0644))

	cpu, err := getProcessorInfo(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "Phytium FT-2000", cpu)
}

func TestGetProcessorInfoKirin(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "cpuinfo")
	require.NoError(t, os.WriteFile(tmpFile, []byte("Hardware: Kirin 9000\n"), 0644))

	cpu, err := getProcessorInfo(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "Kirin 9000", cpu)
}

func TestGetProcessorInfoARM(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "cpuinfo")
	require.NoError(t, os.WriteFile(tmpFile, []byte("Processor: ARMv8 Processor rev 4\n"), 0644))

	cpu, err := getProcessorInfo(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "ARMv8 Processor rev 4", cpu)
}

func TestVerifyOemFile(t *testing.T) {
	t.Run("invalid pem", func(t *testing.T) {
		assert.False(t, verifyOemFile("not a pem", "whatever"))
	})

	t.Run("valid pem but not public key", func(t *testing.T) {
		invalidKey := "-----BEGIN PUBLIC KEY-----\naW52YWxpZA==\n-----END PUBLIC KEY-----"
		assert.False(t, verifyOemFile(invalidKey, "whatever"))
	})

	t.Run("valid key nonexistent file", func(t *testing.T) {
		assert.False(t, verifyOemFile(oemPubKey, filepath.Join(t.TempDir(), "nope")))
	})

	t.Run("valid key existing file missing signature", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "oem-content")
		require.NoError(t, os.WriteFile(tmpFile, []byte("some oem content"), 0644))
		assert.False(t, verifyOemFile(oemPubKey, tmpFile))
	})
}

func TestIsMajorUpgradeWithTarget(t *testing.T) {
	infoMap, err := GetOSVersionInfo(CacheVersion)
	if err != nil {
		// No version file in this environment; IsMajorUpgrade must bail out to false.
		m := &UpdatePlatformManager{targetVersion: "2600"}
		assert.False(t, m.IsMajorUpgrade())
		return
	}
	minor := infoMap["MinorVersion"]
	for _, target := range []string{"0", minor, "999999"} {
		m := &UpdatePlatformManager{targetVersion: target}
		assert.Equal(t, isMajorUpgrade(minor, target), m.IsMajorUpgrade())
	}
}

func TestUpgradePostMsgSaveWriteError(t *testing.T) {
	oldDir := postContentCacheDir
	defer func() { postContentCacheDir = oldDir }()

	// Point the cache dir at a regular file so the inner WriteFile fails.
	fileAsDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(fileAsDir, []byte("x"), 0644))
	postContentCacheDir = fileAsDir

	msg := &UpgradePostMsg{
		Uuid:       "test-save-write-error",
		PostStatus: WaitPost,
	}
	// Must not panic; the write error branch is exercised.
	assert.NotPanics(t, func() { msg.save() })
}

func TestUpdateDeliverySpeedLimitNilLimits(t *testing.T) {
	m := &UpdatePlatformManager{}
	assert.NoError(t, m.UpdateDeliverySpeedLimit())
}

func TestResetIntranetUpdateSettingsNilManager(t *testing.T) {
	var m *UpdatePlatformManager
	assert.NotPanics(t, func() { m.resetIntranetUpdateSettingsAfterUnregister() })
}

func TestResetIntranetUpdateSettingsNilConfig(t *testing.T) {
	m := &UpdatePlatformManager{}
	assert.NotPanics(t, func() { m.resetIntranetUpdateSettingsAfterUnregister() })
}
