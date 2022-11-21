// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"encoding/json"
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
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/procfs"
)

var _tokenUpdateMu sync.Mutex

// 更新 99lastore-token.conf 文件的内容
func updateTokenConfigFile() {
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
	token := strings.Join(tokenSlice, ";")
	token = strings.Replace(token, "\n", "", -1)
	tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	err := ioutil.WriteFile(tokenPath, tokenContent, 0644) // #nosec G306
	if err != nil {
		logger.Warning(err)
	}
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

func getEditionName() string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/os-version")
	// 为避免收集数据的风险，读不到此文件，或者Edition文件不存在也不收集数据
	if err != nil {
		return ""
	}
	edition, err := kf.GetString("Version", "EditionName")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(edition)
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
	t1, err := time.Parse(autoDownloadTimeLayout, presetTime)
	if err != nil {
		logger.Warning(err)
		return _minDelayTime
	}
	t2, err := time.Parse(autoDownloadTimeLayout, fmt.Sprintf("%v:%v", time.Now().Hour(), time.Now().Minute()))
	if err != nil {
		logger.Warning(err)
		return _minDelayTime
	}
	dur := t1.Sub(t2)
	if dur < 0 {
		dur += 24 * time.Hour
	}
	return dur
}
