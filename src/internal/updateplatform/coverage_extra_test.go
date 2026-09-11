// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	Cfg "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
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

	t.Run("valid signature", func(t *testing.T) {
		priv, err := rsa.GenerateKey(crand.Reader, 2048)
		require.NoError(t, err)

		pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		require.NoError(t, err)
		pubPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

		content := []byte("oem content to sign")
		tmpFile := filepath.Join(t.TempDir(), "oem-info")
		require.NoError(t, os.WriteFile(tmpFile, content, 0644))

		hashed := sha256.Sum256(content)
		sig, err := rsa.SignPKCS1v15(crand.Reader, priv, crypto.SHA256, hashed[:])
		require.NoError(t, err)

		sigFile := filepath.Join(t.TempDir(), "oem-shadow")
		require.NoError(t, os.WriteFile(sigFile, sig, 0644))

		origSign := oemSignFile
		oemSignFile = sigFile
		t.Cleanup(func() { oemSignFile = origSign })

		assert.True(t, verifyOemFile(string(pubPem), tmpFile))
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

// ---- tests added to raise coverage of the named functions ----

func TestUpdateSourceListWritesRepos(t *testing.T) {
	// 仅非 root 环境下安全:写入 /etc/apt/sources.list(const,无法注入)会因权限失败,
	// 覆盖 for 循环拼接与写文件失败分支。root 环境下该写会失败于权限校验之外,故假定非 root。
	m := &UpdatePlatformManager{
		config: &Cfg.Config{IntranetUpdate: true, PlatformUpdate: true},
		repoInfos: []repoInfo{
			{Source: "deb http://example.com eagle main"},
			{Source: ""}, // 空 Source 跳过分支
		},
	}
	assert.NotPanics(t, func() { m.UpdateSourceList() })
}

func TestRecoverVersionLinkErrors(t *testing.T) {
	old := CacheVersion
	fileAsDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(fileAsDir, []byte("x"), 0644))
	CacheVersion = filepath.Join(fileAsDir, "os-version.b")
	t.Cleanup(func() { CacheVersion = old })

	m := &UpdatePlatformManager{}
	// RemoveAll 与 Symlink 均失败,覆盖两条错误分支。
	assert.NotPanics(t, func() { m.RecoverVersionLink() })
}

func TestIsUnstableSystemBusError(t *testing.T) {
	old := dbusSystemBus
	dbusSystemBus = func() (*dbus.Conn, error) { return nil, errors.New("no system bus") }
	t.Cleanup(func() { dbusSystemBus = old })

	assert.Equal(t, ReleaseVersion, isUnstable())
}

func TestGenUpdatePolicyByTokenSuccess(t *testing.T) {
	overridePathVars(t)

	const body = `{"result":true,"code":0,"data":{` +
		`"systemType":"desktop",` +
		`"version":{"version":"2600","baseline":"25","taskID":7},` +
		`"policy":{"tp":4,"data":{"updateTime":"2026-01-01T00:00:00Z"}},` +
		`"repoInfos":[{"uri":"http://packages.example.com/repo","codename":"eagle","source":"deb http://packages.example.com/repo eagle main"}],` +
		`"clientPollSetting":{"checkPolicyInterval":3600,"startCheckRange":[1,2]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
		arch:       "x86_64",
	}
	require.NoError(t, m.genUpdatePolicyByToken())
	assert.Equal(t, "2600", m.targetVersion)
	assert.Equal(t, "25", m.targetBaseline)
	assert.Equal(t, 7, m.taskID)
	assert.Equal(t, "desktop", m.systemTypeFromPlatform)
	assert.Equal(t, UpdateRegularly, m.Tp)
	assert.Len(t, m.repoInfos, 1)
	assert.NotEmpty(t, m.checkTime)
}

func TestGenUpdatePolicyByTokenNow(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion},
		Tp:             UpdateNow,
		targetBaseline: "b",
	}
	require.NoError(t, m.GenUpdatePolicyByToken())
	assert.True(t, m.UpdateNowForce)
}

func TestGenUpdatePolicyByTokenShutdown(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion},
		Tp:             UpdateShutdown,
		targetBaseline: "b",
	}
	require.NoError(t, m.GenUpdatePolicyByToken())
	assert.False(t, m.UpdateNowForce)
}

func TestGenUpdatePolicyByTokenRegularly(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion},
		Tp:             UpdateRegularly,
		UpdateTime:     time.Now().Add(-2 * time.Hour),
		targetBaseline: "b",
	}
	require.NoError(t, m.GenUpdatePolicyByToken())
	assert.False(t, m.UpdateNowForce)
}

func TestGenUpdatePolicyByTokenConfigNow(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion, UpdateTime: KeyNow},
		Tp:             UnknownUpdate,
		targetBaseline: "b",
	}
	require.NoError(t, m.GenUpdatePolicyByToken())
	assert.True(t, m.UpdateNowForce)
}

func TestGenUpdatePolicyByTokenConfigShutdown(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion, UpdateTime: KeyShutdown},
		Tp:             UnknownUpdate,
		targetBaseline: "b",
	}
	require.NoError(t, m.GenUpdatePolicyByToken())
	assert.False(t, m.UpdateNowForce)
}

func TestGenUpdatePolicyByTokenConfigParse(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{PlatformDisabled: Cfg.DisabledVersion, UpdateTime: "2026-01-01T10:00:00Z"},
		Tp:             UnknownUpdate,
		targetBaseline: "b",
	}
	require.NoError(t, m.GenUpdatePolicyByToken())
	assert.Equal(t, UpdateRegularly, m.Tp)
}

func TestUpdateTargetPkgMetaSyncWithPackages(t *testing.T) {
	const body = `{"result":true,"code":0,"data":{` +
		`"preCheck":[{"name":"pre.sh","shell":"ZWNobw=="}],` +
		`"packages":{` +
		`"core":[` +
		`{"name":"core1","version":[{"version":"1.0","arch":"x86_64"}],"need":"strict"},` +
		`{"name":"core2","version":[{"version":"2.0","arch":"arm64"}],"need":"strict"}],` +
		`"select":[{"name":"sel1","version":[{"version":"3.0","arch":"x86_64"}],"need":"skipstate"}],` +
		`"freeze":[{"name":"fr1","version":[{"version":"4.0","arch":"x86_64"}],"need":"exist"}],` +
		`"purge":[{"name":"pu1","version":[{"version":"5.0","arch":"x86_64"}],"need":"strict"}]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		targetBaseline: "test-baseline",
		Token:          "tok",
		arch:           "x86_64",
		config:         &Cfg.Config{},
		TargetCorePkgs: make(map[string]system.PackageInfo),
		SelectPkgs:     make(map[string]system.PackageInfo),
		FreezePkgs:     make(map[string]system.PackageInfo),
		PurgePkgs:      make(map[string]system.PackageInfo),
	}
	require.NoError(t, m.updateTargetPkgMetaSync())

	assert.Equal(t, "core1", m.TargetCorePkgs["core1"].Name)
	assert.Equal(t, "1.0", m.TargetCorePkgs["core1"].Version)
	assert.NotContains(t, m.TargetCorePkgs, "core2")
	assert.Equal(t, "sel1", m.SelectPkgs["sel1"].Name)
	assert.Equal(t, "fr1", m.FreezePkgs["fr1"].Name)
	assert.Equal(t, "pu1", m.PurgePkgs["pu1"].Name)
	assert.Len(t, m.PreUpgradeCheck, 1)
}

func TestUpdateCurrentPreInstalledPkgMetaSyncWithPackages(t *testing.T) {
	const body = `{"result":true,"code":0,"data":{"packages":{` +
		`"core":[{"name":"core1","version":[{"version":"1.0","arch":"x86_64"}],"need":"strict"}]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:   srv.URL,
		preBaseline:  "base",
		Token:        "tok",
		arch:         "x86_64",
		config:       &Cfg.Config{},
		BaselinePkgs: make(map[string]system.PackageInfo),
	}
	require.NoError(t, m.updateCurrentPreInstalledPkgMetaSync())
	assert.Equal(t, "core1", m.BaselinePkgs["core1"].Name)
	assert.Equal(t, "1.0", m.BaselinePkgs["core1"].Version)
}

func TestSaveCEVDataWriteError(t *testing.T) {
	old := cveLocalInfo
	fileAsDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(fileAsDir, []byte("x"), 0644))
	cveLocalInfo = filepath.Join(fileAsDir, "cve.json")
	t.Cleanup(func() { cveLocalInfo = old })

	assert.NotPanics(t, func() { saveCEVData(CVEMeta{DateTime: "2026-01-01"}) })
}

func TestUpdateLogMetaSyncGenError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	require.NoError(t, m.updateLogMetaSync())
	require.Len(t, m.SystemUpdateLogs, 1)
	assert.Contains(t, m.SystemUpdateLogs[0].EnLog, "Fixing")
}

func TestUpdateLogMetaSyncNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	require.NoError(t, m.updateLogMetaSync())
	require.Len(t, m.SystemUpdateLogs, 1)
	assert.Contains(t, m.SystemUpdateLogs[0].EnLog, "Fixing")
}

func TestUpdateLogMetaSyncInvalidLog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"result":true,"code":0,"data":[{"enLog":""}]}`)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl: srv.URL,
		Token:      "tok",
		config:     &Cfg.Config{},
	}
	require.NoError(t, m.updateLogMetaSync())
	require.Len(t, m.SystemUpdateLogs, 1)
	assert.Contains(t, m.SystemUpdateLogs[0].EnLog, "Fixing")
}

func TestCheckInReleaseFromPlatformNonOK(t *testing.T) {
	overridePathVars(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: srv.URL, CodeName: "eagle"}},
	}
	assert.NotPanics(t, func() { m.checkInReleaseFromPlatform() })
}

func TestCheckInReleaseFromPlatformDoError(t *testing.T) {
	overridePathVars(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 连接拒绝,覆盖 client.Do 失败分支

	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: url, CodeName: "eagle"}},
	}
	assert.NotPanics(t, func() { m.checkInReleaseFromPlatform() })
}

func TestCheckInReleaseFromPlatformRedirectLoop(t *testing.T) {
	overridePathVars(t)
	// 循环重定向触发 CheckRedirect 回调,覆盖 via 计数与超限返回分支。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: srv.URL, CodeName: "eagle"}},
	}
	assert.NotPanics(t, func() { m.checkInReleaseFromPlatform() })
}

func TestCheckInReleaseFromPlatformBadURI(t *testing.T) {
	overridePathVars(t)
	// 非法端口号使 http.NewRequest 失败,但 URIToPath 仍返回非空,避免删除真实 apt lists。
	m := &UpdatePlatformManager{
		config:    &Cfg.Config{},
		repoInfos: []repoInfo{{Uri: "http://[::1]:namedport", CodeName: "eagle"}},
	}
	assert.NotPanics(t, func() { m.checkInReleaseFromPlatform() })
}

func TestPostStatusMessageDisabled(t *testing.T) {
	m := &UpdatePlatformManager{config: &Cfg.Config{PlatformDisabled: Cfg.DisabledProcess}}
	assert.NotPanics(t, func() { m.PostStatusMessage(StatusMessage{}, false) })
}

func TestPostStatusMessageSkipUpload(t *testing.T) {
	m := &UpdatePlatformManager{
		config:         &Cfg.Config{UpdateProcessUpload: false, IntranetUpdate: true},
		targetBaseline: "b",
	}
	assert.NotPanics(t, func() { m.PostStatusMessage(StatusMessage{}, false) })
}

func TestPostStatusMessageEmptyBaseline(t *testing.T) {
	m := &UpdatePlatformManager{
		config: &Cfg.Config{UpdateProcessUpload: true},
	}
	assert.NotPanics(t, func() { m.PostStatusMessage(StatusMessage{}, false) })
}

func TestPostStatusMessageGenError(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl:     "http://127.0.0.1:1",
		Token:          "tok",
		config:         &Cfg.Config{UpdateProcessUpload: true},
		targetBaseline: "b",
	}
	assert.NotPanics(t, func() { m.PostStatusMessage(StatusMessage{Type: "info"}, false) })
}

func TestPostStatusMessageNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		Token:          "tok",
		config:         &Cfg.Config{UpdateProcessUpload: true},
		targetBaseline: "b",
	}
	assert.NotPanics(t, func() { m.PostStatusMessage(StatusMessage{Type: "info"}, false) })
}

func TestPostUpdateLogFilesDisabled(t *testing.T) {
	m := &UpdatePlatformManager{config: &Cfg.Config{PlatformDisabled: Cfg.DisabledProcess}}
	assert.NotPanics(t, func() { m.PostUpdateLogFiles(nil) })
}

func TestPostUpdateLogFilesTarError(t *testing.T) {
	m := &UpdatePlatformManager{config: &Cfg.Config{}}
	assert.NotPanics(t, func() {
		m.PostUpdateLogFiles([]string{filepath.Join(t.TempDir(), "nope.log")})
	})
}

func TestPostUpdateLogFilesGenError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.log")
	require.NoError(t, os.WriteFile(f, []byte("log a"), 0644))

	m := &UpdatePlatformManager{
		requestUrl:     "http://127.0.0.1:1",
		Token:          "tok",
		config:         &Cfg.Config{},
		preBaseline:    "p",
		targetBaseline: "b",
	}
	assert.NotPanics(t, func() { m.PostUpdateLogFiles([]string{f}) })
}

func TestPostUpdateLogFilesNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := filepath.Join(t.TempDir(), "a.log")
	require.NoError(t, os.WriteFile(f, []byte("log a"), 0644))

	m := &UpdatePlatformManager{
		requestUrl:     srv.URL,
		Token:          "tok",
		config:         &Cfg.Config{},
		preBaseline:    "p",
		targetBaseline: "b",
	}
	assert.NotPanics(t, func() { m.PostUpdateLogFiles([]string{f}) })
}

func TestRetryPostHistoryWithEntries(t *testing.T) {
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		config:     &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"u1": {Uuid: "u1", PostStatus: WaitPost},
			"u2": {Uuid: "u2", PostStatus: PostFailure},
			"u3": {Uuid: "u3", PostStatus: PostSuccess}, // 跳过
		},
	}
	assert.NotPanics(t, func() { m.RetryPostHistory() })
}

func TestPostUpgradeStatusWithInhibit(t *testing.T) {
	var inhibitCalls int32
	m := &UpdatePlatformManager{
		requestUrl: "http://127.0.0.1:1",
		Token:      "tok",
		config:     &Cfg.Config{},
		jobPostMsgMap: map[string]*UpgradePostMsg{
			"uuid-1": {Uuid: "uuid-1", PostStatus: WaitPost},
		},
		inhibitAutoQuit:   func() { atomic.AddInt32(&inhibitCalls, 1) },
		UnInhibitAutoQuit: func() { atomic.AddInt32(&inhibitCalls, 1) },
	}
	m.PostUpgradeStatus("uuid-1", UpgradeSucceed, "")

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&inhibitCalls) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	// inhibitAutoQuit 在 goroutine 中被直接调用一次并 defer 一次。
	assert.Equal(t, int32(2), atomic.LoadInt32(&inhibitCalls))
}

func TestSaveTaskIdExistingFile(t *testing.T) {
	overridePathVars(t)

	(&UpdatePlatformManager{taskID: 1}).saveTaskId()  // 首次创建文件
	(&UpdatePlatformManager{taskID: 99}).saveTaskId() // 文件已存在,走 LoadFromFile 分支
	assert.Equal(t, 99, getTaskId())
}

func TestSaveTaskIdLoadError(t *testing.T) {
	overridePathVars(t)
	require.NoError(t, os.WriteFile(cacheTaskInfo, []byte("not a valid keyfile\n"), 0644))

	(&UpdatePlatformManager{taskID: 99}).saveTaskId() // LoadFromFile 失败,提前返回
	assert.Equal(t, 0, getTaskId())
}

func TestSaveTaskIdSaveError(t *testing.T) {
	old := cacheTaskInfo
	fileAsDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(fileAsDir, []byte("x"), 0644))
	cacheTaskInfo = filepath.Join(fileAsDir, "task")
	t.Cleanup(func() { cacheTaskInfo = old })

	assert.NotPanics(t, func() { (&UpdatePlatformManager{taskID: 99}).saveTaskId() })
}

func TestTryToUnRegisterConsoleScriptFails(t *testing.T) {
	old := iupUninstallScript
	script := filepath.Join(t.TempDir(), "uninstall")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0755))
	iupUninstallScript = script
	t.Cleanup(func() { iupUninstallScript = old })

	m := &UpdatePlatformManager{}
	ok, err := m.tryToUnRegisterConsole()
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestTryToUnRegisterConsoleScriptSuccess(t *testing.T) {
	old := iupUninstallScript
	script := filepath.Join(t.TempDir(), "uninstall")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0755))
	iupUninstallScript = script
	t.Cleanup(func() { iupUninstallScript = old })

	// config 为 nil,resetIntranetUpdateSettingsAfterUnregister 提前返回。
	m := &UpdatePlatformManager{}
	ok, err := m.tryToUnRegisterConsole()
	assert.True(t, ok)
	assert.NoError(t, err)
}
