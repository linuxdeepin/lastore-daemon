// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

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

	ErrorCheckMetaInfoFile              JobErrorType = "ErrorCheckMetaInfoFile"
	ErrorPreUpdateCheckScriptsFailed    JobErrorType = "ErrorPreUpdateCheckScriptsFailed"
	ErrorPostUpdateCheckScriptsFailed   JobErrorType = "ErrorPostUpdateCheckScriptsFailed"
	ErrorPreDownloadCheckScriptsFailed  JobErrorType = "ErrorPreDownloadCheckScriptsFailed"
	ErrorPostDownloadCheckScriptsFailed JobErrorType = "ErrorPostDownloadCheckScriptsFailed"
	ErrorPreBackupCheckScriptsFailed    JobErrorType = "ErrorPreBackupCheckScriptsFailed"
	ErrorPostBackupCheckScriptsFailed   JobErrorType = "ErrorPostBackupCheckScriptsFailed"
	ErrorPreCheckScriptsFailed          JobErrorType = "ErrorPreCheckScriptsFailed"
	ErrorMidCheckScriptsFailed          JobErrorType = "ErrorMidCheckScriptsFailed"
	ErrorPostCheckScriptsFailed         JobErrorType = "ErrorPostCheckScriptsFailed"
	ErrorSysPkgInfoLoad                 JobErrorType = "ErrorSysPkgInfoLoad"
	ErrorCheckToolsDependFailed         JobErrorType = "ErrorCheckToolsDependFailed"
	ErrorMetaInfoFile                   JobErrorType = "ErrorMetaInfoFile"
	ErrorDpkgVersion                    JobErrorType = "ErrorDpkgVersion"
	ErrorDpkgNotFound                   JobErrorType = "ErrorDpkgNotFound"
	ErrorCheckProgramFailed             JobErrorType = "ErrorCheckProgramFailed"
	ErrorCheckServiceFailed             JobErrorType = "ErrorCheckServiceFailed"
	ErrorCheckSysDiskOutSpace           JobErrorType = "ErrorCheckSysDiskOutSpace"
	ErrorCheckProcessNotRunning         JobErrorType = "ErrorCheckProcessNotRunning"
	ErrorCheckPkgNotFound               JobErrorType = "ErrorCheckPkgNotFound"
	ErrorCheckPkgState                  JobErrorType = "ErrorCheckPkgState"
	ErrorCheckPkgVersion                JobErrorType = "ErrorCheckPkgVersion"

	// running状态
	ErrorNeedCheck JobErrorType = "needCheck"
)

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
	// TODO: only for test
	if IntranetUpdate {
		return true
	}
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

// 单位是K
func GetFreeSpace(diskPath string) (int, error) {
	content, err := exec.Command("/usr/bin/df", "-BK", "--output=avail", diskPath).CombinedOutput()
	if err == nil {
		var contentKb string
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		lineCount := 1
		for scanner.Scan() {
			if lineCount == 2 {
				contentKb = scanner.Text()
				break
			}
			lineCount++
		}
		spaceStr := strings.Replace(contentKb, "K", "", -1)
		spaceStr = strings.TrimSpace(spaceStr)
		spaceNum, err := strconv.Atoi(spaceStr)
		if err == nil {
			spaceNum = spaceNum * 1000
			return spaceNum, nil
		} else {
			return 0, err
		}
	}
	return 0, fmt.Errorf("run /usr/bin/df -BK --output=avail %v err: %v", diskPath, string(content))
}
