// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"unicode"

	grub2 "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.grub2"
	license "github.com/linuxdeepin/go-dbus-factory/system/com.deepin.license"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/log"
)

const (
	// DeepinImmutableCtlPath is the path of deepin-immutable-ctl
	DeepinImmutableCtlPath = "/usr/sbin/deepin-immutable-ctl"
)

type MirrorSource struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Url  string `json:"url"`

	NameLocale  map[string]string `json:"name_locale"`
	Weight      int               `json:"weight"`
	Country     string            `json:"country"`
	AdjustDelay int               `json:"adjust_delay"` // ms
}

var RepoInfos []RepositoryInfo
var logger = log.NewLogger("lastore/system")

type RepositoryInfo struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Mirror string `json:"mirror"`
}

func DecodeJson(fpath string, d interface{}) error {
	// #nosec G304
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	return json.NewDecoder(f).Decode(&d)
}

func EncodeJson(fpath string, d interface{}) error {
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	return json.NewEncoder(f).Encode(d)
}

func NormalFileExists(fpath string) bool {
	info, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return true
}

// UpgradeStatusAndReason 用于记录整体更新安装的流程状态和原因,dde-session-daemon和回滚界面会根据该配置进行提示
type UpgradeStatusAndReason struct {
	Status     UpgradeStatus
	ReasonCode JobErrorType
}

// UpgradeStatus 整体更新安装的流程状态
type UpgradeStatus string

const (
	UpgradeReady   UpgradeStatus = "ready"
	UpgradeRunning UpgradeStatus = "running"
	UpgradeFailed  UpgradeStatus = "failed"
)

type JobErrorType string

func (j JobErrorType) String() string {
	return string(j)
}

const (
	NoError                      JobErrorType = "NoError"
	ErrorUnknown                 JobErrorType = "ErrorUnknown"
	ErrorProgram                 JobErrorType = "ErrorProgram"
	ErrorFetchFailed             JobErrorType = "fetchFailed"
	ErrorDpkgError               JobErrorType = "dpkgError"
	ErrorPkgNotFound             JobErrorType = "pkgNotFound"
	ErrorDpkgInterrupted         JobErrorType = "dpkgInterrupted"
	ErrorDependenciesBroken      JobErrorType = "dependenciesBroken"
	ErrorUnmetDependencies       JobErrorType = "unmetDependencies"
	ErrorNoInstallationCandidate JobErrorType = "noInstallationCandidate"
	ErrorInsufficientSpace       JobErrorType = "insufficientSpace"
	ErrorUnauthenticatedPackages JobErrorType = "unauthenticatedPackages"
	ErrorOperationNotPermitted   JobErrorType = "operationNotPermitted"
	ErrorIndexDownloadFailed     JobErrorType = "IndexDownloadFailed"
	ErrorIO                      JobErrorType = "ioError"
	ErrorDamagePackage           JobErrorType = "damagePackage" // 包损坏,需要删除后重新下载或者安装
	ErrorInvalidSourcesList      JobErrorType = "invalidSourceList"
	ErrorPlatformUnreachable     JobErrorType = "platformUnreachable"

	ErrorMissCoreFile  JobErrorType = "missCoreFile"
	ErrorScript        JobErrorType = "scriptError"
	ErrorProgressCheck JobErrorType = "progressCheckError"

	ErrorCheckMetaInfoFile       JobErrorType = "ErrorCheckMetaInfoFile"
	ErrorPreCheckScriptsFailed   JobErrorType = "ErrorPreCheckScriptsFailed"
	ErrorMidCheckScriptsFailed   JobErrorType = "ErrorMidCheckScriptsFailed"
	ErrorPostCheckScriptsFailed  JobErrorType = "ErrorPostCheckScriptsFailed"
	ErrorSysPkgInfoLoad          JobErrorType = "ErrorSysPkgInfoLoad"
	ErrorCheckToolsDependFailed  JobErrorType = "ErrorCheckToolsDependFailed"
	ErrorMetaInfoFile            JobErrorType = "ErrorMetaInfoFile"
	ErrorDpkgVersion             JobErrorType = "ErrorDpkgVersion"
	ErrorDpkgNotFound            JobErrorType = "ErrorDpkgNotFound"
	ErrorCheckProgramFailed     JobErrorType = "ErrorCheckProgramFailed"
	ErrorCheckSysDiskOutSpace    JobErrorType = "ErrorCheckSysDiskOutSpace"
	ErrorCheckProcessNotRunning JobErrorType = "ErrorCheckProcessNotRunning"
	ErrorCheckPkgNotFound        JobErrorType = "ErrorCheckPkgNotFound"
	ErrorCheckPkgState           JobErrorType = "ErrorCheckPkgState"
	ErrorCheckPkgVersion         JobErrorType = "ErrorCheckPkgVersion"

	// running状态
	ErrorNeedCheck JobErrorType = "needCheck"
)

const (
	GrubTitleRollbackPrefix = "BEGIN /etc/grub.d/11_deepin_ab_recovery"
	GrubTitleRollbackSuffix = "END /etc/grub.d/11_deepin_ab_recovery"
	GrubTitleNormalPrefix   = "BEGIN /etc/grub.d/15_immutable"
	GrubTitleNormalSuffix   = "END /etc/grub.d/15_immutable"
)

func GetGrubRollbackTitle(grubPath string) string {
	return getGrubTitleByPrefix(grubPath, GrubTitleRollbackPrefix, GrubTitleRollbackSuffix)
}

func GetGrubNormalTitle(grubPath string) string {
	return getGrubTitleByPrefix(grubPath, GrubTitleNormalPrefix, GrubTitleNormalSuffix)
}

func getGrubTitleByPrefix(grubPath string, start, end string) (entryTitle string) {
	fileContent, err := os.ReadFile(grubPath)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	sl := bufio.NewScanner(strings.NewReader(string(fileContent)))
	sl.Split(bufio.ScanLines)
	needNext := false
	for sl.Scan() {
		line := sl.Text()
		line = strings.TrimSpace(line)
		if !needNext {
			needNext = strings.Contains(line, start)
		} else {
			if strings.Contains(line, end) {
				logger.Warningf("%v not found %v entry", grubPath, start)
				return ""
			}
			if strings.HasPrefix(line, "menuentry ") {
				title, ok := parseTitle(line)
				if ok {
					entryTitle = title
					break
				} else {
					logger.Warningf("parse entry title failed from: %q", line)
					return ""
				}
			}
		}
	}
	err = sl.Err()
	if err != nil {
		return ""
	}
	return entryTitle
}

// getGrubTitleByIndex index 的起始值是0
func getGrubTitleByIndex(grub grub2.Grub2, index int) (entryTitle string) {
	if grub == nil {
		return ""
	}
	entryList, err := grub.GetSimpleEntryTitles(0)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	if len(entryList) < index+1 {
		logger.Warningf(" index:%v out of range", index)
		return ""
	}
	return entryList[index]
}

var (
	entryRegexpSingleQuote = regexp.MustCompile(`^ *(menuentry|submenu) +'(.*?)'.*$`)
	entryRegexpDoubleQuote = regexp.MustCompile(`^ *(menuentry|submenu) +"(.*?)".*$`)
)

func parseTitle(line string) (string, bool) {
	line = strings.TrimLeftFunc(line, unicode.IsSpace)
	if entryRegexpSingleQuote.MatchString(line) {
		return entryRegexpSingleQuote.FindStringSubmatch(line)[2], true
	} else if entryRegexpDoubleQuote.MatchString(line) {
		return entryRegexpDoubleQuote.FindStringSubmatch(line)[2], true
	} else {
		return "", false
	}
}

func HandleDelayPackage(hold bool, packages []string) {
	action := "unhold"
	if hold {
		action = "hold"
	}
	args := []string{
		action,
	}
	args = append(args, packages...)
	err := exec.Command("apt-mark", args...).Run()
	if err != nil {
		logger.Warning(err)
	}
}

type UpdateModeStatus string

const (
	NoUpdate       UpdateModeStatus = "noUpdate"    // 无更新
	NotDownload    UpdateModeStatus = "notDownload" // 包含了有更新没下载
	IsDownloading  UpdateModeStatus = "isDownloading"
	DownloadPause  UpdateModeStatus = "downloadPause"
	DownloadErr    UpdateModeStatus = "downloadFailed"
	CanUpgrade     UpdateModeStatus = "downloaded"   // Downloaded
	WaitRunUpgrade UpdateModeStatus = "upgradeReady" // 进行备份+更新时,当处于更新未开始状态
	Upgrading      UpdateModeStatus = "upgrading"
	UpgradeErr     UpdateModeStatus = "upgradeFailed"
	Upgraded       UpdateModeStatus = "needReboot" // need reboot
)

type ABStatus string

const (
	NotBackup ABStatus = "notBackup"
	// NotSupportBackup ABStatus = "notSupportBackup"
	BackingUp    ABStatus = "backingUp"
	BackupFailed ABStatus = "backupFailed"
	HasBackedUp  ABStatus = "hasBackedUp"
)

type ABErrorType string

const (
	NoABError    ABErrorType = "noError"
	CanNotBackup ABErrorType = "canNotBackup"
	OtherError   ABErrorType = "otherError"
)

type UiActiveState int32

const (
	Unknown         UiActiveState = -1 // 未知
	Unauthorized    UiActiveState = 0  // 未授权
	Authorized      UiActiveState = 1  // 已授权
	AuthorizedLapse UiActiveState = 2  // 授权失效
	TrialAuthorized UiActiveState = 3  // 试用期已授权
	TrialExpired    UiActiveState = 4  // 试用期已过期
)

func IsAuthorized() bool {
	edition, err := getEditionName()
	if err != nil {
		return false
	}
	// 社区版不需要鉴权
	if edition == "Community" {
		return true
	}
	sysBus, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning(err)
		return false
	}
	licenseObj := license.NewLicense(sysBus.Conn())
	state, err := licenseObj.AuthorizationState().Get(0)
	if err != nil {
		logger.Warning(err)
		return false
	}
	if UiActiveState(state) == Authorized || UiActiveState(state) == TrialAuthorized {
		return true
	}
	return false
}

func IsActiveCodeExist() bool {
	sysBus, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning(err)
		return false
	}
	licenseObj := license.NewLicense(sysBus.Conn())
	code, err := licenseObj.ActiveCode().Get(0)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return strings.TrimSpace(code) != ""
}

func CheckLock(p string) (string, bool) {
	// #nosec G304
	file, err := os.Open(p)
	if err != nil {
		logger.Warningf("error opening %q: %v", p, err)
		return "", false
	}
	defer func() {
		_ = file.Close()
	}()

	flockT := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
		Start:  0,
		Len:    0,
		Pid:    0,
	}
	err = syscall.FcntlFlock(file.Fd(), syscall.F_GETLK, &flockT)
	if err != nil {
		logger.Warningf("unable to check file %q lock status: %s", p, err)
		return p, true
	}

	if flockT.Type == syscall.F_WRLCK {
		return p, true
	}

	return "", false
}

func getEditionName() (string, error) {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/os-version")
	if err != nil {
		return "", err
	}
	editionName, err := kf.GetString("Version", "EditionName")
	if err != nil {
		return "", err
	}
	return editionName, nil
}
