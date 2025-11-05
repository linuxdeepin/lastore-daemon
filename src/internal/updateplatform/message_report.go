// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system/dut"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/controller/check"

	"github.com/godbus/dbus/v5"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/utils"
	. "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

var logger = log.NewLogger("lastore/messageReport")

type ShellCheck struct {
	Name  string `json:"name"`  //检查脚本的名字
	Shell string `json:"shell"` //检查脚本的内容
}

// 发送给更新平台的状态信息
type StatusMessage struct {
	Type           string `json:"type"`           //消息类型，info,warning,error
	UpdateType     string `json:"updateType"`     //system.UpdateType类型
	JobDescription string `json:"jobDescription"` //job.Description
	Detail         string `json:"detail"`         //消息详情
}

type UpdatePlatformManager struct {
	config                            *Config
	allowPostSystemUpgradeMessageType system.UpdateType
	preBuild                          string // 进行系统更新前的版本号，如20.1060.11018.100.100，本地os-baseline获取
	preBaseline                       string // 更新前的基线号，从baseline获取，本地os-baseline获取

	targetVersion          string // 更新到的目标版本号,本地os-baseline.b获取
	targetBaseline         string // 更新到的目标基线号,本地os-baseline.b获取
	checkTime              string // 基线检查时间
	systemTypeFromPlatform string // 从更新平台获取的系统类型,本地os-baseline.b获取
	requestUrl             string // 更新平台请求地址

	PreCheck       []ShellCheck                  // 更新前检查脚本
	MidCheck       []ShellCheck                  // 更新后检查脚本
	PostCheck      []ShellCheck                  // 更新完成重启后检查脚本
	TargetCorePkgs map[string]system.PackageInfo // 必须安装软件包信息清单  	对应dut的core list
	BaselinePkgs   map[string]system.PackageInfo // 当前版本的核心软件包清单	对应dut的baseline
	SelectPkgs     map[string]system.PackageInfo // 可选软件包清单
	FreezePkgs     map[string]system.PackageInfo // 禁止升级包清单
	PurgePkgs      map[string]system.PackageInfo // 删除软件包清单

	repoInfos        []repoInfo      // 从更新平台获取的仓库信息
	SystemUpdateLogs []UpdateLogMeta // 更新注记
	cveDataTime      string
	cvePkgs          map[string][]string // cve信息 pkgname:[cveid...]

	packagesPrefixList []string // 根据仓库

	Token string
	arch  string

	Tp             UpdateTp  // 更新策略类型:1.非强制更新，2.强制更新/立即更新，3.强制更新/关机或重启时更新，4.强制更新/指定时间更新
	UpdateTime     time.Time // 更新时间(指定时间更新时的时间)
	UpdateNowForce bool      // 立即更新
	mu             sync.Mutex

	jobPostMsgMap     map[string]*UpgradePostMsg
	jobPostMsgMapMu   sync.Mutex
	inhibitAutoQuit   func()
	UnInhibitAutoQuit func()
}

type platformCacheContent struct {
	CoreListPkgs map[string]system.PackageInfo
	BaselinePkgs map[string]system.PackageInfo
	SelectPkgs   map[string]system.PackageInfo
	PreCheck     []ShellCheck
	MidCheck     []ShellCheck
	PostCheck    []ShellCheck
}

// 需要注意cache文件的同步时机，所有数据应该不会从os-version和os-baseline获取
const (
	CacheVersion  = "/var/lib/lastore/os-version.b"
	cacheBaseline = "/var/lib/lastore/os-baseline.b"
	realBaseline  = "/etc/os-baseline"
	realVersion   = "/etc/os-version"

	KeyNow      string = "now"      // 立即更新
	KeyShutdown string = "shutdown" // 关机更新
	KeyLayout   string = "15:04"
)

func NewUpdatePlatformManager(c *Config, updateToken bool) *UpdatePlatformManager {
	platformUrl := c.PlatformUrl
	if len(platformUrl) == 0 {
		platformUrl = os.Getenv("UPDATE_PLATFORM_URL")
	}

	if !utils.IsFileExist(CacheVersion) {
		err := os.Symlink(realVersion, CacheVersion)
		if err != nil {
			logger.Warning(err)
		}
	}
	if !utils.IsFileExist(cacheBaseline) {
		copyFile(realBaseline, cacheBaseline)
	}
	arch, err := GetArchInfo()
	if err != nil {
		logger.Warning(err)
	}
	var token string
	if updateToken {
		token = UpdateTokenConfigFile(c.IncludeDiskInfo) // update source时生成即可,初始化时由于授权服务返回SN非常慢(超过25s),因此不在初始化时生成
	}
	cache := platformCacheContent{}
	err = json.Unmarshal([]byte(c.OnlineCache), &cache)
	if err != nil {
		logger.Warning(err)
	}

	return &UpdatePlatformManager{
		config:                            c,
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
		preBuild:                          genPreBuild(),
		preBaseline:                       getCurrentBaseline(),
		targetVersion:                     getTargetVersion(),
		targetBaseline:                    getTargetBaseline(),
		requestUrl:                        platformUrl,
		cvePkgs:                           make(map[string][]string),
		Token:                             token,
		arch:                              arch,
		Tp:                                UnknownUpdate,
		UpdateNowForce:                    false,
		jobPostMsgMap:                     getLocalJobPostMsg(),
		TargetCorePkgs:                    cache.CoreListPkgs,
		BaselinePkgs:                      cache.BaselinePkgs,
		SelectPkgs:                        cache.SelectPkgs,
		PreCheck:                          cache.PreCheck,
		MidCheck:                          cache.MidCheck,
		PostCheck:                         cache.PostCheck,
	}
}

func (m *UpdatePlatformManager) GetCVEUpdateLogs(pkgs []string) map[string]CEVInfo {
	var cveInfos = make(map[string]CEVInfo)
	for _, name := range pkgs {
		for _, id := range m.cvePkgs[name] {
			if _, ok := cveInfos[id]; ok {
				continue
			}
			cveInfos[id] = CVEs[id]
		}
	}
	return cveInfos
}

func genPreBuild() string {
	var preBuild string
	infoMap, err := GetOSVersionInfo(CacheVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		preBuild = strings.Join(
			[]string{infoMap["MajorVersion"], infoMap["MinorVersion"], infoMap["OsBuild"]}, ".")
	}
	return preBuild
}

func copyFile(src, dst string) {
	content, err := os.ReadFile(src)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = os.WriteFile(dst, content, 0644)
	if err != nil {
		logger.Warning(err)
		return
	}
}

type updateTarget struct {
	TargetVersion   string
	TargetOsVersion string
	CheckTime       string
}

func (m *UpdatePlatformManager) GetUpdateTarget() string {
	target := &updateTarget{
		TargetOsVersion: m.targetVersion,
		TargetVersion:   m.targetBaseline,
		CheckTime:       m.checkTime,
	}
	content, err := json.Marshal(target)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return string(content)
}

/*
[General]
Baseline=""
SystemType=""
*/
func getCurrentBaseline() string {
	return getGeneralValueFromKeyFile(realBaseline, "Baseline")
}

func getCurrentSystemType() string {
	return getGeneralValueFromKeyFile(realBaseline, "SystemType")
}

func getTargetBaseline() string {
	return getGeneralValueFromKeyFile(cacheBaseline, "Baseline")
}

func getTargetSystemType() string {
	return getGeneralValueFromKeyFile(cacheBaseline, "SystemType")
}

func getTargetVersion() string {
	return getGeneralValueFromKeyFile(cacheBaseline, "Version")
}

func getGeneralValueFromKeyFile(path, key string) string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(path)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	content, err := kf.GetString("General", key)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return content
}

// UpdateBaseline 更新安装并检查成功后，同步baseline文件
func (m *UpdatePlatformManager) UpdateBaseline() {
	copyFile(cacheBaseline, realBaseline)
	m.preBaseline = getCurrentBaseline()
}

// 进行安装更新前，需要复制文件替换软连接
func (m *UpdatePlatformManager) ReplaceVersionCache() {
	if utils.IsSymlink(CacheVersion) {
		// 如果cacheVersion已经是文件了，那么不再用源文件替换
		err := os.RemoveAll(CacheVersion)
		if err != nil {
			logger.Warning(err)
		}
		copyFile(realVersion, CacheVersion)
	}
}

// RecoverVersionLink 安装更新并检查完成后，需要用软连接替换文件
func (m *UpdatePlatformManager) RecoverVersionLink() {
	err := os.RemoveAll(CacheVersion)
	if err != nil {
		logger.Warning(err)
	}
	err = os.Symlink(realVersion, CacheVersion)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *UpdatePlatformManager) UpdateBaselineCache() {
	updateBaseline(cacheBaseline, m.targetBaseline)
	updateSystemType(cacheBaseline, m.systemTypeFromPlatform)
	updateVersion(cacheBaseline, m.targetVersion)
}

func updateBaseline(path, content string) bool {
	return updateKeyFile(path, "Baseline", content)
}

func updateSystemType(path, content string) bool {
	return updateKeyFile(path, "SystemType", content)
}

func updateVersion(path, content string) bool {
	return updateKeyFile(path, "Version", content)
}

func updateKeyFile(path, key, content string) bool {
	kf := keyfile.NewKeyFile()
	if system.NormalFileExists(path) {
		err := kf.LoadFromFile(path)
		if err != nil {
			logger.Warning(err)
			return false
		}
	}
	kf.SetString("General", key, content)
	err := kf.SaveToFile(path)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}

const (
	ReleaseVersion  = 1
	UnstableVersion = 2
)

func isUnstable() int {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return ReleaseVersion
	}
	ds := ConfigManager.NewConfigManager(sysBus)
	dsPath, err := ds.AcquireManager(0, "org.deepin.unstable", "org.deepin.unstable", "")
	if err != nil {
		logger.Warning(err)
		return ReleaseVersion
	}
	unstableManager, err := ConfigManager.NewManager(sysBus, dsPath)
	if err != nil {
		logger.Warning(err)
		return ReleaseVersion
	}
	v, err := unstableManager.Value(0, "updateUnstable")
	if err != nil {
		return ReleaseVersion
	} else {
		value := v.Value().(string)
		if value == "Enabled" {
			return UnstableVersion
		} else {
			return ReleaseVersion
		}
	}
}

type Version struct {
	Version  string `json:"version"`
	Baseline string `json:"baseline"`
}

type Policy struct {
	Tp UpdateTp `json:"tp"`

	Data policyData `json:"data"`
}

type policyData struct {
	UpdateTime string `json:"updateTime"`
}

type repoInfo struct {
	Uri      string `json:"uri"`
	Cdn      string `json:"cdn"`
	CodeName string `json:"codename"`
	Version  string `json:"version"`
}

type updateMessage struct {
	SystemType string     `json:"systemType"`
	Version    Version    `json:"version"`
	Policy     Policy     `json:"policy"`
	RepoInfos  []repoInfo `json:"repoInfos"`
}

type tokenMessage struct {
	Result bool            `json:"result"`
	Code   int             `json:"code"`
	Data   json.RawMessage `json:"data"`
}
type tokenErrorMessage struct {
	Result bool   `json:"result"`
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

type requestType uint

const (
	GetVersion requestType = iota
	GetUpdateLog
	GetTargetPkgLists // 系统软件包清单
	GetCurrentPkgLists
	GetPkgCVEs // CVE 信息
	PostProcess
	PostResult
)

type requestContent struct {
	path   string
	method string
}

func (r requestType) string() string {
	return fmt.Sprintf("%v %v", Urls[r].method, Urls[r].path)
}

var Urls = map[requestType]requestContent{
	GetVersion: {
		"/api/v1/version",
		"GET",
	},
	GetTargetPkgLists: {
		"/api/v1/package",
		"GET",
	},
	GetCurrentPkgLists: {
		"/api/v1/package",
		"GET",
	},
	GetUpdateLog: {
		"/api/v1/systemupdatelogs",
		"GET",
	},
	GetPkgCVEs: {
		"/api/v1/cve/sync",
		"GET",
	},
	PostProcess: {
		"/api/v1/process",
		"POST",
	},
	PostResult: {
		"/api/v1/update/status",
		"POST",
	},
}

const secret = "DflXyFwTmaoGmbDkVj8uD62XGb01pkJn"

func (m *UpdatePlatformManager) genVersionResponse() (*http.Response, error) {
	policyUrl := m.requestUrl + Urls[GetVersion].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	request, err := http.NewRequest(Urls[GetVersion].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetVersion.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	request.Header.Set("X-Packages", base64.RawStdEncoding.EncodeToString([]byte(getClientPackageInfo(m.config.ClientPackageName))))
	return client.Do(request)
}

func (m *UpdatePlatformManager) genTargetPkgListsResponse() (*http.Response, error) {
	policyUrl := m.requestUrl + Urls[GetTargetPkgLists].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	values := url.Values{}
	values.Add("baseline", m.targetBaseline)
	policyUrl = policyUrl + "?" + values.Encode()

	request, err := http.NewRequest(Urls[GetTargetPkgLists].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetTargetPkgLists.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	return client.Do(request)
}

func (m *UpdatePlatformManager) genCurrentPkgListsResponse() (*http.Response, error) {
	policyUrl := m.requestUrl + Urls[GetCurrentPkgLists].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	values := url.Values{}
	values.Add("baseline", m.preBaseline)
	policyUrl = policyUrl + "?" + values.Encode()
	request, err := http.NewRequest(Urls[GetCurrentPkgLists].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetCurrentPkgLists.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	return client.Do(request)
}

func (m *UpdatePlatformManager) genCVEInfoResponse(syncTime string) (*http.Response, error) {
	policyUrl := m.requestUrl + Urls[GetPkgCVEs].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	values := url.Values{}
	values.Add("synctime", syncTime)
	policyUrl = policyUrl + "?" + values.Encode()
	request, err := http.NewRequest(Urls[GetPkgCVEs].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetPkgCVEs.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	return client.Do(request)
}

func (m *UpdatePlatformManager) genUpdateLogResponse() (*http.Response, error) {
	policyUrl := m.requestUrl + Urls[GetUpdateLog].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	values := url.Values{}
	values.Add("baseline", m.targetBaseline)
	values.Add("isUnstable", fmt.Sprintf("%d", isUnstable()))
	policyUrl = policyUrl + "?" + values.Encode()
	request, err := http.NewRequest(Urls[GetUpdateLog].method, policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", GetUpdateLog.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	return client.Do(request)
}

// genPostProcessResponse 生成数据，发送请求，并返回response.
// buf: 数据输入,io.Reader.
// filePath: 生成的xz压缩的中间文件.
func (m *UpdatePlatformManager) genPostProcessResponse(buf io.Reader, filePath string) (*http.Response, error) {
	policyUrl := m.requestUrl + Urls[PostProcess].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	if log.LevelDebug != logger.GetLogLevel() {
		defer os.RemoveAll(filePath)
	}
	xzFile, err := os.Create(filePath)
	if err != nil {
		logger.Warning("create file failed:", err)
		return nil, err
	}
	xzCmd := exec.Command("xz", "-z", "-c")
	xzCmd.Stdin = buf
	xzCmd.Stdout = xzFile
	if err := xzCmd.Run(); err != nil {
		_ = xzFile.Close()
		logger.Warning("exec xz command err:", err)
		return nil, err
	}
	_ = xzFile.Close()

	hash := sha256.New()
	xTime := fmt.Sprintf("%d", time.Now().Unix())

	xzFileContent, err := os.ReadFile(filePath)
	if err != nil {
		logger.Warning("open xz file failed:", err)
		return nil, err
	}
	body := bytes.NewBuffer(xzFileContent)

	hash.Write([]byte(fmt.Sprintf("%s%s%s", secret, xTime, xzFileContent)))
	sign := base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(hash.Sum(nil))))
	request, err := http.NewRequest(Urls[PostProcess].method, policyUrl, body)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", PostProcess.string(), err.Error())
	}
	hardwareId := GetHardwareId(m.config.IncludeDiskInfo)

	request.Header.Set("X-MachineID", hardwareId)
	request.Header.Set("X-CurrentBaseline", m.preBaseline)
	request.Header.Set("X-Baseline", m.targetBaseline)
	request.Header.Set("X-Time", xTime)
	request.Header.Set("X-Sign", sign)
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(m.Token)))
	logger.Debug("genPostProcessResponse:", request.Header)
	return client.Do(request)
}

func getResponseData(response *http.Response, reqType requestType) (json.RawMessage, error) {
	if http.StatusOK == response.StatusCode {
		respData, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("%v failed to read response body: %v ", response.Request.RequestURI, err.Error())
		}
		logger.Infof("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
		msg := &tokenMessage{}
		err = json.Unmarshal(respData, msg)
		if err != nil {
			logger.Warningf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
			return nil, fmt.Errorf("%v failed to Unmarshal respData to tokenMessage: %v ", reqType.string(), err.Error())
		}
		if !msg.Result {
			logger.Warningf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
			errorMsg := &tokenErrorMessage{}
			err = json.Unmarshal(respData, errorMsg)
			if err != nil {
				return nil, fmt.Errorf("%v request for %s", reqType.string(), response.Request.RequestURI)
			}
			return nil, fmt.Errorf("%v request for %s err:%s", reqType.string(), response.Request.RequestURI, errorMsg.Msg)
		}
		return msg.Data, nil
	} else {
		return nil, fmt.Errorf("request for %s failed, response code=%d", response.Request.RequestURI, response.StatusCode)
	}
}

func getVersionData(data json.RawMessage) *updateMessage {
	tmp := &updateMessage{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to updateMessage: %v ", GetVersion.string(), err.Error())
		return nil
	}
	return tmp
}

func getTargetPkgListData(data json.RawMessage) *PreInstalledPkgMeta {
	tmp := &PreInstalledPkgMeta{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to PreInstalledPkgMeta: %v ", GetTargetPkgLists.string(), err.Error())
		return nil
	}
	return tmp
}

func getCurrentPkgListsData(data json.RawMessage) *PreInstalledPkgMeta {
	tmp := &PreInstalledPkgMeta{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to PreInstalledPkgMeta: %v ", GetCurrentPkgLists.string(), err.Error())
		return nil
	}
	return tmp
}
func getUpdateLogData(data json.RawMessage) []UpdateLogMeta {
	var tmp []UpdateLogMeta
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to UpdateLogMeta: %v ", GetUpdateLog.string(), err.Error())
		return nil
	}
	return tmp
}

func getCVEData(data json.RawMessage) *CVEMeta {
	tmp := &CVEMeta{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		logger.Warningf("%v failed to Unmarshal msg.Data to CVEMeta: %v ", GetPkgCVEs.string(), err.Error())
		return tmp
	}
	return tmp
}

type UpdateTp int

const (
	UnknownUpdate   UpdateTp = 0
	NormalUpdate    UpdateTp = 1 // 更新
	UpdateNow       UpdateTp = 2 // 立即更新 // 以下为强制更新
	UpdateShutdown  UpdateTp = 3 // 关机更新
	UpdateRegularly UpdateTp = 4 // 定时更新
)

func IsForceUpdate(tp UpdateTp) bool {
	if tp >= UpdateNow && tp <= UpdateRegularly {
		return true
	}
	return false
}

// GenUpdatePolicyByToken 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) genUpdatePolicyByToken(updateInRelease bool) error {
	response, err := m.genVersionResponse()
	if err != nil {
		return fmt.Errorf("failed get version data %v", err)
	}
	data, err := getResponseData(response, GetVersion)
	if err != nil {
		return fmt.Errorf("failed get version data %v", err)
	}
	msg := getVersionData(data)
	if msg == nil {
		return errors.New("failed get version data")
	}
	m.targetBaseline = msg.Version.Baseline
	m.targetVersion = msg.Version.Version
	m.systemTypeFromPlatform = msg.SystemType
	m.repoInfos = msg.RepoInfos
	m.checkTime = time.Now().String()
	// 更新策略处理
	m.Tp = msg.Policy.Tp
	if m.Tp == UpdateRegularly {
		m.UpdateTime, _ = time.Parse(time.RFC3339, msg.Policy.Data.UpdateTime)
	}

	m.UpdateBaselineCache()
	// 生成仓库和InRelease
	if updateInRelease {
		m.genDepositoryFromPlatform()
		m.checkInReleaseFromPlatform()
	}

	return nil
}

func (m *UpdatePlatformManager) GenUpdatePolicyByToken(updateInRelease bool) error {
	var err error
	if (m.config.PlatformDisabled & DisabledVersion) == 0 {
		err = m.genUpdatePolicyByToken(updateInRelease)
	}
	// 根据配置更新Tp
	switch m.Tp {
	case UpdateNow:
	case UpdateShutdown:
	case UpdateRegularly:
	default:
		logger.Debug("Config update time:", m.config.UpdateTime)
		switch m.config.UpdateTime {
		case KeyNow:
			m.Tp = UpdateNow
		case KeyShutdown:
			m.Tp = UpdateShutdown
		default:
			updateTime, err := time.Parse(time.RFC3339, m.config.UpdateTime)
			if err == nil {
				m.Tp = UpdateRegularly
				m.UpdateTime = updateTime
			} else {
				logger.Warning(err)
			}
		}
	}
	logger.Info("Policy tp:", m.Tp, "update time:", m.UpdateTime)
	logger.Info("pre Baseline:", m.preBaseline, "target Baseline：", m.targetBaseline)
	if len(m.targetBaseline) == 0 || m.preBaseline == m.targetBaseline {
		m.Tp = NormalUpdate
	}
	m.UpdateNowForce = false
	switch m.Tp {
	case UpdateNow:
		m.UpdateNowForce = true
	case UpdateRegularly:
		// 距更新时间3min内则立即更新
		nowTime := time.Now()
		dot := time.Duration(3) * time.Minute
		bTime := nowTime.Add(dot)
		updateTime := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), m.UpdateTime.Hour(), m.UpdateTime.Minute(), 0, 0, nowTime.Location())
		if m.Tp == UpdateRegularly && updateTime.Before(bTime) && updateTime.After(nowTime) {
			m.UpdateNowForce = true
		}
		m.config.SetInstallUpdateTime(m.UpdateTime.Format(time.RFC3339))
	}
	logger.Info("Force Update:", IsForceUpdate(m.Tp), "update Immediate:", m.UpdateNowForce, "updateTime:", m.UpdateTime)
	return err
}

type packageLists struct {
	Core   []system.PlatformPackageInfo `json:"core"`   // "必须安装软件包清单"
	Select []system.PlatformPackageInfo `json:"select"` // "可选软件包清单"
	Freeze []system.PlatformPackageInfo `json:"freeze"` // "禁止升级包清单"
	Purge  []system.PlatformPackageInfo `json:"purge"`  // "删除软件包清单"
}

type PreInstalledPkgMeta struct {
	PreCheck  []ShellCheck `json:"preCheck"`  // "更新前检查脚本"
	MidCheck  []ShellCheck `json:"midCheck"`  // "更新后检查脚本"
	PostCheck []ShellCheck `json:"postCheck"` // "更新完成重启后检查脚本"
	Packages  packageLists `json:"packages"`  // "基线软件包清单"
}

// 从更新平台获取升级目标版本的软件包清单
func (m *UpdatePlatformManager) updateTargetPkgMetaSync() error {
	response, err := m.genTargetPkgListsResponse()
	if err != nil {
		return fmt.Errorf("failed get target pkg list data %v", err)
	}
	data, err := getResponseData(response, GetTargetPkgLists)
	if err != nil {
		return fmt.Errorf("failed get target pkg list data %v", err)
	}

	pkgs := getTargetPkgListData(data)
	if pkgs == nil {
		return errors.New("failed get target pkg list data")
	}
	m.PreCheck = pkgs.PreCheck
	m.MidCheck = pkgs.MidCheck
	m.PostCheck = pkgs.PostCheck

	for _, pkg := range pkgs.Packages.Core {
		version := ""
		hasMatch := false
		for _, v := range pkg.AllArchVersion {
			if strings.Contains(v.Arch, m.arch) {
				version = v.Version
				hasMatch = true
				break
			}
		}
		if hasMatch {
			m.TargetCorePkgs[pkg.Name] = system.PackageInfo{
				Name:    pkg.Name,
				Need:    pkg.Need,
				Version: version,
			}
		}
	}

	for _, pkg := range pkgs.Packages.Select {
		version := ""
		hasMatch := false
		for _, v := range pkg.AllArchVersion {
			if strings.Contains(v.Arch, m.arch) {
				version = v.Version
				hasMatch = true
				break
			}
		}
		if hasMatch {
			m.SelectPkgs[pkg.Name] = system.PackageInfo{
				Name:    pkg.Name,
				Version: version,
				Need:    pkg.Need,
			}
		}
	}
	for _, pkg := range pkgs.Packages.Freeze {
		version := ""
		hasMatch := false
		for _, v := range pkg.AllArchVersion {
			if strings.Contains(v.Arch, m.arch) {
				version = v.Version
				hasMatch = true
				break
			}
		}
		if hasMatch {
			m.FreezePkgs[pkg.Name] = system.PackageInfo{
				Name:    pkg.Name,
				Version: version,
				Need:    pkg.Need,
			}
		}

	}
	for _, pkg := range pkgs.Packages.Purge {
		version := ""
		hasMatch := false
		for _, v := range pkg.AllArchVersion {
			if v.Arch == m.arch {
				version = v.Version
				hasMatch = true
				break
			}
		}
		if hasMatch {
			m.PurgePkgs[pkg.Name] = system.PackageInfo{
				Name:    pkg.Name,
				Version: version,
				Need:    pkg.Need,
			}
		}
	}

	return nil
}

// 从更新平台获取当前版本的预装清单
func (m *UpdatePlatformManager) updateCurrentPreInstalledPkgMetaSync() error {
	response, err := m.genCurrentPkgListsResponse()
	if err != nil {
		return fmt.Errorf("failed get current pkg list data %v", err)
	}
	data, err := getResponseData(response, GetCurrentPkgLists)
	if err != nil {
		return fmt.Errorf("failed get current pkg list data %v", err)
	}
	pkgs := getCurrentPkgListsData(data)
	if pkgs == nil {
		return errors.New("failed get current pkg list data")
	}
	for _, pkg := range pkgs.Packages.Core {
		version := ""
		hasMatch := false
		for _, v := range pkg.AllArchVersion {
			if v.Arch == m.arch {
				version = v.Version
				hasMatch = true
				break
			}
		}
		if hasMatch {
			m.BaselinePkgs[pkg.Name] = system.PackageInfo{
				Name:    pkg.Name,
				Version: version,
				Need:    pkg.Need,
			}
		}
	}

	return nil
}

type CEVInfo struct {
	SyncTime       string `json:"synctime"`       // "CVE类型"
	CveId          string `json:"cveId"`          // "CVE编号"
	Source         string `json:"source"`         // "包名"
	FixedVersion   string `json:"fixedVersion"`   // "修复版本"
	Archs          string `json:"archs"`          // "架构信息"
	Score          string `json:"score"`          // "评分"
	Status         string `json:"status"`         // "修复状态"
	VulCategory    string `json:"vulCategory"`    // "漏洞类型"
	VulName        string `json:"vulName"`        // "漏洞名称"
	VulLevel       string `json:"vulLevel"`       // "⻛险等级"
	PubTime        string `json:"pubTime"`        // "CVE公开时间"
	Binary         string `json:"binary"`         // "二进制包"
	Description    string `json:"description"`    // "漏洞描述"
	CveDescription string `json:"cveDescription"` // "漏洞描述(英文)"
}

type CVEMeta struct {
	DateTime string    `json:"dateTime"`
	Cves     []CEVInfo `json:"cves"`
}

var CVEs map[string]CEVInfo // 保存全局cves信息，方便查询

const cveLocalInfo = "/var/lib/lastore/cve_local_info.json"

func loadLocalCVEData() []byte {
	data, err := os.ReadFile(cveLocalInfo)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	return data
}

func saveCEVData(meta CVEMeta) {
	data, err := json.Marshal(meta)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = os.WriteFile(cveLocalInfo, data, 0644)
	if err != nil {
		logger.Warning(err)
		return
	}
}

// 从更新平台获取CVE元数据
func (m *UpdatePlatformManager) updateCVEMetaDataSync() error {
	localData := loadLocalCVEData()
	localCVE := getCVEData(localData)
	response, err := m.genCVEInfoResponse(localCVE.DateTime)
	if err != nil {
		return fmt.Errorf("failed get cve meta info %v", err)
	}
	data, err := getResponseData(response, GetPkgCVEs)
	if err != nil {
		return fmt.Errorf("failed get cve meta info %v", err)
	}
	cves := getCVEData(data)
	if cves == nil {
		return errors.New("failed get cve meta info")
	}
	cves.Cves = append(cves.Cves, localCVE.Cves...)
	saveCEVData(*cves)
	// 重置CVEs
	CVEs = make(map[string]CEVInfo)
	m.cveDataTime = cves.DateTime
	for _, cve := range cves.Cves {
		CVEs[cve.CveId] = cve
		str := cve.Binary
		str = strings.ReplaceAll(str, "[", "")
		str = strings.ReplaceAll(str, "]", "")
		str = strings.ReplaceAll(str, " ", "")
		str = strings.ReplaceAll(str, "'", "")
		if str == "None" || len(str) == 0 {
			continue
		}

		binarys := strings.Split(str, ",")
		if len(binarys) > 0 {
			for _, binary := range binarys {
				m.cvePkgs[binary] = append(m.cvePkgs[binary], cve.CveId)
			}
		}

	}
	return nil
}

func (m *UpdatePlatformManager) GetSystemMeta() map[string]system.PackageInfo {
	infos := make(map[string]system.PackageInfo)
	for name, info := range m.TargetCorePkgs {
		infos[name] = info
	}
	// 暂时应该只有core中的包是需要装的,可选包需要由前端选择
	// for name, info := range m.selectPkgs {
	// 	infos[name] = info
	// }
	return infos
}

type UpdateLogMeta struct {
	Baseline      string    `json:"baseline"`
	ShowVersion   string    `json:"showVersion"`
	CnLog         string    `json:"cnLog"`
	EnLog         string    `json:"enLog"`
	LogType       int       `json:"logType"`
	IsUnstable    int       `json:"isUnstable"`
	SystemVersion string    `json:"systemVersion"`
	PublishTime   time.Time `json:"publishTime"`
}

// 如果更新日志无法获取到,不会返回错误,而是设置默认日志文案
func (m *UpdatePlatformManager) updateLogMetaSync() error {
	response, err := m.genUpdateLogResponse()
	if err != nil {
		logger.Warning(err)
		return nil
	}
	data, err := getResponseData(response, GetUpdateLog)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	forceLog := response.Header.Get("X-Force")
	if forceLog == "true" {
		// 强制使用更新平台日志
		m.SystemUpdateLogs = getUpdateLogData(data)
	} else {
		// 根据本地环境判断是否使用更新平台日志
		// m.targetVersion
		m.SystemUpdateLogs = make([]UpdateLogMeta, 0)
		osVersionInfoMap, err := GetOSVersionInfo(realVersion)
		if err != nil {
			logger.Warning("failed to get os-version:", err)
			// 获取本地版本号失败，使用默认日志
		} else {
			minorVersion := osVersionInfoMap["MinorVersion"]
			osBuild := osVersionInfoMap["OsBuild"]
			osBuildSlice := strings.Split(osBuild, ".")
			var secVersionStr string
			if len(osBuildSlice) >= 3 {
				var globalVersionStr string
				var ok bool
				// 社区版的version是minorVersion，直接可以显示小版本
				if osVersionInfoMap["EditionName"] == "Community" {
					ok = true
				} else {
					secVersionStr = osBuildSlice[1]
					secVersionInt, err := strconv.Atoi(secVersionStr)
					if err != nil {
						logger.Warning(err)
						return nil
					}
					realSecVersion := secVersionInt - 100
					minorVersionInt, err := strconv.Atoi(minorVersion)
					if err != nil {
						logger.Warning(err)
						return nil
					}
					globalVersion := minorVersionInt + realSecVersion
					if globalVersion < 1000 {
						logger.Warningf("system version is %v, not support compare with %v", globalVersion, m.targetVersion)
						return nil
					}
					targetVersionInt, err := strconv.Atoi(m.targetVersion)
					if err != nil {
						logger.Warning(err)
						return nil
					}
					if targetVersionInt > globalVersion {
						ok = true
					}
					globalVersionStr = fmt.Sprintf("%v", globalVersion)
				}
				logData := getUpdateLogData(data)
				if ok {
					m.SystemUpdateLogs = logData
				} else {
					var lastLog UpdateLogMeta
					if logData != nil && len(logData) > 0 {
						lastLog = logData[0]
					}

					m.SystemUpdateLogs = append(m.SystemUpdateLogs, UpdateLogMeta{
						Baseline:      lastLog.Baseline,
						ShowVersion:   globalVersionStr,
						CnLog:         "修复部分系统已知问题与缺陷",
						EnLog:         "Fixing some of the system's known problems and defects",
						LogType:       lastLog.LogType,
						IsUnstable:    lastLog.IsUnstable,
						SystemVersion: lastLog.SystemVersion,
						PublishTime:   lastLog.PublishTime,
					})
				}
			}
		}
	}
	return nil
}

func (m *UpdatePlatformManager) genDepositoryFromPlatform() {
	prefix := "deb"
	// v25上应该是这个
	suffix := "main community commercial"
	if m.config.PlatformRepoComponents != "" {
		suffix = m.config.PlatformRepoComponents
	}
	var repos []string
	for _, repo := range m.repoInfos {
		codeName := repo.CodeName
		// 如果有cdn，则使用cdn，效率更高
		var uri = repo.Uri
		// if repo.Cdn != "" {
		// 	uri = repo.Cdn
		// }
		repos = append(repos, fmt.Sprintf("%s %s %s %s", prefix, uri, codeName, suffix))
	}
	err := os.WriteFile(system.PlatFormSourceFile, []byte(strings.Join(repos, "\n")), 0644)
	if err != nil {
		logger.Warning("update source list file err")
	}

}

func getAptAuthConf(domain string) (user, password string) {
	AuthFile := "/etc/apt/auth.conf.d/uos.conf"
	file, err := os.Open(AuthFile)
	if err != nil {
		logger.Warning("failed open uos.conf:", err)
		return "", ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// 逐行读取文件内容
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), " ")
		if len(line) < 6 {
			continue
		}
		if line[1] == domain {
			return line[3], line[5]
		}
	}
	return "", ""
}

// 校验InRelease文件，如果平台和本地不同，则删除
func (m *UpdatePlatformManager) checkInReleaseFromPlatform() {
	// 更新获取InRelease文件
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	// 用于Debug时查看重定向地址
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		logger.Info("CheckRedirect  :", req.Response.Header)
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}

	for _, repo := range m.repoInfos {
		func(repo repoInfo) {
			needRemoveCache := new(bool)
			*needRemoveCache = true
			var cachePrefix string
			defer func() {
				if *needRemoveCache {
					infos, err := os.ReadDir(system.OnlineListPath)
					if err != nil {
						logger.Warning(err)
						_ = os.RemoveAll(system.OnlineListPath)
						return
					}
					for _, info := range infos {
						if strings.Contains(info.Name(), cachePrefix) {
							_ = os.RemoveAll(filepath.Join(system.OnlineListPath, info.Name()))
						}
					}
				}
			}()
			// 如果有cdn，则使用cdn，效率更高
			var uri = repo.Uri
			if repo.Cdn != "" {
				uri = repo.Cdn
			}
			cachePrefix = strings.ReplaceAll(utils.URIToPath(fmt.Sprintf("%s/dists/%s", uri, repo.CodeName)), "/", "_")
			uri = fmt.Sprintf("%s/dists/%s/InRelease", uri, repo.CodeName)
			logger.Info("prefix:", cachePrefix)
			request, err := http.NewRequest("GET", uri, nil)
			if err != nil {
				logger.Warning(err)
				return
			}

			// 获取仓库文件路径
			file := utils.URIToPath(uri)
			if len(file) == 0 {
				logger.Warning("illegal uri:", repo.Uri)
				return
			}
			// 获取域名
			domain := strings.Split(file, "/")[0]

			request.Header.Set("X-Repo-Token", m.Token)
			request.SetBasicAuth(getAptAuthConf(domain))
			resp, err := client.Do(request)
			if err != nil {
				logger.Warning(err)
				return
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
			default:
				logger.Warningf("failed download InRelease:%s ,respCode:%d", uri, resp.StatusCode)
				return
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				logger.Warning(err)
				return
			}

			file = strings.ReplaceAll(file, "/", "_")
			lastoreFile := "/tmp/" + file
			aptFile := filepath.Join(system.OnlineListPath, file)

			err = os.WriteFile(lastoreFile, data, 0644)
			if err != nil {
				logger.Warning(err)
				return
			}
			if utils.IsFileExist(aptFile) {
				// 文件存在，则校验MD5值
				aptSum, ok := utils.SumFileMd5(aptFile)
				if !ok {
					logger.Warningf("check %s md5sum failed", aptFile)
					return
				}
				lastoreSum, ok := utils.SumFileMd5(lastoreFile)
				if !ok {
					logger.Warningf("check %s md5sum failed", lastoreFile)
					return
				}
				if aptSum != lastoreSum {
					logger.Warning("InRelease changed:", aptFile)
				} else {
					logger.Warningf("InRelease unchanged: %s", aptFile)
					*needRemoveCache = false
				}
			}
			// stat 失败情况可以直接忽略，通过apt update获取索引即可
		}(repo)
	}
}

// UpdateAllPlatformDataSync 同步获取所有需要从更新平台获取的数据
func (m *UpdatePlatformManager) UpdateAllPlatformDataSync() error {
	var wg sync.WaitGroup
	var errList []string
	var syncFuncList []func() error
	m.TargetCorePkgs = make(map[string]system.PackageInfo)
	m.BaselinePkgs = make(map[string]system.PackageInfo)
	m.SelectPkgs = make(map[string]system.PackageInfo)
	m.FreezePkgs = make(map[string]system.PackageInfo)
	m.PurgePkgs = make(map[string]system.PackageInfo)
	if (m.config.PlatformDisabled & DisabledUpdateLog) == 0 {
		syncFuncList = append(syncFuncList, m.updateLogMetaSync) // 日志
	}
	if (m.config.PlatformDisabled & DisabledTargetPkgLists) == 0 {
		syncFuncList = append(syncFuncList, m.updateTargetPkgMetaSync) // 目标版本信息
	}
	if (m.config.PlatformDisabled & DisabledCurrentPkgLists) == 0 {
		syncFuncList = append(syncFuncList, m.updateCurrentPreInstalledPkgMetaSync) // 基线版本信息
	}
	if (m.config.PlatformDisabled & DisabledPkgCVEs) == 0 {
		syncFuncList = append(syncFuncList, m.updateCVEMetaDataSync) // cve信息
	}
	for _, syncFunc := range syncFuncList {
		wg.Add(1)
		go func(f func() error) {
			err := f()
			if err != nil {
				errList = append(errList, err.Error())
			}
			wg.Done()
		}(syncFunc)

	}
	wg.Wait()
	if len(errList) > 0 {
		return errors.New(strings.Join(errList, "\n"))
	}
	return nil
}

// PostStatusMessage 将检查\下载\安装过程中所有异常状态和每个阶段成功的正常状态上报
func (m *UpdatePlatformManager) PostStatusMessage(message StatusMessage) {
	if (m.config.PlatformDisabled & DisabledProcess) != 0 {
		return
	}

	msg, err := json.Marshal(message)
	if err != nil {
		logger.Warningf("marshal status message failed:%v", err)
		return
	}

	logger.Debugf("post status msg:%s", msg)

	buf := bytes.NewBufferString(string(msg))
	filePath := fmt.Sprintf("/tmp/%s_%s.xz", "update", time.Now().Format("20231019102233444"))
	response, err := m.genPostProcessResponse(buf, filePath)
	if err != nil {
		logger.Warningf("post status message failed:%v", err)
		return
	}
	data, err := getResponseData(response, PostProcess)
	if err != nil {
		logger.Warningf("get post status response failed:%v", err)
		return
	}
	logger.Info(string(data))
}

func tarFiles(files []string, outFile string) error {
	// 创建tar包文件
	tarFile, err := os.Create(outFile)
	if err != nil {
		logger.Warning("create tar failed:", err)
		return err
	}
	defer tarFile.Close()

	// 创建tar包写入器
	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()
	// 将文件添加到tar包中
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			logger.Warning("open file failed:", err)
			return err
		}
		defer file.Close()

		// 获取文件信息
		info, err := file.Stat()
		if err != nil {
			logger.Warning("get file info err:", err)
			return err
		}

		// 创建tar头部信息
		header := new(tar.Header)
		header.Name = filepath.Base(filePath)
		header.Size = info.Size()
		header.Mode = int64(info.Mode())
		header.ModTime = info.ModTime()

		// 写入tar头部信息
		if err := tarWriter.WriteHeader(header); err != nil {
			logger.Warning("create tar header failed:", err)
			return err
		}

		// 写入文件内容到tar包
		if _, err := io.Copy(tarWriter, file); err != nil {
			logger.Warning("input data to tar failed:", err)
			return err
		}
	}
	return nil
}

// PostUpdateLogFiles 将更新日志上传
func (m *UpdatePlatformManager) PostUpdateLogFiles(files []string) {
	if (m.config.PlatformDisabled & DisabledProcess) != 0 {
		return
	}
	hardwareId := GetHardwareId(m.config.IncludeDiskInfo)

	outFilename := fmt.Sprintf("/tmp/%s_%s_%s_%s.tar", "update", hardwareId, utils.GenUuid(), time.Now().Format("20231019102233444"))
	err := tarFiles(files, outFilename)
	if err != nil {
		logger.Warningf("tar log files failed:%v", err)
		return
	}
	tarFile, err := os.Open(outFilename)
	if err != nil {
		logger.Warning("open file failed:", err)
		return
	}
	defer tarFile.Close()
	response, err := m.genPostProcessResponse(tarFile, outFilename+".xz")
	if err != nil {
		logger.Warningf("post status message failed:%v", err)
		return
	}
	data, err := getResponseData(response, PostProcess)
	if err != nil {
		logger.Warningf("get post status response failed:%v", err)
		return
	}
	logger.Info(string(data))
}

func (m *UpdatePlatformManager) needPostSystemUpgradeMessage(mode system.UpdateType) bool {
	var editionName string
	infoMap, err := GetOSVersionInfo(CacheVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		editionName = infoMap["EditionName"]
	}
	return strv.Strv(m.config.AllowPostSystemUpgradeMessageVersion).Contains(editionName) && ((mode & m.allowPostSystemUpgradeMessageType) != 0)
}

// CreateJobPostMsgInfo 初始化创建上报信息
func (m *UpdatePlatformManager) CreateJobPostMsgInfo(uuid string, updateType system.UpdateType) {
	if !m.needPostSystemUpgradeMessage(updateType) {
		return
	}
	info := &UpgradePostMsg{
		Uuid:             uuid,
		UpgradeStartTime: time.Now().Unix(),
		PostStatus:       NotReady,
	}
	info.save()
	m.jobPostMsgMapMu.Lock()
	defer m.jobPostMsgMapMu.Unlock()
	m.jobPostMsgMap[uuid] = info
	return
}

// SaveJobPostMsgByUUID 需要在success或failed状态迁移前调用,保证数据存储
func (m *UpdatePlatformManager) SaveJobPostMsgByUUID(uuid string, upgradeStatus UpgradeResult, Description string) {
	m.jobPostMsgMapMu.Lock()
	defer m.jobPostMsgMapMu.Unlock()
	if msg, ok := m.jobPostMsgMap[uuid]; ok {
		var upgradeErrorMsg string
		if upgradeStatus == UpgradeFailed || upgradeStatus == CheckFailed {
			upgradeErrorMsg = Description
		}
		hardwareId := GetHardwareId(m.config.IncludeDiskInfo)
		msg.MachineID = hardwareId
		msg.UpgradeStatus = upgradeStatus
		msg.UpgradeErrorMsg = upgradeErrorMsg
		msg.PreBuild = m.preBuild
		msg.NextShowVersion = m.targetVersion
		msg.PreBaseline = m.preBaseline
		msg.NextBaseline = m.targetBaseline
		msg.UpgradeEndTime = time.Now().Unix()
		msg.updatePostStatus(WaitPost)
	}
}

// PostSystemUpgradeMessage 发送系统更新成功或失败的状态
func (m *UpdatePlatformManager) PostSystemUpgradeMessage(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobPostMsgMapMu.Lock()
	defer m.jobPostMsgMapMu.Unlock()
	msg, ok := m.jobPostMsgMap[uuid]
	if !ok {
		return
	}
	msg.updateTimeStamp()
	content, err := json.Marshal(msg)
	if err != nil {
		logger.Warning(err)
		return
	}

	logger.Debugf("upgrade post content is %v", string(content))
	encryptMsg, err := EncryptMsg(content)
	if err != nil {
		logger.Warning(err)
		return
	}
	base64EncodeString := base64.StdEncoding.EncodeToString(encryptMsg)
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	requestUrl := m.requestUrl + "/api/v1/update/status"
	request, err := http.NewRequest("POST", requestUrl, strings.NewReader(base64EncodeString))
	if err != nil {
		logger.Warning(err)
		return
	}
	response, err := client.Do(request)
	if err == nil {
		defer func() {
			_ = response.Body.Close()
		}()
		body, _ := io.ReadAll(response.Body)
		logger.Debug("postSystemUpgradeMessage response is:", string(body))
		msg.updatePostStatus(PostSuccess)
		delete(m.jobPostMsgMap, uuid)
	} else {
		logger.Warning("post upgrade message failed:", err)
	}
}

func (m *UpdatePlatformManager) RetryPostHistory() {
	for _, v := range m.jobPostMsgMap {
		if v.PostStatus == WaitPost || v.PostStatus == PostFailure {
			m.PostSystemUpgradeMessage(v.Uuid)
		}
	}
	return
}

// PrepareCheckScripts decodes and deploys base64-encoded check scripts to the filesystem.
// It processes three types of check scripts:
//   - PreCheck: scripts executed before system update
//   - MidCheck: scripts executed during system update
//   - PostCheck: scripts executed after system update
//
// The function performs the following operations:
//  1. Cleans up existing script directories
//  2. Creates fresh directories for each check type
//  3. Decodes base64-encoded script content from memory
//  4. Writes executable script files to the filesystem with 0755 permissions
//
// Script files are organized under /var/lib/lastore/check/ in subdirectories:
//   - pre_check/: for pre-update validation scripts
//   - mid_check/: for mid-update validation scripts
//   - post_check/: for post-update validation scripts
//
// Any decode or file write errors are logged as warnings but do not stop
// the processing of remaining scripts.
func (m *UpdatePlatformManager) PrepareCheckScripts() {

	preShellPath := filepath.Join(check.CheckBaseDir, "pre_check")
	midShellPath := filepath.Join(check.CheckBaseDir, "mid_check")
	postShellPath := filepath.Join(check.CheckBaseDir, "post_check")

	_ = os.RemoveAll(preShellPath)
	_ = os.RemoveAll(midShellPath)
	_ = os.RemoveAll(postShellPath)

	err := utils.EnsureDirExist(preShellPath)
	if err != nil {
		logger.Warning(err)
	}

	err = utils.EnsureDirExist(midShellPath)
	if err != nil {
		logger.Warning(err)
	}

	err = utils.EnsureDirExist(postShellPath)
	if err != nil {
		logger.Warning(err)
	}

	checkGroups := []struct {
		list      []ShellCheck
		dir       string
		checkType dut.CheckType
	}{
		{m.PreCheck, preShellPath, dut.PreCheck},
		{m.MidCheck, midShellPath, dut.MidCheck},
		{m.PostCheck, postShellPath, dut.PostCheck},
	}

	for _, g := range checkGroups {
		for _, c := range g.list {
			filePath := filepath.Join(g.dir, c.Name)
			content, err := base64.RawStdEncoding.DecodeString(c.Shell)
			if err != nil {
				logger.Warningf("decode shell for %s failed: %v", c.Name, err)
				continue
			}

			if err := utils.SyncWriteFile(filePath, content, 0755); err != nil {
				logger.Warningf("write file %s failed: %v", filePath, err)
				continue
			}

		}
	}
}

func (m *UpdatePlatformManager) SaveCache(c *Config) {
	cache := platformCacheContent{}
	cache.CoreListPkgs = m.TargetCorePkgs
	cache.BaselinePkgs = m.BaselinePkgs
	cache.SelectPkgs = m.SelectPkgs
	cache.PreCheck = m.PreCheck
	cache.MidCheck = m.MidCheck
	cache.PostCheck = m.PostCheck
	content, err := json.Marshal(cache)
	if err != nil {
		logger.Warning("marshal cache failed:", err)
		return
	}
	err = c.SetOnlineCache(string(content))
	if err != nil {
		logger.Warning("save cache failed:", err)
	}
}

func (m *UpdatePlatformManager) PostUpgradeStatus(uuid string, upgradeStatus UpgradeResult, Description string) {
	m.SaveJobPostMsgByUUID(uuid, upgradeStatus, Description)
	go func() {
		if m.inhibitAutoQuit != nil && m.UnInhibitAutoQuit != nil {
			m.inhibitAutoQuit()
			defer m.inhibitAutoQuit()
		}
		m.PostSystemUpgradeMessage(uuid)
	}()
}

func (m *UpdatePlatformManager) SetInhibitAutoQuit() {

}
