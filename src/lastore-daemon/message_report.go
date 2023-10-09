// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/base64"
	"encoding/json"
	"internal/system"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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
	requestUrl             string
}

// 需要注意cache文件的同步时机，所有数据应该不会从os-version和os-baseline获取
const (
	cacheVersion  = "/var/lib/lastore/os-version.b"
	cacheBaseline = "/var/lib/lastore/os-baseline.b"
	realBaseline  = "/etc/os-baseline"
	realVersion   = "/etc/os-version"
)

func newUpdatePlatformManager(c *Config, agents *userAgentMap) *UpdatePlatformManager {
	url := os.Getenv("UPDATE_PLATFORM_URL")
	if len(url) == 0 {
		url = "https://update-platform.uniontech.com"
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

type version struct {
	Version  string `json:"version"`
	Baseline string `json:"baseline"`
}

type policy struct {
	Tp int `json:"tp"`

	Data struct {
	} `json:"data"`
}

type updateMessage struct {
	SystemType string  `json:"systemType"`
	Version    version `json:"version"`
	Policy     policy  `json:"policy"`
}

type tokenMessage struct {
	Result bool          `json:"result"`
	Code   int           `json:"code"`
	Data   updateMessage `json:"data"`
}
type tokenErrorMessage struct {
	Result bool   `json:"result"`
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

// 检查更新时将token数据发送给更新平台，获取本次更新信息
func (m *UpdatePlatformManager) genUpdatePolicyByToken() bool {
	policyUrl := m.requestUrl + "/api/v1/version"
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	if logger.GetLogLevel() == log.LevelDebug {
		logger.Info("debug mode,default target is 1062")
		time.Sleep(4 * time.Second)
		m.targetBaseline = "106x_debug"
		m.targetVersion = "1062"
		m.systemTypeFromPlatform = "professional"
		m.checkTime = time.Now().String()
		return true
	}
	request, err := http.NewRequest("GET", policyUrl, nil)
	if err != nil {
		logger.Warning(err)
		return false
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateTokenConfigFile())))
	response, err := client.Do(request)
	if err == nil {
		defer func() {
			_ = response.Body.Close()
		}()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			logger.Warning(err)
			return false
		}
		if response.StatusCode == 200 {
			logger.Debug(string(body))
			msg := &tokenMessage{}
			err = json.Unmarshal(body, msg)
			if err != nil {
				logger.Warning(err)
				return false
			}
			if !msg.Result {
				errorMsg := &tokenErrorMessage{}
				err = json.Unmarshal(body, errorMsg)
				if err != nil {
					logger.Warning(err)
					return false
				}
				logger.Warning(errorMsg.Msg)
				return false
			}
			m.targetBaseline = msg.Data.Version.Baseline
			m.targetVersion = msg.Data.Version.Version
			m.systemTypeFromPlatform = msg.Data.SystemType
			m.checkTime = time.Now().String()
			m.UpdateBaselineCache()
			return true
		}
		logger.Warning(string(body))
		return false
	} else {
		logger.Warning(err)
		return false
	}
}

// TODO 更新平台数据处理

type PreInstalledPkgMeta struct {
}

// 从更新平台获取当前基线版本预装列表,暂不缓存至本地;
func (m *UpdatePlatformManager) updateCurrentPreInstalledPkgMetaSync() error {
	return nil
}

type CEVMeta struct {
}

// 从更新平台获取CVE元数据
func (m *UpdatePlatformManager) updateCVEMetaDataSync() error {
	return nil
}

type SystemMeta struct {
}

func (m *UpdatePlatformManager) updateSystemMetaSync() error {
	return nil
}

func (m *UpdatePlatformManager) GetSystemMeta() map[string]system.UpgradeInfo {
	if logger.GetLogLevel() == log.LevelDebug {
		r := make(map[string]system.UpgradeInfo)
		r["deepin-camera"] = system.UpgradeInfo{
			Package:        "deepin-camera",
			CurrentVersion: "",
			LastVersion:    "1.4.16-1",
			ChangeLog:      "",
			Category:       "",
		}
		r["dde-launcher"] = system.UpgradeInfo{
			Package:        "dde-launcher",
			CurrentVersion: "",
			LastVersion:    "5.6.10-1",
			ChangeLog:      "",
			Category:       "",
		}
		return r
	}
	return nil
}

type ChangelogMeta struct {
}

// 如果更新日志无法获取到,不会返回错误,而是设置默认日志文案
func (m *UpdatePlatformManager) updateChangelogMetaSync() error {
	return nil
}

func (m *UpdatePlatformManager) getDepositoryFromPlatform() []string {
	if logger.GetLogLevel() == log.LevelDebug {
		return []string{
			"deb http://pools.uniontech.com/ppa/dde-eagle eagle/1061 main contrib non-free",
		}
	}
	return nil
}

// UpdatePlatFormSourceListFile 更新从平台获取的仓库
func (m *UpdatePlatformManager) UpdatePlatFormSourceListFile() error {
	return ioutil.WriteFile(system.PlatFormSourceFile, []byte(strings.Join(m.getDepositoryFromPlatform(), "\n")), 0644)
}

// UpdateAllPlatformDataSync 同步获取所有需要从更新平台获取的数据
func (m *UpdatePlatformManager) UpdateAllPlatformDataSync() error {
	var wg sync.WaitGroup
	var errGlobal error
	syncFuncList := []func() error{
		m.updateCurrentPreInstalledPkgMetaSync,
		m.updateCVEMetaDataSync,
		m.updateSystemMetaSync,
		m.updateChangelogMetaSync,
	}
	for _, syncFunc := range syncFuncList {
		f := syncFunc
		go func() {
			wg.Add(1)
			err := f()
			if err != nil {
				logger.Warning(err)
				errGlobal = err
			}
			wg.Done()
		}()

	}
	wg.Wait()
	return errGlobal
}

// PostStatusMessage 将检查\下载\安装过程中所有异常状态和每个阶段成功的正常状态上报
func (m *UpdatePlatformManager) PostStatusMessage() {

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
	url := m.requestUrl + "/api/v1/update/status"
	request, err := http.NewRequest("POST", url, strings.NewReader(base64EncodeString))
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
