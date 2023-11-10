// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"internal/system"
	"internal/system/dut"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/utils"
)

type UpdatePlatformManager struct {
	config                            *Config
	userAgents                        *userAgentMap
	allowPostSystemUpgradeMessageType system.UpdateType
	preBuild                          string // 进行系统更新前的版本号，如20.1060.11018.100.100，本地os-baseline获取
	preBaseline                       string // 更新前的基线号，从baseline获取，本地os-baseline获取

	targetVersion          string // 更新到的目标版本号,本地os-baseline.b获取
	targetBaseline         string // 更新到的目标基线号,本地os-baseline.b获取
	checkTime              string // 基线检查时间
	systemTypeFromPlatform string // 从更新平台获取的系统类型,本地os-baseline.b获取
	requestUrl             string // 更新平台请求地址

	preCheck       string                        // 更新前检查脚本
	midCheck       string                        // 更新中检查脚本
	postCheck      string                        // 更新后检查脚本
	targetCorePkgs map[string]system.PackageInfo // 必须安装软件包信息清单  	对应dut的core list
	baselinePkgs   map[string]system.PackageInfo // 当前版本的核心软件包清单	对应dut的baseline
	selectPkgs     map[string]system.PackageInfo // 可选软件包清单
	freezePkgs     map[string]system.PackageInfo // 禁止升级包清单
	purgePkgs      map[string]system.PackageInfo // 删除软件包清单

	repoInfos        []repoInfo      // 从更新平台获取的仓库信息
	systemUpdateLogs []UpdateLogMeta // 更新注记
	cveDataTime      string
	cvePkgs          map[string][]string // cve信息 pkgname:[cveid...]

	packagesPrefixList []string // 根据仓库

	token string
	arch  string
}

// 需要注意cache文件的同步时机，所有数据应该不会从os-version和os-baseline获取
const (
	cacheVersion  = "/var/lib/lastore/os-version.b"
	cacheBaseline = "/var/lib/lastore/os-baseline.b"
	realBaseline  = "/etc/os-baseline"
	realVersion   = "/etc/os-version"
)

func isZH() bool {
	lang := gettext.QueryLang()
	return strings.HasPrefix(lang, "zh")
}

func newUpdatePlatformManager(c *Config, agents *userAgentMap) *UpdatePlatformManager {
	platformUrl := os.Getenv("UPDATE_PLATFORM_URL")
	if len(platformUrl) == 0 {
		platformUrl = "https://update-platform-pre.uniontech.com"
	}

	if !utils.IsFileExist(cacheVersion) {
		err := os.Symlink(realVersion, cacheVersion)
		if err != nil {
			logger.Warning(err)
		}
	}
	if !utils.IsFileExist(cacheBaseline) {
		copyFile(realBaseline, cacheBaseline)
	}
	arch, err := getArchInfo()
	if err != nil {
		logger.Warning(err)
	}
	return &UpdatePlatformManager{
		config:                            c,
		userAgents:                        agents,
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
		preBuild:                          genPreBuild(),
		preBaseline:                       getCurrentBaseline(),
		targetVersion:                     getTargetVersion(),
		targetBaseline:                    getTargetBaseline(),
		requestUrl:                        platformUrl,
		cvePkgs:                           make(map[string][]string),
		token:                             updateTokenConfigFile(),
		targetCorePkgs:                    make(map[string]system.PackageInfo),
		baselinePkgs:                      make(map[string]system.PackageInfo),
		selectPkgs:                        make(map[string]system.PackageInfo),
		freezePkgs:                        make(map[string]system.PackageInfo),
		purgePkgs:                         make(map[string]system.PackageInfo),
		arch:                              arch,
	}
}

func (m *UpdatePlatformManager) GetCVEUpdateLogs(pkgs map[string]system.PackageInfo) map[string]CEVInfo {
	var cveInfos = make(map[string]CEVInfo)
	for name, _ := range pkgs {
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
	infoMap, err := getOSVersionInfo(cacheVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		preBuild = strings.Join(
			[]string{infoMap["MajorVersion"], infoMap["MinorVersion"], infoMap["OsBuild"]}, ".")
	}
	return preBuild
}

func copyFile(src, dst string) {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = ioutil.WriteFile(dst, content, 0644)
	if err != nil {
		logger.Warning(err)
		return
	}
}

type updateTarget struct {
	TargetVersion string
	CheckTime     string
}

func (m *UpdatePlatformManager) getUpdateTarget() string {
	target := &updateTarget{
		TargetVersion: m.targetBaseline,
		CheckTime:     m.checkTime,
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
}

// 进行安装更新前，需要复制文件替换软连接
func (m *UpdatePlatformManager) replaceVersionCache() {
	err := os.RemoveAll(cacheVersion)
	if err != nil {
		logger.Warning(err)
	}
	copyFile(realVersion, cacheVersion)
}

// 安装更新并检查完成后，需要用软连接替换文件
func (m *UpdatePlatformManager) recoverVersionLink() {
	err := os.RemoveAll(cacheVersion)
	if err != nil {
		logger.Warning(err)
	}
	err = os.Symlink(realVersion, cacheVersion)
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
		if value == "Enable" {
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
	Tp int `json:"tp"`

	Data interface {
	} `json:"data"`
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

// Report 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) Report(reqType requestType, msg string, token string) (data interface{}, err error) {
	// 设置请求url
	policyUrl := m.requestUrl + Urls[reqType].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	var sign string
	var xTime string
	var tarFilePath string
	var request *http.Request
	var body *bytes.Buffer
	body = bytes.NewBuffer([]byte{})
	// 设置请求参数
	switch reqType {
	case GetTargetPkgLists:
		values := url.Values{}
		values.Add("baseline", m.targetBaseline)
		policyUrl = policyUrl + "?" + values.Encode()
	case GetCurrentPkgLists:
		values := url.Values{}
		values.Add("baseline", m.preBaseline)
		policyUrl = policyUrl + "?" + values.Encode()
	case GetPkgCVEs:
		// values := url.Values{}
		// values.Add("synctime", m.config.LastCVESyncTime)
		// policyUrl = policyUrl + "?" + values.Encode()
	case GetUpdateLog:
		values := url.Values{}
		values.Add("baseline", m.targetBaseline)
		values.Add("isUnstable", fmt.Sprintf("%d", isUnstable()))
		policyUrl = policyUrl + "?" + values.Encode()
	case PostProcess:
		buf := bytes.NewBufferString(msg)
		tarFilePath = fmt.Sprintf("/tmp/%s_%s.xz", "update", time.Now().Format("20231019102233444"))
		xzFile, err := os.Create(tarFilePath)
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
		xTime = fmt.Sprintf("%d", time.Now().Unix())

		byt, err := ioutil.ReadFile(tarFilePath)
		if err != nil {
			logger.Warning("open xz file failed:", err)
			return nil, err
		}
		body = bytes.NewBuffer(byt)

		hash.Write([]byte(fmt.Sprintf("%s%s%s", secret, xTime, byt)))
		sign = base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(hash.Sum(nil))))
	}
	request, err = http.NewRequest(Urls[reqType].method, policyUrl, body)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", reqType.string(), err.Error())
	}

	// 设置header
	if reqType == PostProcess {
		// 如果是更新过程日志上报，设置header
		hardwareId, err := getHardwareId()
		if err != nil {
			return nil, fmt.Errorf("%v failed to get hardware id: %v ", reqType.string(), err.Error())
		}

		request.Header.Set("X-MachineID", hardwareId)
		request.Header.Set("X-CurrentBaseline", m.preBaseline)
		request.Header.Set("X-Baseline", m.targetBaseline)
		request.Header.Set("X-Time", xTime)
		request.Header.Set("X-Sign", sign)
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%v failed to do request: %v ", reqType.string(), err.Error())
	}
	defer func() {
		_ = response.Body.Close()
	}()
	var respData []byte

	switch response.StatusCode {
	case http.StatusOK:
		respData, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("%v failed to read response body: %v ", reqType.string(), err.Error())
		}
		if reqType == GetVersion {
			logger.Infof("%v request for %s ,body:%s respData:%s ", reqType.string(), policyUrl, msg, string(respData))
		}
		msg := &tokenMessage{}
		err = json.Unmarshal(respData, msg)
		if err != nil {
			return nil, fmt.Errorf("%v failed to Unmarshal respData to tokenMessage: %v ", reqType.string(), err.Error())
		}
		if !msg.Result {
			errorMsg := &tokenErrorMessage{}
			err = json.Unmarshal(respData, errorMsg)
			if err != nil {
				return nil, fmt.Errorf("%v failed to Unmarshal respData to tokenErrorMessage: %v ", reqType.string(), err.Error())
			}
			return nil, fmt.Errorf("%v request for %s err:%s", reqType.string(), policyUrl, errorMsg.Msg)
		}
		switch reqType {
		case GetVersion:
			tmp := updateMessage{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, fmt.Errorf("%v failed to Unmarshal msg.Data to updateMessage: %v ", reqType.string(), err.Error())
			}
			data = tmp
		case GetTargetPkgLists:
			if logger.GetLogLevel() == log.LevelDebug {
				ioutil.WriteFile("/tmp/platform-pkglist", respData, 0644)
			}
			tmp := PreInstalledPkgMeta{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, fmt.Errorf("%v failed to Unmarshal msg.Data to PreInstalledPkgMeta: %v ", reqType.string(), err.Error())
			}
			data = tmp
		case GetCurrentPkgLists:
			tmp := PreInstalledPkgMeta{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, fmt.Errorf("%v failed to Unmarshal msg.Data to PreInstalledPkgMeta: %v ", reqType.string(), err.Error())
			}
			data = tmp
		case GetPkgCVEs:
			tmp := CEVMeta{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, fmt.Errorf("%v failed to Unmarshal msg.Data to CEVMeta: %v ", reqType.string(), err.Error())
			}
			data = tmp
		case GetUpdateLog:
			var tmp []UpdateLogMeta
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, fmt.Errorf("%v failed to Unmarshal msg.Data to UpdateLogMeta: %v ", reqType.string(), err.Error())
			}
			data = tmp
		case PostProcess:
			return
		default:
			return nil, fmt.Errorf("unknown report type:%d", reqType)
		}
	default:
		err = fmt.Errorf("request for %s failed, response code=%d", policyUrl, response.StatusCode)
	}
	return
}

// 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) genUpdatePolicyByToken() error {
	data, err := m.Report(GetVersion, "", m.token)
	if err != nil {
		return err
	}
	msg, ok := data.(updateMessage)
	if !ok {
		return errors.New("failed convert to updateMessage")
	}
	m.targetBaseline = msg.Version.Baseline
	m.targetVersion = msg.Version.Version
	m.systemTypeFromPlatform = msg.SystemType
	m.repoInfos = msg.RepoInfos
	m.checkTime = time.Now().String()
	m.UpdateBaselineCache()

	// 生成仓库和InRelease
	m.genDepositoryFromPlatform()
	m.checkInReleaseFromPlatform()

	return nil

}

type packageLists struct {
	Core   []system.PlatformPackageInfo `json:"core"`   // "必须安装软件包清单"
	Select []system.PlatformPackageInfo `json:"select"` // "可选软件包清单"
	Freeze []system.PlatformPackageInfo `json:"freeze"` // "禁止升级包清单"
	Purge  []system.PlatformPackageInfo `json:"purge"`  // "删除软件包清单"
}

type PreInstalledPkgMeta struct {
	PreCheck  string       `json:"preCheck"`  // "更新前检查脚本"
	MidCheck  string       `json:"midCheck"`  // "更新中检查"
	PostCheck string       `json:"postCheck"` // "更新后检查"
	Packages  packageLists `json:"packages"`  // "基线软件包清单"
}

// 从更新平台获取升级目标版本的软件包清单
func (m *UpdatePlatformManager) updateTargetPkgMetaSync() error {
	data, err := m.Report(GetTargetPkgLists, "", m.token)
	if err != nil {
		return err
	}
	pkgs, ok := data.(PreInstalledPkgMeta)
	if !ok {
		return errors.New("failed convert to PreInstalledPkgMeta")
	}
	m.preCheck = pkgs.PreCheck
	m.midCheck = pkgs.MidCheck
	m.postCheck = pkgs.PostCheck

	if logger.GetLogLevel() == log.LevelDebug {
		m.targetCorePkgs["deepin-camera"] = system.PackageInfo{
			Name:    "deepin-camera",
			Version: "1.4.13-1",
			Need:    "strict",
		}
	} else {
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
				m.targetCorePkgs[pkg.Name] = system.PackageInfo{
					Name:    pkg.Name,
					Need:    pkg.Need,
					Version: version,
				}
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
			m.selectPkgs[pkg.Name] = system.PackageInfo{
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
			m.freezePkgs[pkg.Name] = system.PackageInfo{
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
			m.purgePkgs[pkg.Name] = system.PackageInfo{
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
	data, err := m.Report(GetCurrentPkgLists, "", m.token)
	if err != nil {
		return err
	}
	pkgs, ok := data.(PreInstalledPkgMeta)
	if !ok {
		logger.Warning("bad format")
		return err
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
			m.baselinePkgs[pkg.Name] = system.PackageInfo{
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

type CEVMeta struct {
	DateTime string    `json:"dateTime"`
	Cves     []CEVInfo `json:"cves"`
}

var CVEs map[string]CEVInfo // 保存全局cves信息，方便查询

// 从更新平台获取CVE元数据
func (m *UpdatePlatformManager) updateCVEMetaDataSync() error {
	data, err := m.Report(GetPkgCVEs, "", m.token)
	if err != nil {
		return err
	}
	cves, ok := data.(CEVMeta)
	if !ok {
		return errors.New("failed convert to CEVMeta")
	}
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

	_ = m.config.UpdateLastCVESyncTime(m.cveDataTime)
	return nil
}

func (m *UpdatePlatformManager) GetSystemMeta() map[string]system.PackageInfo {
	infos := make(map[string]system.PackageInfo)
	for name, info := range m.targetCorePkgs {
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
	data, err := m.Report(GetUpdateLog, "", m.token)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	var ok bool
	m.systemUpdateLogs, ok = data.([]UpdateLogMeta)
	if !ok {
		logger.Warning("failed convert to UpdateLogMeta")
		return nil
	}
	return nil
}

func (m *UpdatePlatformManager) genDepositoryFromPlatform() {
	prefix := "deb"
	suffix := "main contrib non-free"
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
	err := ioutil.WriteFile(system.PlatFormSourceFile, []byte(strings.Join(repos, "\n")), 0644)
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
					infos, err := ioutil.ReadDir(system.OnlineListPath)
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

			request.Header.Set("X-Repo-Token", m.token)
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

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logger.Warning(err)
				return
			}

			file = strings.ReplaceAll(file, "/", "_")
			lastoreFile := "/tmp/" + file
			aptFile := filepath.Join(system.OnlineListPath, file)

			err = ioutil.WriteFile(lastoreFile, data, 0644)
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
	syncFuncList := []func() error{
		m.updateLogMetaSync,                    // 日志
		m.updateTargetPkgMetaSync,              // 目标版本信息
		m.updateCurrentPreInstalledPkgMetaSync, // 基线版本信息
		m.updateCVEMetaDataSync,                // cve信息
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

// postStatusMessage 将检查\下载\安装过程中所有异常状态和每个阶段成功的正常状态上报
func (m *UpdatePlatformManager) postStatusMessage(body string) {
	logger.Debug("post msg:", body)
	_, err := m.Report(PostProcess, body, m.token)
	if err != nil {
		logger.Warning(err)
		return
	}
}

// 更新平台上报

type upgradePostContent struct {
	SerialNumber    string   `json:"serialNumber"`
	MachineID       string   `json:"machineId"`
	UpgradeStatus   int      `json:"status"`
	UpgradeErrorMsg string   `json:"msg"`
	TimeStamp       int64    `json:"timestamp"`
	SourceUrl       []string `json:"sourceUrl"`
	Version         string   `json:"version"`

	PreBuild        string `json:"preBuild"`
	NextShowVersion string `json:"nextShowVersion"`
	PreBaseline     string `json:"preBaseline"`
	NextBaseline    string `json:"nextBaseline"`
}

const (
	upgradeSucceed = 0
	upgradeFailed  = 1
)

func (m *UpdatePlatformManager) needPostSystemUpgradeMessage(mode system.UpdateType) bool {
	var editionName string
	infoMap, err := getOSVersionInfo(cacheVersion)
	if err != nil {
		logger.Warning(err)
	} else {
		editionName = infoMap["EditionName"]
	}
	return strv.Strv(m.config.AllowPostSystemUpgradeMessageVersion).Contains(editionName) && ((mode & m.allowPostSystemUpgradeMessageType) != 0)
}

// 发送系统更新成功或失败的状态
func (m *UpdatePlatformManager) postSystemUpgradeMessage(upgradeStatus int, j *Job, updateType system.UpdateType) {
	if !m.needPostSystemUpgradeMessage(updateType) {
		return
	}
	// updateType &= m.allowPostSystemUpgradeMessageType
	var upgradeErrorMsg string
	if upgradeStatus == upgradeFailed && j != nil {
		upgradeErrorMsg = j.Description
	}
	hardwareId, err := getHardwareId()
	if err != nil {
		logger.Warning(err)
	}

	postContent := &upgradePostContent{
		MachineID:       hardwareId,
		UpgradeStatus:   upgradeStatus,
		UpgradeErrorMsg: upgradeErrorMsg,
		TimeStamp:       time.Now().Unix(),
		PreBuild:        m.preBuild,
		NextShowVersion: m.targetVersion,
		PreBaseline:     m.preBaseline,
		NextBaseline:    m.targetBaseline,
	}
	content, err := json.Marshal(postContent)
	if err != nil {
		logger.Warning(err)
		return
	}
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	logger.Debug(postContent)
	encryptMsg, err := EncryptMsg(content)
	if err != nil {
		logger.Warning(err)
		return
	}
	base64EncodeString := base64.StdEncoding.EncodeToString(encryptMsg)
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
		body, _ := ioutil.ReadAll(response.Body)
		logger.Info(string(body))
	} else {
		logger.Warning(err)
	}
}

func (m *UpdatePlatformManager) getRules() []dut.RuleInfo {
	defaultCmd := "echo default rules"
	var rules []dut.RuleInfo

	if len(strings.TrimSpace(m.preCheck)) == 0 {
		rules = append(rules, dut.RuleInfo{
			Name:    "00_precheck",
			Type:    dut.PreCheck,
			Command: defaultCmd,
			Argv:    "",
		})
	} else {
		rules = append(rules, dut.RuleInfo{
			Name:    "00_precheck",
			Type:    dut.PreCheck,
			Command: m.preCheck,
			Argv:    "",
		})
	}

	if len(strings.TrimSpace(m.midCheck)) == 0 {
		rules = append(rules, dut.RuleInfo{
			Name:    "10_midcheck",
			Type:    dut.MidCheck,
			Command: defaultCmd,
			Argv:    "",
		})
	} else {
		rules = append(rules, dut.RuleInfo{
			Name:    "10_midcheck",
			Type:    dut.MidCheck,
			Command: m.midCheck,
			Argv:    "",
		})
	}

	if len(strings.TrimSpace(m.postCheck)) == 0 {
		rules = append(rules, dut.RuleInfo{
			Name:    "20_postcheck",
			Type:    dut.PostCheck,
			Command: defaultCmd,
			Argv:    "",
		})
	} else {
		rules = append(rules, dut.RuleInfo{
			Name:    "20_postcheck",
			Type:    dut.PostCheck,
			Command: m.postCheck,
			Argv:    "",
		})
	}
	return rules
}

// 埋点数据上报

type reportCategory uint32

const (
	updateStatusReport reportCategory = iota
	downloadStatusReport
	upgradeStatusReport
)

type reportLogInfo struct {
	Tid    int
	Result bool
	Reason string
}

// 数据埋点接口
func (m *UpdatePlatformManager) reportLog(category reportCategory, status bool, description string) {
	agent := m.userAgents.getActiveLastoreAgent()
	if agent != nil {
		logInfo := reportLogInfo{
			Result: status,
			Reason: description,
		}
		switch category {
		case updateStatusReport:
			logInfo.Tid = 1000600002
		case downloadStatusReport:
			logInfo.Tid = 1000600003
		case upgradeStatusReport:
			logInfo.Tid = 1000600004
		}
		infoContent, err := json.Marshal(logInfo)
		if err != nil {
			logger.Warning(err)
		}
		err = agent.ReportLog(0, string(infoContent))
		if err != nil {
			logger.Warning(err)
		}
	}
}
