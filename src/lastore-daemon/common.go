// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"github.com/linuxdeepin/dde-api/polkit"
	utils2 "github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
	. "github.com/linuxdeepin/lastore-daemon/src/internal/config"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/strv"
)

var _urlReg = regexp.MustCompile(`^[ ]*deb .*((?:https?|ftp|file|p2p)://[^ ]+)`)

// 获取list文件或list.d文件夹中所有list文件的未被屏蔽的仓库地址
func getUpgradeUrls(path string) []string {
	var upgradeUrls []string
	info, err := os.Stat(path)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	if info.IsDir() {
		infos, err := os.ReadDir(path)
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
			allMatchedString := _urlReg.FindAllStringSubmatch(s, -1)
			for _, MatchedString := range allMatchedString {
				if len(MatchedString) == 2 {
					upgradeUrls = append(upgradeUrls, MatchedString[1])
				}
			}
			if err != nil {
				break
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
	uid, err := m.service.GetConnUID(string(sender))
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
		environ["PACKAGEKIT_CALLER_UID"] = fmt.Sprint(uid)
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

var originalLocaleEnvs []string

// collectAndClearLocaleEnvs collects current locale environment variables,
// clears them from the environment, and stores the original values for later use
func collectAndClearLocaleEnvs() {
	originalLocaleEnvs = nil
	// Iterate over the list of environment variable names related to locale settings
	for _, localeEnvName := range []string{"LC_ALL", "LANGUAGE",
		"LC_MESSAGES", "LANG"} {
		localeEnvValue, exists := os.LookupEnv(localeEnvName)
		if exists {
			// Store the original locale environment variables for later use
			originalLocaleEnvs = append(originalLocaleEnvs, localeEnvName+"="+localeEnvValue)
		}
		_ = utils.UnsetEnv(localeEnvName)
	}
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
	config := NewConfig(path.Join("/var/lib/lastore", "config.json"))
	isIncrementalUpdate := ""
	if config.IncrementalUpdate {
		isIncrementalUpdate = "--incremental-update"
	}
	out, err := exec.Command("/usr/bin/lastore-apt-clean", "-print-json", isIncrementalUpdate).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getNeedCleanCacheSize() (float64, error) {
	config := NewConfig(path.Join("/var/lib/lastore", "config.json"))
	isIncrementalUpdate := ""
	if config.IncrementalUpdate {
		isIncrementalUpdate = "--incremental-update"
	}
	output, err := exec.Command("/usr/bin/lastore-apt-clean", "-print-json", isIncrementalUpdate).Output()
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
		err = os.WriteFile(configPath, []byte(config), 0644)
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

func parseAutoDownloadTime(hourMinute string, now time.Time) (time.Time, error) {
	if hourMinute == "" {
		return time.Time{}, fmt.Errorf("hourMinute cannot be empty")
	}

	t, err := time.Parse(autoDownloadTimeLayout, hourMinute)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time %q: %w",
			hourMinute, err)
	}

	result := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	return result, nil
}

func parseAutoDownloadRange(idleDownloadConfig idleDownloadConfig, now time.Time) (TimeRange, error) {
	if idleDownloadConfig.BeginTime == "" || idleDownloadConfig.EndTime == "" {
		return TimeRange{}, fmt.Errorf("begin time and end time cannot be empty")
	}

	beginTime, err := parseAutoDownloadTime(idleDownloadConfig.BeginTime, now)
	if err != nil {
		return TimeRange{}, fmt.Errorf("failed to parse begin time: %w", err)
	}
	endTime, err := parseAutoDownloadTime(idleDownloadConfig.EndTime, now)
	if err != nil {
		return TimeRange{}, fmt.Errorf("failed to parse end time: %w", err)
	}
	// If beginTime is greater than endTime, for example, if beginTime is 23:00 and endTime is 03:00,
	// we need to add one day to endTime to ensure that beginTime is less than endTime.
	if beginTime.After(endTime) {
		endTime = endTime.AddDate(0, 0, 1)
	}
	return NewTimeRange(beginTime, endTime), nil
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
	oldControlCenterPath  = "/usr/lib/x86_64-linux-gnu/dde-control-center/dde-control-center-old"
	dataTransferPath      = "/usr/bin/deepin-data-transfer"
	amDaemonPath          = "/usr/bin/dde-application-manager"
	launcherPath          = "/usr/bin/dde-launcher"
	amDaemonCompatPath    = "/usr/libexec/dde-application-wizard-daemon-compat"
)

// TODO delete
var (
	allowInstallPackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		printerPath,
		printerHelperPath,
		langSelectorPath,
		controlCenterPath,
		oldControlCenterPath,
		dataTransferPath,
	}
	allowRemovePackageExecPaths = strv.Strv{
		appStoreDaemonPath,
		oldAppStoreDaemonPath,
		sessionDaemonPath,
		langSelectorPath,
		controlCenterPath,
		oldControlCenterPath,
		amDaemonPath,
		launcherPath,
		amDaemonCompatPath,
	}
)

// execPath和cmdLine可以有一个为空,其中一个存在即可作为判断调用者的依据
func getExecutablePathAndCmdline(service *dbusutil.Service, sender dbus.Sender) (string, string, error) {
	pid, err := service.GetConnPID(string(sender))
	if err != nil {
		return "", "", err
	}

	proc := procfs.Process(pid)
	if !checkSenderNsMntValid(pid) {
		return "", "", errors.New("due to the difference between the current process's ns mnt and the init process's ns mnt, the exe field is not reliable")
	}
	execPath, err := proc.Exe()
	if err != nil {
		// 当调用者在使用过程中发生了更新,则在获取该进程的exe时,会出现lstat xxx (deleted)此类的error,如果发生的是覆盖,则该路径依旧存在,因此增加以下判断
		var pErr *os.PathError
		ok := errors.As(err, &pErr)
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

func cleanAllCache() {
	err := exec.Command("apt-get", "clean", "-c", system.LastoreAptV2CommonConfPath).Run()
	if err != nil {
		logger.Warning(err)
	}
}

const aptLimitKey = "Acquire::http::Dl-Limit"

const upgradeRecordPath = "/var/cache/lastore/upgrade_record.json"

type recordInfo struct {
	UUID            string
	UpgradeTime     string
	UpgradeMode     system.UpdateType
	OriginChangelog interface{}
}

// mode 只能为单一类型
func recordUpgradeLog(uuid string, mode system.UpdateType, originChangelog interface{}, path string) {
	var allContent []recordInfo
	content, _ := os.ReadFile(path)
	if len(content) > 0 {
		err := json.Unmarshal(content, &allContent)
		if err != nil {
			logger.Warning(err)
			return
		}
	}

	info := recordInfo{
		UUID:            uuid,
		UpgradeTime:     time.Now().Format("2006-01-02"),
		UpgradeMode:     mode,
		OriginChangelog: originChangelog,
	}
	allContent = append([]recordInfo{
		info,
	}, allContent...)

	res, err := json.Marshal(allContent)
	if err != nil {
		logger.Warning("failed to marshal all upgrade log:", err)
		return
	}
	err = os.WriteFile(path, res, 0644)
	if err != nil {
		logger.Warning(err)
		return
	}
}

func getHistoryChangelog(path string) (changeLogs string) {
	content, err := os.ReadFile(path)
	if err != nil {
		logger.Warning(err)
		return
	}
	return string(content)
}

func checkSupportDpkgScriptIgnore() bool {
	output, err := exec.Command("dpkg", "--script-ignore-error", "--audit").Output()
	if err != nil {
		logger.Warning("audit dpkg script ignore capability:", err, string(output))
		return false
	}
	return true
}

const (
	coreListPath    = "/usr/share/core-list/corelist"
	coreListVarPath = "/var/lib/lastore/corelist"
	coreListPkgName = "deepin-package-list"
)

// 下载并解压coreList
func downloadAndDecompressCoreList() (string, error) {
	downloadPackages := []string{coreListPkgName}
	systemSource := system.GetCategorySourceMap()[system.SystemUpdate]
	var options map[string]string
	if info, err := os.Stat(systemSource); err == nil {
		if info.IsDir() {
			options = map[string]string{
				"Dir::Etc::SourceList":  "/dev/null",
				"Dir::Etc::SourceParts": systemSource,
			}
		} else {
			options = map[string]string{
				"Dir::Etc::SourceList":  systemSource,
				"Dir::Etc::SourceParts": "/dev/null",
			}
		}
	}
	downloadPkg, err := apt.DownloadPackages(downloadPackages, nil, options)
	if err != nil {
		// 下载失败则直接去本地目录查找
		logger.Warningf("download %v failed:%v", downloadPackages, err)
		return coreListPath, nil
	}
	// 去下载路径查找
	files, err := os.ReadDir(downloadPkg)
	if err != nil {
		return "", err
	}
	var debFile string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), coreListPkgName) && strings.HasSuffix(file.Name(), ".deb") {
			debFile = filepath.Join(downloadPkg, file.Name())
			break
		}
	}
	if debFile != "" {
		tmpDir, err := os.MkdirTemp("/tmp", coreListPkgName+".XXXXXX")
		if err != nil {
			return "", err
		}
		cmd := exec.Command("dpkg-deb", "-x", debFile, tmpDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if err != nil {
			return "", err
		}
		return filepath.Join(tmpDir, coreListPath), nil
	} else {
		return "", fmt.Errorf("coreList deb not found")
	}
}

func getCoreListFromCache() []string {
	// 初始化时获取coreList数据
	data, err := os.ReadFile(coreListVarPath)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	var pkgList PackageList
	err = json.Unmarshal(data, &pkgList)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	var pkgs []string
	for _, pkg := range pkgList.PkgList {
		pkgs = append(pkgs, pkg.PkgName)
	}
	return pkgs
}

type Package struct {
	PkgName string `json:"PkgName"`
	Version string `json:"Version"`
}

type PackageList struct {
	PkgList []Package `json:"PkgList"`
	Version string    `json:"Version"`
}

func getCoreListOnline() []string {
	// 1. download coreList to /var/cache/lastore/archives/
	// 2. 使用dpkg-deb解压deb得到coreList文件
	coreFilePath, err := downloadAndDecompressCoreList()
	if err != nil {
		logger.Warning(err)
		return nil
	}
	// 将coreList 备份到/var/lib/lastore/中
	err = utils2.CopyFile(coreFilePath, coreListVarPath)
	if err != nil {
		logger.Warning("backup coreList failed:", err)
	}
	// 3. 解析文件获取coreList必装列表
	data, err := os.ReadFile(coreFilePath)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	var pkgList PackageList
	err = json.Unmarshal(data, &pkgList)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	var pkgs []string
	for _, pkg := range pkgList.PkgList {
		pkgs = append(pkgs, pkg.PkgName)
	}
	return pkgs
}

var _initProcNsMnt string
var _once sync.Once

// 通过判断/proc/pid/ns/mnt 和 /proc/1/ns/mnt是否相同，如果不相同，则进程exe字段不可信
func checkSenderNsMntValid(pid uint32) bool {
	_once.Do(func() {
		out, err := os.Readlink("/proc/1/ns/mnt")
		if err != nil {
			fmt.Println(err)
			return
		}
		_initProcNsMnt = strings.TrimSpace(out)
	})
	c, err := os.Readlink(fmt.Sprintf("/proc/%v/ns/mnt", pid))
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer func() {
		fmt.Printf("pid 1 mnt ns is %v,pid %v mnt ns is %v\n", _initProcNsMnt, pid, strings.TrimSpace(c))
	}()
	return strings.TrimSpace(c) == _initProcNsMnt
}

const polkitActionChangeOwnData = "org.deepin.dde.accounts.user-administration"

func checkInvokePermission(service *dbusutil.Service, sender dbus.Sender) error {
	uid, err := service.GetConnUID(string(sender))
	if err != nil {
		return fmt.Errorf("failed to get sender conn uid:%v", err)
	}
	if uid != 0 {
		execPath, cmdLine, err := getExecutablePathAndCmdline(service, sender)
		if err != nil {
			logger.Warning(err)
			return polkit.CheckAuth(polkitActionChangeOwnData, string(sender), nil)
		}
		caller := mapMethodCaller(execPath, cmdLine)
		if methodCallerControlCenter == caller {
			return nil
		} else {
			logger.Infof("not allow %v  call this method ,need check auth by polkit", caller)
			return polkit.CheckAuth(polkitActionChangeOwnData, string(sender), nil)
		}

	} else {
		logger.Info("caller's uid is 0,allow to call this method")
		return nil
	}
}

type UpdateSourceConfig map[config.RepoType]*RepoInfo
type RepoInfo struct {
	/*
		UOS_DEFAULT 对应当前dconfig配置；RepoConfig 只读；
		OEM_DEFAULT 对应增加的OEM配置(为配置文件)，使用conf.d机制获取，取最高优先级配置；RepoConfig 只读；
		CUSTOM 默认为空，对应外部调用工具修改的配置，存储在dconfig中,生成的source.list文件存放在/var/lib/lastore/Custom.list.d下；RepoConfig 可读写
	*/
	RepoShowNameZh string
	RepoShowNameEn string
	IsUsing        bool
	RepoConfig     []string // "deb http://ftp.cn.debian.org/debian sid main"
}

func InitConfig(sourceConfig UpdateSourceConfig, oemRepoConfig config.OemRepoConfig, customRepo []string) {
	sourceConfig[config.OSDefaultRepo] = &RepoInfo{}
	sourceConfig[config.OemDefaultRepo] = &RepoInfo{
		RepoShowNameZh: oemRepoConfig.RepoShowNameZh,
		RepoShowNameEn: oemRepoConfig.RepoShowNameEn,
		RepoConfig:     oemRepoConfig.RepoUrl,
	}
	sourceConfig[config.CustomRepo] = &RepoInfo{
		RepoConfig: customRepo,
	}
}

func SetUsingRepoType(sourceConfig UpdateSourceConfig, repoType config.RepoType) {
	for k, v := range sourceConfig {
		if k == repoType {
			v.IsUsing = true
		} else {
			v.IsUsing = false
		}
	}
}

// getFileSha256 calculates the SHA-256 hash of a file.
func getFileSha256(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("filePath cannot be empty")
	}
	hash := sha256.New()
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer file.Close()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("failed to copy file %q: %w", filePath, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getContentSha256 calculates the SHA-256 hash of a string.
func getContentSha256(content string) string {
	hash := sha256.New()
	hash.Write([]byte(content))
	return hex.EncodeToString(hash.Sum(nil))
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewTimeRange creates a new time range
// If start > end, they will be swapped automatically
func NewTimeRange(start, end time.Time) TimeRange {
	if start.After(end) {
		start, end = end, start
	}
	return TimeRange{Start: start, End: end}
}

// Contains determines if a given time point is within the range (inclusive)
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

func (tr TimeRange) String() string {
	return fmt.Sprintf("%v ~ %v", tr.Start.Format(time.RFC3339), tr.End.Format(time.RFC3339))
}
