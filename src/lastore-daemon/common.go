// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"internal/system"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/strv"
)

var _tokenUpdateMu sync.Mutex

// 更新 99lastore-token.conf 文件的内容
func updateTokenConfigFile() string {
	logger.Debug("start updateTokenConfigFile")
	_tokenUpdateMu.Lock()
	defer _tokenUpdateMu.Unlock()
	systemInfo := getSystemInfo()
	tokenPath := path.Join(aptConfDir, tokenConfFileName)
	var tokenSlice []string
	tokenSlice = append(tokenSlice, "a="+systemInfo.SystemName)
	tokenSlice = append(tokenSlice, "b="+systemInfo.ProductType)
	tokenSlice = append(tokenSlice, "c="+systemInfo.EditionName)
	tokenSlice = append(tokenSlice, "v="+systemInfo.Version)
	tokenSlice = append(tokenSlice, "i="+systemInfo.HardwareId)
	tokenSlice = append(tokenSlice, "m="+systemInfo.Processor)
	tokenSlice = append(tokenSlice, "ac="+systemInfo.Arch)
	tokenSlice = append(tokenSlice, "cu="+systemInfo.Custom)
	tokenSlice = append(tokenSlice, "sn="+systemInfo.SN)
	tokenSlice = append(tokenSlice, "vs="+systemInfo.HardwareVersion)
	tokenSlice = append(tokenSlice, "oid="+systemInfo.OEMID)
	tokenSlice = append(tokenSlice, "pid="+systemInfo.ProjectId)
	tokenSlice = append(tokenSlice, "baseline="+systemInfo.Baseline)
	tokenSlice = append(tokenSlice, "st="+systemInfo.SystemType)
	token := strings.Join(tokenSlice, ";")
	token = strings.Replace(token, "\n", "", -1)
	tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	err := ioutil.WriteFile(tokenPath, tokenContent, 0644) // #nosec G306
	if err != nil {
		logger.Warning(err)
	}
	// TODO: 使用教育版token，获取仓库
	if logger.GetLogLevel() == log.LevelDebug {
		token = "a=edu-20-std;b=Desktop;c=E;v=20.1060.11068.101.100;i=905923cfb835f3649e79fa90b28dad6fa973425a12d1b6a2ebd3dcf4a52eab92;m=Hygon C86 3250 8-core Processor;ac=amd64;cu=0;sn=N9DA5MAAAFPSL66NBNEAAVS5G;vs=Dhyana+;oid=f1800c30-ceb6-58a4-bcb2-0e4a565947a6;pid=;baseline=edu-20-std-0002;st="
	}
	return token
}

var _urlReg = regexp.MustCompile(`^[ ]*deb .*((?:https?|ftp|file)://[^ ]+)`)

// 获取list文件或list.d文件夹中所有list文件的未被屏蔽的仓库地址
func getUpgradeUrls(path string) []string {
	var upgradeUrls []string
	info, err := os.Stat(path)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	if info.IsDir() {
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			logger.Warning(err)
			return nil
		}
		for _, info := range infos {
			upgradeUrls = append(upgradeUrls, getUpgradeUrls(filepath.Join(path, info.Name()))...)
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			logger.Warning(err)
			return nil
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				logger.Warning(err)
			}
		}(f)
		r := bufio.NewReader(f)
		for {
			s, err := r.ReadString('\n')
			if err != nil {
				break
			}
			allMatchedString := _urlReg.FindAllStringSubmatch(s, -1)
			for _, MatchedString := range allMatchedString {
				if len(MatchedString) == 2 {
					upgradeUrls = append(upgradeUrls, MatchedString[1])
				}
			}
		}
	}
	return upgradeUrls
}

var pkgNameRegexp = regexp.MustCompile(`^[a-z0-9]`)

func NormalizePackageNames(s string) ([]string, error) {
	pkgNames := strings.Fields(s)
	for _, pkgName := range pkgNames {
		if !pkgNameRegexp.MatchString(pkgName) {
			return nil, fmt.Errorf("invalid package name %q", pkgName)
		}
	}

	if s == "" || len(pkgNames) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	return pkgNames, nil
}

// makeEnvironWithSender 从sender获取 DISPLAY XAUTHORITY DEEPIN_LASTORE_LANG环境变量,从manager的agent获取系统代理(手动)的环境变量
func makeEnvironWithSender(m *Manager, sender dbus.Sender) (map[string]string, error) {
	environ := make(map[string]string)
	var err error
	agent := m.userAgents.getActiveLastoreAgent()
	if agent != nil {
		environ, err = agent.GetManualProxy(0)
		if err != nil {
			logger.Warning(err)
			environ = make(map[string]string)
		}
	}
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		return nil, err
	}
	p := procfs.Process(pid)
	envVars, err := p.Environ()
	if err != nil {
		logger.Warningf("failed to get process %d environ: %v", p, err)
	} else {
		environ["DISPLAY"] = envVars.Get("DISPLAY")
		environ["XAUTHORITY"] = envVars.Get("XAUTHORITY")
		environ["DEEPIN_LASTORE_LANG"] = getLang(envVars)
	}
	return environ, nil
}

func getUsedLang(environ map[string]string) string {
	return environ["DEEPIN_LASTORE_LANG"]
}

func getLang(envVars procfs.EnvVars) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGE", "LANG"} {
		value := envVars.Get(name)
		if value != "" {
			return value
		}
	}
	return ""
}

func listPackageDesktopFiles(pkg string) []string {
	var result []string
	filenames := system.ListPackageFile(pkg)
	for _, filename := range filenames {
		if strings.HasPrefix(filename, "/usr/") {
			// len /usr/ is 5
			if strings.HasSuffix(filename, ".desktop") &&
				(strings.HasPrefix(filename[5:], "share/applications") ||
					strings.HasPrefix(filename[5:], "local/share/applications")) {

				fileInfo, err := os.Stat(filename)
				if err != nil {
					continue
				}
				if fileInfo.IsDir() {
					continue
				}
				if !utf8.ValidString(filename) {
					continue
				}
				result = append(result, filename)
			}
		}
	}
	return result
}

func getArchiveInfo() (string, error) {
	out, err := exec.Command("/usr/bin/lastore-apt-clean", "-print-json").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getNeedCleanCacheSize() (float64, error) {
	output, err := exec.Command("/usr/bin/lastore-apt-clean", "-print-json").Output()
	if err != nil {
		return 0, err
	}
	var archivesInfo map[string]json.RawMessage
	err = json.Unmarshal(output, &archivesInfo)
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseFloat(string(archivesInfo["total"]), 64)
	if err != nil {
		return 0, err
	}
	return size, nil
}

var _securityConfigUpdateMu sync.Mutex

// 在控制中心打开仅安全更新时,在apt配置文件中增加参数,用户使用命令行安装更新时,也同样仅会进行安全更新
func updateSecurityConfigFile(create bool) error {
	_securityConfigUpdateMu.Lock()
	defer _securityConfigUpdateMu.Unlock()
	configPath := path.Join(aptConfDir, securityConfFileName)
	if create {
		_, err := os.Stat(configPath)
		if err == nil {
			return nil
		}
		configContent := []string{
			`Dir::Etc::SourceParts "/dev/null";`,
			fmt.Sprintf(`Dir::Etc::SourceList "/etc/apt/sources.list.d/%v";`, system.SecurityList),
		}
		config := strings.Join(configContent, "\n")
		err = ioutil.WriteFile(configPath, []byte(config), 0644)
		if err != nil {
			return err
		}
	} else {
		err := os.RemoveAll(configPath)
		if err != nil {
			return err
		}
	}
	return nil
}

const autoDownloadTimeLayout = "15:04"

var _minDelayTime = 10 * time.Second

// getCustomTimeDuration 按照autoDownloadTimeLayout的格式计算时间差
func getCustomTimeDuration(presetTime string) time.Duration {
	presetTimer, err := time.Parse(autoDownloadTimeLayout, presetTime)
	if err != nil {
		logger.Warning(err)
		return _minDelayTime
	}
	var timeStr string
	if time.Now().Minute() < 10 {
		timeStr = fmt.Sprintf("%v:0%v", time.Now().Hour(), time.Now().Minute())
	} else {
		timeStr = fmt.Sprintf("%v:%v", time.Now().Hour(), time.Now().Minute())
	}
	nowTimer, err := time.Parse(autoDownloadTimeLayout, timeStr)
	if err != nil {
		logger.Warning(err)
		return _minDelayTime
	}
	dur := presetTimer.Sub(nowTimer)
	if dur <= 0 {
		dur += 24 * time.Hour
	}
	if dur < _minDelayTime {
		dur = _minDelayTime
	}
	return dur
}

const (
	appStoreDaemonPath    = "/usr/bin/deepin-app-store-daemon"
	oldAppStoreDaemonPath = "/usr/bin/deepin-appstore-daemon"
	printerPath           = "/usr/bin/dde-printer"
	printerHelperPath     = "/usr/bin/dde-printer-helper"
	sessionDaemonPath     = "/usr/lib/deepin-daemon/dde-session-daemon"
	langSelectorPath      = "/usr/lib/deepin-daemon/langselector"
	controlCenterPath     = "/usr/bin/dde-control-center"
	controlCenterCmdLine  = "/usr/share/applications/dde-control-center.deskto" // 缺个 p 是因为 deepin-turbo 修改命令的时候 buffer 不够用, 所以截断了.
)

var (
	allowInstallPackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		printerPath,
		printerHelperPath,
		langSelectorPath,
		controlCenterPath,
	}
	allowRemovePackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		sessionDaemonPath,
		langSelectorPath,
		controlCenterPath,
	}
)

// execPath和cmdLine可以有一个为空,其中一个存在即可作为判断调用者的依据
func getExecutablePathAndCmdline(service *dbusutil.Service, sender dbus.Sender) (string, string, error) {
	pid, err := service.GetConnPID(string(sender))
	if err != nil {
		return "", "", err
	}

	proc := procfs.Process(pid)

	execPath, err := proc.Exe()
	if err != nil {
		// 当调用者在使用过程中发生了更新,则在获取该进程的exe时,会出现lstat xxx (deleted)此类的error,如果发生的是覆盖,则该路径依旧存在,因此增加以下判断
		pErr, ok := err.(*os.PathError)
		if ok {
			if os.IsNotExist(pErr.Err) {
				errExecPath := strings.Replace(pErr.Path, "(deleted)", "", -1)
				oldExecPath := strings.TrimSpace(errExecPath)
				if system.NormalFileExists(oldExecPath) {
					execPath = oldExecPath
					err = nil
				}
			}
		}
	}

	cmdLine, err1 := proc.Cmdline()
	if err != nil && err1 != nil {
		return "", "", errors.New(strings.Join([]string{
			err.Error(),
			err1.Error(),
		}, ";"))
	}
	return execPath, strings.Join(cmdLine, " "), nil
}

// 根据类型过滤数据
func getFilterPackages(infosMap map[string][]string, updateType system.UpdateType) []string {
	var r []string
	for _, t := range system.AllInstallUpdateType() {
		if updateType&t != 0 {
			info, ok := infosMap[t.JobType()]
			if ok {
				r = append(r, info...)
			}
		}
	}
	return r
}

// SystemUpgradeInfo 将update_infos.json数据解析成map
func SystemUpgradeInfo() (map[string][]system.UpgradeInfo, error) {
	r := make(system.SourceUpgradeInfoMap)

	filename := path.Join(system.VarLibDir, "update_infos.json")
	var updateInfosList []system.UpgradeInfo
	err := system.DecodeJson(filename, &updateInfosList)
	if err != nil {
		if os.IsNotExist(err) {
			outputErrorPath := fmt.Sprintf("error_%v", "update_infos.json")
			filename = path.Join(system.VarLibDir, outputErrorPath)
			if system.NormalFileExists(filename) {
				var updateInfoErr system.UpdateInfoError
				err2 := system.DecodeJson(filename, &updateInfoErr)
				if err2 == nil {
					return nil, &updateInfoErr
				}
				return nil, fmt.Errorf("Invalid update_infos: %v\n", err)
			}
			return nil, err
		}
	}
	for _, info := range updateInfosList {
		r[info.Category] = append(r[info.Category], info)
	}
	return r, nil
}

func cleanAllCache() {
	err := exec.Command("apt-get", "clean", "-c", system.LastoreAptV2CommonConfPath).Run()
	if err != nil {
		logger.Warning(err)
	}
}

const aptLimitKey = "Acquire::http::Dl-Limit"
