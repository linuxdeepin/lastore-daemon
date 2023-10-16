// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"internal/system"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/keyfile"
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

	preCheck   string                        // 更新前检查脚本
	midCheck   string                        // 更新中检查脚本
	postCheck  string                        // 更新后检查脚本
	corePkgs   map[string]system.UpgradeInfo // 必须安装软件包信息清单
	selectPkgs map[string]system.UpgradeInfo // 可选软件包清单
	freezePkgs map[string]packageInfo        // 禁止升级包清单
	purgePkgs  map[string]packageInfo        // 删除软件包清单

	repoInfos        []repoInfo      // 从更新平台获取的仓库信息
	systemUpdataLogs []UpdatelogMeta // 更新注记
	cveDataTime      string
	cvePkgs          map[string][]string // cve信息 pkgname:[cveid...]
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

func (m *UpdatePlatformManager) GetSystemUpdataLogs() []string {
	var updataLogs []string
	zh := isZH()

	var logStr string
	for _, updateLog := range m.systemUpdataLogs {
		if zh {
			logStr = updateLog.CnLog
		} else {
			logStr = updateLog.EnLog
		}
		updataLogs = append(updataLogs, logStr)
	}

	sort.Strings(updataLogs)
	return updataLogs
}

func (m *UpdatePlatformManager) GetCVEUpdataLogs(pkgs []string) []string {
	var cves map[string]string = make(map[string]string)
	var updataLogs []string
	zh := isZH()

	for _, pkg := range pkgs {
		for _, id := range m.cvePkgs[pkg] {
			if _, ok := cves[id]; ok {
				continue
			}
			if zh {
				cves[id] = CVEs[id].Description
			} else {
				cves[id] = CVEs[id].CveDescription
			}
			updataLogs = append(updataLogs, cves[id])
		}
	}

	sort.Strings(updataLogs)
	return updataLogs
}

func newUpdatePlatformManager(c *Config, agents *userAgentMap) *UpdatePlatformManager {
	url := os.Getenv("UPDATE_PLATFORM_URL")
	if len(url) == 0 {
		url = "https://update-platform-pre.uniontech.com"
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
	return &UpdatePlatformManager{
		config:                            c,
		userAgents:                        agents,
		allowPostSystemUpgradeMessageType: system.SystemUpdate,
		preBuild:                          genPreBuild(),
		preBaseline:                       getCurrentBaseline(),
		targetVersion:                     getTargetVersion(),
		targetBaseline:                    getTargetBaseline(),
		requestUrl:                        url,
		cvePkgs:                           make(map[string][]string),
	}
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
	RELEASE_VERSION  = 1
	UNSTABLE_VERSION = 2
)

func isUnstable() int {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return RELEASE_VERSION
	}
	ds := ConfigManager.NewConfigManager(sysBus)
	dsPath, err := ds.AcquireManager(0, "org.deepin.unstable", "org.deepin.unstable", "")
	if err != nil {
		logger.Warning(err)
		return RELEASE_VERSION
	}
	unstableManager, err := ConfigManager.NewManager(sysBus, dsPath)
	if err != nil {
		logger.Warning(err)
		return RELEASE_VERSION
	}
	v, err := unstableManager.Value(0, "updateUnstable")
	if err != nil {
		return RELEASE_VERSION
	} else {
		value := v.Value().(string)
		if value == "Enable" {
			return UNSTABLE_VERSION
		} else {
			return RELEASE_VERSION
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

const (
	GetVersion = iota
	GetUpdataLog
	GetPkgLists // 系统软件包清单
	GetPkgCVEs  // CVE 信息
	PostProcess
)

type requestType struct {
	path   string
	method string
}

var Urls = map[uint32]requestType{
	GetVersion: {
		"/api/v1/version",
		"GET",
	},
	GetPkgLists: {
		"/api/v1/package",
		"GET",
	},
	GetUpdataLog: {
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
}

// 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) Report(reqType uint32, body string) (data interface{}, err error) {
	// 设置请求url
	policyUrl := m.requestUrl + Urls[reqType].path
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	// 设置请求参数
	switch reqType {
	case GetPkgLists:
		values := url.Values{}
		values.Add("baseline", m.targetBaseline)
		policyUrl = policyUrl + "?" + values.Encode()
	case GetPkgCVEs:
		values := url.Values{}
		values.Add("synctime", m.config.LastCVESyncTime)
		policyUrl = policyUrl + "?" + values.Encode()
	case GetUpdataLog:
		values := url.Values{}
		values.Add("baseline", m.targetBaseline)
		values.Add("isUnstable", fmt.Sprintf("%d", isUnstable()))
		policyUrl = policyUrl + "?" + values.Encode()
	}

	request, err := http.NewRequest(Urls[reqType].method, policyUrl, bytes.NewBuffer([]byte(body)))
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	// 设置header
	if reqType == PostProcess {
		// 如果是更新过程日志上报，设置header
		hardwareId, err := getHardwareId()
		if err != nil {
			return nil, err
		}

		request.Header.Set("X-MachineID", hardwareId)
		request.Header.Set("X-CurrentBaseline", m.preBaseline)
		request.Header.Set("X-Baseline", m.targetBaseline)
		request.Header.Set("X-Time", fmt.Sprintf("%d", time.Now().Unix()))
		request.Header.Set("X-Sign", "TODO")
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateTokenConfigFile())))
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	var respData []byte

	switch response.StatusCode {
	case http.StatusOK:
		respData, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		// logger.Infof("request for %s,body:%s respData:%s", policyUrl, string(body), string(respData))
		msg := &tokenMessage{}
		err = json.Unmarshal(respData, msg)
		if err != nil {
			return nil, err
		}
		if !msg.Result {
			errorMsg := &tokenErrorMessage{}
			err = json.Unmarshal(respData, errorMsg)
			if err != nil {
				return nil, err
			}
			err = fmt.Errorf("request for %s err:%s", policyUrl, errorMsg.Msg)
			return nil, err
		}
		switch reqType {
		case GetVersion:
			tmp := updateMessage{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, err
			}
			data = tmp
		case GetPkgLists:
			tmp := PreInstalledPkgMeta{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, err
			}
			data = tmp
		case GetPkgCVEs:
			tmp := CEVMeta{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, err
			}
			data = tmp
		case GetUpdataLog:
			var tmp []UpdatelogMeta
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, err
			}
			data = tmp
		case PostProcess:
			return
		default:
			err = fmt.Errorf("unknown report type:%d", reqType)
			return
		}
	default:
		err = fmt.Errorf("request for %s failed, response code=%d", policyUrl, response.StatusCode)
	}
	return
}

// 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) genUpdatePolicyByToken() bool {
	data, err := m.Report(GetVersion, "")
	if err != nil {
		logger.Warning(err)
		return false
	}
	msg, ok := data.(updateMessage)
	if !ok {
		logger.Warning("bad format")
		return false
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

	return true

}

// TODO 更新平台数据处理

type packageInfo struct {
	Name    string `json:"name"`    // "软件包名"
	Version string `json:"version"` // "软件包版本"
}

type packageLists struct {
	Core   []packageInfo `json:"core"`   // "必须安装软件包清单"
	Select []packageInfo `json:"select"` // "可选软件包清单"
	Freeze []packageInfo `json:"freeze"` // "禁止升级包清单"
	Purge  []packageInfo `json:"purge"`  // "删除软件包清单"
}

type PreInstalledPkgMeta struct {
	PreCheck  string       `json:"preCheck"`  // "更新前检查脚本"
	MidCheck  string       `json:"midCheck"`  // "更新中检查"
	PostCheck string       `json:"postCheck"` // "更新后检查"
	Packages  packageLists `json:"packages"`  // "基线软件包清单"
}

// 从更新平台获取当前基线版本预装列表,暂不缓存至本地;
func (m *UpdatePlatformManager) updateCurrentPreInstalledPkgMetaSync() error {
	data, err := m.Report(GetPkgLists, "")
	if err != nil {
		logger.Warning(err)
		return err
	}
	pkgs, ok := data.(PreInstalledPkgMeta)
	if !ok {
		logger.Warning("bad format")
		return err
	}
	m.preCheck = pkgs.PreCheck
	m.midCheck = pkgs.MidCheck
	m.postCheck = pkgs.PostCheck

	if pkgs.Packages.Core != nil {
		for _, pkg := range pkgs.Packages.Core {
			m.corePkgs[pkg.Name] = system.UpgradeInfo{
				Package:        pkg.Name,
				CurrentVersion: "TODO",
				LastVersion:    pkg.Version,
				ChangeLog:      "TODO",
				Category:       "core",
			}
		}
		for _, pkg := range pkgs.Packages.Select {
			m.selectPkgs[pkg.Name] = system.UpgradeInfo{
				Package:        pkg.Name,
				CurrentVersion: "TODO",
				LastVersion:    pkg.Version,
				ChangeLog:      "TODO",
				Category:       "select",
			}
		}
		for _, pkg := range pkgs.Packages.Freeze {
			m.freezePkgs[pkg.Name] = packageInfo{
				Name:    pkg.Name,
				Version: pkg.Version,
			}
		}
		for _, pkg := range pkgs.Packages.Purge {
			m.purgePkgs[pkg.Name] = packageInfo{
				Name:    pkg.Name,
				Version: pkg.Version,
			}
		}
	}

	return nil
}

type CEVInfo struct {
	SyncTime       string `json:"synctime"`        // "CVE类型"
	CveId          string `json:"cveid"`           // "CVE编号"
	Source         string `json:"source"`          // "包名"
	FixedVersion   string `json:"fixed_version"`   // "修复版本"
	Archs          string `json:"archs"`           // "架构信息"
	Score          string `json:"score"`           // "评分"
	Status         string `json:"status"`          // "修复状态"
	VulCategory    string `json:"vul_category"`    // "漏洞类型"
	VulName        string `json:"vul_name"`        // "漏洞名称"
	VulLevel       string `json:"vul_level"`       // "⻛险等级"
	PubTime        string `json:"pub_time"`        // "CVE公开时间"
	Binary         string `json:"binary"`          // "二进制包"
	Description    string `json:"description"`     // "漏洞描述"
	CveDescription string `json:"cve_description"` // "漏洞描述(英文)"
}

type CEVMeta struct {
	DateTime string    `json:"dateTime"`
	Cves     []CEVInfo `json:"cves"`
}

var CVEs map[string]CEVInfo // 保存全局cves信息，方便查询

// 从更新平台获取CVE元数据
func (m *UpdatePlatformManager) updateCVEMetaDataSync() error {
	data, err := m.Report(GetPkgCVEs, "")
	if err != nil {
		logger.Warning(err)
		return err
	}
	cves, ok := data.(CEVMeta)
	if !ok {
		logger.Warning("bad format")
		return err
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

	m.config.UpdateLastCVESyncTime(m.cveDataTime)
	return nil
}

func (m *UpdatePlatformManager) GetSystemMeta() map[string]system.UpgradeInfo {
	infos := make(map[string]system.UpgradeInfo)
	for name, info := range m.corePkgs {
		infos[name] = info
	}

	for name, info := range m.selectPkgs {
		infos[name] = info
	}
	return infos
}

type UpdatelogMeta struct {
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
	data, err := m.Report(GetUpdataLog, "")
	if err != nil {
		logger.Warning(err)
		return err
	}
	var ok bool
	m.systemUpdataLogs, ok = data.([]UpdatelogMeta)
	if !ok {
		return err
	}
	return nil
}

func (m *UpdatePlatformManager) genDepositoryFromPlatform() {
	prefix := "deb"
	suffix := "main contrib non-free"
	var repos []string
	for _, repo := range m.repoInfos {
		codeName := repo.CodeName
		if repo.Version != "" {
			codeName = fmt.Sprintf("%s/%s", codeName, repo.Version)
		}
		// 如果有cdn，则使用cdn，效率更高
		var uri = repo.Uri
		if repo.Cdn != "" {
			uri = repo.Cdn
		}
		repos = append(repos, fmt.Sprintf("%s %s %s %s", prefix, uri, codeName, suffix))
	}

	err := ioutil.WriteFile(system.PlatFormSourceFile, []byte(strings.Join(repos, "\n")), 0644)
	if err != nil {
		logger.Warning("update sourcefile err")
	}

}

func getAptAuthConf(domain string) string {
	AuthFile := "/etc/apt/auth.conf.d/uos.conf"
	file, err := os.Open(AuthFile)
	if err != nil {
		logger.Warning("无法打开文件:", err)
		return ""
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
			auth := line[3] + ":" + line[5]
			auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
			return auth
		}
	}
	return ""
}

// 校验InRelease文件，如果平台和本地不同，则删除
func (m *UpdatePlatformManager) checkInReleaseFromPlatform() {
	// 更新获取InRelease文件
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	for _, repo := range m.repoInfos {
		// 如果有cdn，则使用cdn，效率更高
		var uri = repo.Uri
		if repo.Cdn != "" {
			uri = repo.Cdn
		}

		uri = fmt.Sprintf("%s/dists/%s/InRelease", uri, repo.CodeName)
		request, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			logger.Warning(err)
			return
		}

		// 获取仓库文件路径
		file := utils.URIToPath(uri)
		if len(file) == 0 {
			logger.Warning("unillegal uri:", repo.Uri)
		}
		// 获取域名
		domain := strings.Split(file, "/")[0]

		request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateTokenConfigFile())))
		request.Header.Set("Authorization", getAptAuthConf(domain))
		resp, err := client.Do(request)
		if err != nil {
			logger.Warning(err)
			continue
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		default:
			logger.Warningf("failed download InRelease:%s,respCode:%d", uri, resp.StatusCode)
			continue
		}

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Warning(err)
			continue
		}

		file = strings.ReplaceAll(file, "/", "_")
		lastoreFile := "/tmp/" + file
		aptFile := "/var/lib/apt/lists/" + file

		err = ioutil.WriteFile(lastoreFile, data, 0644)
		if err != nil {
			logger.Warning(err)
			continue
		}

		_, err = os.Stat(aptFile)
		// 文件存在，则校验MD5值
		if err == nil {
			aptSum, ok := utils.SumFileMd5(aptFile)
			if !ok {
				logger.Warningf("check %s md5sum failed", aptFile)
				continue
			}
			lastoreSum, ok := utils.SumFileMd5(lastoreFile)
			if !ok {
				logger.Warningf("check %s md5sum failed", lastoreFile)
				continue
			}
			if aptSum != lastoreSum {
				logger.Warning("InRelease changed:", aptFile)
				os.Remove(aptFile)
			} else {
				logger.Warningf("InRelease unchanged: %s", aptFile)
				continue
			}
		}
		// 文件不存在直接拷贝过去
		if os.IsNotExist(err) {
			logger.Warningf("failed check InRelease: %s ", aptFile)
			continue
		}
	}
}

// UpdateAllPlatformDataSync 同步获取所有需要从更新平台获取的数据
func (m *UpdatePlatformManager) UpdateAllPlatformDataSync() error {
	var wg sync.WaitGroup
	var errGlobal error
	syncFuncList := []func() error{
		m.updateLogMetaSync,
		m.updateCurrentPreInstalledPkgMetaSync,
		m.updateCVEMetaDataSync,
	}
	for _, syncFunc := range syncFuncList {
		wg.Add(1)
		go func(f func() error) {
			err := f()
			if err != nil {
				logger.Warning(err)
				errGlobal = err
			}
			wg.Done()
		}(syncFunc)

	}
	wg.Wait()
	return errGlobal
}

// PostStatusMessage 将检查\下载\安装过程中所有异常状态和每个阶段成功的正常状态上报
func (m *UpdatePlatformManager) PostStatusMessage(body string) {
	_, err := m.Report(PostProcess, body)
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
	logger.Debug(postContent)
	encryptMsg, err := EncryptMsg(content)
	if err != nil {
		logger.Warning(err)
		return
	}
	base64EncodeString := base64.StdEncoding.EncodeToString(encryptMsg)
	_, err = m.Report(PostProcess, base64EncodeString)
	if err != nil {
		logger.Warning(err)
		return
	}
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

func (m *UpdatePlatformManager) reportLog(category reportCategory, status bool, description string) {
	go func() {
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
	}()
}
