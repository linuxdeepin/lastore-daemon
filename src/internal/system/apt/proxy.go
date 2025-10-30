// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

const aptLimitKey = "Acquire::http::Dl-Limit"
const aptSourcePartsKey = "Dir::Etc::SourceParts"
const aptSourceListKey = "Dir::Etc::SourceList"

type APTSystem struct {
	CmdSet            map[string]*system.Command
	Indicator         system.Indicator
	IncrementalUpdate bool
}

func NewSystem(nonUnknownList []string, otherList []string, incrementalUpdate bool) system.System {
	logger.Info("using apt for update...")
	apt := New(nonUnknownList, otherList, incrementalUpdate)
	return &apt
}

func New(nonUnknownList []string, otherList []string, incrementalUpdate bool) APTSystem {
	p := APTSystem{
		CmdSet:            make(map[string]*system.Command),
		IncrementalUpdate: incrementalUpdate,
	}
	//WaitDpkgLockRelease()
	//_ = exec.Command("/var/lib/lastore/scripts/build_safecache.sh").Run() // TODO
	p.initSource(nonUnknownList, otherList)
	return p
}

func parseProgressField(v string) (float64, error) {
	progress, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return -1, fmt.Errorf("unknown progress value: %q", v)
	}
	return progress, nil
}

func parseProgressInfo(id, line string) (system.JobProgressInfo, error) {
	fs := strings.SplitN(line, ":", 4)
	if len(fs) != 4 {
		return system.JobProgressInfo{JobId: id}, fmt.Errorf("Invlaid Progress line:%q", line)
	}

	progress, err := parseProgressField(fs[2])
	if err != nil {
		return system.JobProgressInfo{JobId: id}, err
	}
	description := strings.TrimSpace(fs[3])

	var status system.Status
	var cancelable = true

	infoType := fs[0]

	switch infoType {
	case "dummy":
		status = system.Status(fs[1])
	case "dlstatus":
		progress = progress / 100.0
		status = system.RunningStatus
	case "pmstatus":
		progress = progress / 100.0
		status = system.RunningStatus
		cancelable = false
	case "pmerror":
		progress = -1
		if id != system.DistUpgradeJobType {
			status = system.FailedStatus
		}

	default:
		//	case "pmconffile", "media-change":
		return system.JobProgressInfo{JobId: id},
			fmt.Errorf("W: unknow status:%q", line)

	}

	return system.JobProgressInfo{
		JobId:       id,
		Progress:    progress,
		Description: description,
		Status:      status,
		Cancelable:  cancelable,
	}, nil
}

func (p *APTSystem) AttachIndicator(f system.Indicator) {
	p.Indicator = f
}

func WaitDpkgLockRelease() {
	for {
		msg, wait := system.CheckLock("/var/lib/dpkg/lock")
		if wait {
			logger.Warningf("Wait 5s for unlock\n\"%s\" \n at %v\n",
				msg, time.Now())
			time.Sleep(time.Second * 5)
			continue
		}

		msg, wait = system.CheckLock("/var/lib/dpkg/lock-frontend")
		if wait {
			logger.Warningf("Wait 5s for unlock\n\"%s\" \n at %v\n",
				msg, time.Now())
			time.Sleep(time.Second * 5)
			continue
		}

		return
	}
}

// ParsePkgSystemError is a wrapper for parsePkgSystemError
func ParsePkgSystemError(out, err []byte) error {
	return parsePkgSystemError(out, err)
}

func parsePkgSystemError(out, err []byte) error {
	if len(err) == 0 {
		return nil
	}
	switch {
	case bytes.Contains(err, []byte("dpkg was interrupted")):
		return &system.JobError{
			ErrType:   system.ErrorDpkgInterrupted,
			ErrDetail: string(err),
		}

	case bytes.Contains(err, []byte("Unmet dependencies")), bytes.Contains(err, []byte("generated breaks")):
		var detail string
		idx := bytes.Index(out,
			[]byte("The following packages have unmet dependencies:"))
		if idx == -1 {
			// not found
			detail = string(out)
		} else {
			detail = string(out[idx:])
		}

		return &system.JobError{
			ErrType:   system.ErrorDependenciesBroken,
			ErrDetail: detail,
		}

	case bytes.Contains(err, []byte("The list of sources could not be read")):
		detail := string(err)
		return &system.JobError{
			ErrType:   system.ErrorInvalidSourcesList,
			ErrDetail: detail,
		}

	default:
		detail := string(append(out, err...))
		return &system.JobError{
			ErrType:   system.ErrorUnknown,
			ErrDetail: detail,
		}
	}
}

func CheckPkgSystemError(lock bool, indicator system.Indicator) error {
	args := []string{"check"}
	if !lock {
		// without locking, it can only check for dependencies broken
		args = append(args, "-o", "Debug::NoLocking=1")
	}

	cmd := exec.Command("apt-get", args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	defer func() {
		indicator(system.JobProgressInfo{
			OnlyLog:     true,
			OriginalLog: fmt.Sprintf("=== CheckPkg %v end ===\n", cmd.Args),
		})
	}()

	indicator(system.JobProgressInfo{
		OnlyLog:     true,
		OriginalLog: fmt.Sprintf("=== CheckPkg cmd running: %v ===\n", cmd.Args),
	})
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return parsePkgSystemError(outBuf.Bytes(), errBuf.Bytes())
}

func safeStart(c *system.Command) error {
	args := c.Cmd.Args
	// add -s option
	args = append([]string{"-s"}, args[1:]...)
	cmd := exec.Command("apt-get", args...) // #nosec G204

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// perform apt-get action simulate
	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			jobErr := parseJobError(stderr.String(), stdout.String())
			c.IndicateFailed(jobErr.ErrType, jobErr.ErrDetail, false)
			return
		}

		// cmd run ok
		// check rm dde?
		if bytes.Contains(stdout.Bytes(), []byte("Remv dde ")) {
			c.IndicateFailed("removeDDE", "", true)
			return
		}

		// really perform apt-get action
		err = c.Start()
		if err != nil {
			c.IndicateFailed(system.ErrorUnknown,
				"apt-get start failed: "+err.Error(), false)
		}
	}()
	return nil
}

func OptionToArgs(options map[string]string) []string {
	var args []string
	for key, value := range options { // apt 命令执行参数
		args = append(args, "-o")
		args = append(args, fmt.Sprintf("%v=%v", key, value))
	}
	return args
}

func (p *APTSystem) DownloadPackages(jobId string, packages []string, environ map[string]string, args map[string]string) error {
	err := CheckPkgSystemError(false, p.Indicator)
	if err != nil {
		return err
	}
	c := newAPTCommand(p, jobId, system.DownloadJobType, p.Indicator, append(packages, OptionToArgs(args)...))
	c.SetEnv(environ)
	return c.Start()
}

func (p *APTSystem) DownloadSource(jobId string, packages []string, environ map[string]string, args map[string]string) error {
	// 无需检查依赖错误
	/*
		err := CheckPkgSystemError(false)
		if err != nil {
			return err
		}
	*/

	if p.IncrementalUpdate {
		var cmdArgs []string
		speedLimit, ok := args[aptLimitKey]
		if ok {
			cmdArgs = append(cmdArgs, "--max-recv-speed", speedLimit)
		}

		var upgradeArgString string
		aptSourceList, ok1 := args[aptSourceListKey]
		aptSourceParts, ok2 := args[aptSourcePartsKey]
		if ok1 && ok2 {
			upgradeArgString = fmt.Sprintf("-o %s=%s -o %s=%s",
				aptSourceListKey, aptSourceList,
				aptSourcePartsKey, aptSourceParts)
		}
		environ["DEEPIN_IMMUTABLE_UPGRADE_APT_OPTION"] = upgradeArgString
		logger.Info("DownloadSource set env DEEPIN_IMMUTABLE_UPGRADE_APT_OPTION:", upgradeArgString)

		c := newAPTCommand(p, jobId, system.IncrementalDownloadJobType, p.Indicator, cmdArgs)
		c.SetEnv(environ)
		return c.Start()
	}
	c := newAPTCommand(p, jobId, system.PrepareDistUpgradeJobType, p.Indicator, append(packages, OptionToArgs(args)...))
	c.SetEnv(environ)
	return c.Start()
}

func (p *APTSystem) Remove(jobId string, packages []string, environ map[string]string) error {
	WaitDpkgLockRelease()
	err := CheckPkgSystemError(true, p.Indicator)
	if err != nil {
		return err
	}

	c := newAPTCommand(p, jobId, system.RemoveJobType, p.Indicator, packages)
	environ["IMMUTABLE_DISABLE_REMOUNT"] = "false"
	c.SetEnv(environ)
	return safeStart(c)
}

func (p *APTSystem) Install(jobId string, packages []string, environ map[string]string, args map[string]string) error {
	WaitDpkgLockRelease()
	err := CheckPkgSystemError(true, p.Indicator)
	if err != nil {
		return err
	}
	c := newAPTCommand(p, jobId, system.InstallJobType, p.Indicator, append(OptionToArgs(args), packages...))
	environ["IMMUTABLE_DISABLE_REMOUNT"] = "false"
	c.SetEnv(environ)
	return safeStart(c)
}

func (p *APTSystem) DistUpgrade(jobId string, packages []string, environ map[string]string, args map[string]string) error {
	WaitDpkgLockRelease()
	err := CheckPkgSystemError(true, p.Indicator)
	if err != nil {
		// 无需处理依赖错误,在获取可更新包时,使用dist-upgrade -d命令获取,就会报错了
		var e *system.JobError
		ok := errors.As(err, &e)
		if !ok || (ok && e.ErrType != system.ErrorDependenciesBroken) {
			return err
		}
	}

	if p.IncrementalUpdate {
		logger.Info("incremental update")
		var cmdArgs []string
		speedLimit, ok := args[aptLimitKey]
		if ok {
			cmdArgs = append(cmdArgs, "--max-recv-speed", speedLimit)
		}

		var upgradeArgString string
		aptSourceList, ok1 := args[aptSourceListKey]
		aptSourceParts, ok2 := args[aptSourcePartsKey]
		if ok1 && ok2 {
			upgradeArgString = fmt.Sprintf("-o %s=%s -o %s=%s",
				aptSourceListKey, aptSourceList,
				aptSourcePartsKey, aptSourceParts)
		}
		environ["DEEPIN_IMMUTABLE_UPGRADE_APT_OPTION"] = upgradeArgString
		logger.Info("DistUpgrade set env DEEPIN_IMMUTABLE_UPGRADE_APT_OPTION:", upgradeArgString)

		c := newAPTCommand(p, jobId, system.IncrementalUpdateJobType, p.Indicator, cmdArgs)
		c.SetEnv(environ)
		return c.Start()
	}

	c := newAPTCommand(p, jobId, system.DistUpgradeJobType, p.Indicator, append(OptionToArgs(args), packages...))
	environ["IMMUTABLE_DISABLE_REMOUNT"] = "false"
	c.SetEnv(environ)
	return safeStart(c)
}

func (p *APTSystem) UpdateSource(jobId string, environ map[string]string, args map[string]string) error {
	if p.IncrementalUpdate {
		cmd := exec.Command(system.DeepinImmutableCtlPath, "upgrade", "update-remote")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to update remotes: %w, %s", err, string(output))
		}
	}
	c := newAPTCommand(p, jobId, system.UpdateSourceJobType, p.Indicator, OptionToArgs(args))
	c.AtExitFn = func() bool {
		// 无网络时检查更新失败,exitCode为0,空间不足(不确定exit code)导致需要特殊处理
		if c.ExitCode == system.ExitSuccess && bytes.Contains(c.Stderr.Bytes(), []byte("Some index files failed to download")) {
			if bytes.Contains(c.Stderr.Bytes(), []byte("No space left on device")) {
				c.IndicateFailed(system.ErrorInsufficientSpace, c.Stderr.String(), false)
			} else {
				c.IndicateFailed(system.ErrorIndexDownloadFailed, c.Stderr.String(), false)
			}
			return true
		}
		return false
	}
	c.SetEnv(environ)
	return c.Start()
}

func (p *APTSystem) Clean(jobId string) error {
	c := newAPTCommand(p, jobId, system.CleanJobType, p.Indicator, nil)
	return c.Start()
}

func (p *APTSystem) Abort(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		return c.Abort()
	}
	return system.NotFoundError("abort " + jobId)
}

func (p *APTSystem) AbortWithFailed(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		return c.AbortWithFailed()
	}
	return system.NotFoundError("abort " + jobId)
}

func (p *APTSystem) FixError(jobId string, errType string, environ map[string]string, args map[string]string) error {
	WaitDpkgLockRelease()
	c := newAPTCommand(p, jobId, system.FixErrorJobType, p.Indicator, append([]string{errType}, OptionToArgs(args)...))
	environ["IMMUTABLE_DISABLE_REMOUNT"] = "false"
	c.SetEnv(environ)
	if system.JobErrorType(errType) == system.ErrorDependenciesBroken { // 修复依赖错误的时候，会有需要卸载dde的情况，因此需要用safeStart来进行处理
		return safeStart(c)
	}
	return c.Start()
}

func (p *APTSystem) CheckSystem(jobId string, checkType string, environ map[string]string, cmdArgs map[string]string) error {
	return nil
}

func (p *APTSystem) initSource(nonUnknownList []string, otherList []string) {
	err := system.UpdateUnknownSourceDir(nonUnknownList)
	if err != nil {
		logger.Warning(err)
	}
	err = system.UpdateOtherSystemSourceDir(otherList)
	if err != nil {
		logger.Warning(err)
	}
}

func ListInstallPackages(packages []string) ([]string, error) {
	args := []string{
		"-c", system.LastoreAptV2CommonConfPath,
		"install", "-s",
		"-o", "Debug::NoLocking=1",
	}
	args = append(args, packages...)
	cmd := exec.Command("apt-get", args...) // #nosec G204
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	// NOTE: 这里不能使用命令的退出码来判断，因为 --assume-no 会让命令的退出码为 1
	_ = cmd.Run()

	const newInstalled = "The following additional packages will be installed:"
	if bytes.Contains(outBuf.Bytes(), []byte(newInstalled)) {
		p := parseAptShowList(bytes.NewReader(outBuf.Bytes()), newInstalled)
		return p, nil
	}

	err := parsePkgSystemError(outBuf.Bytes(), errBuf.Bytes())
	return nil, err
}

var _installRegex = regexp.MustCompile(`Inst (.*) \[.*] \(([^ ]+) .*\)`)
var _installRegex2 = regexp.MustCompile(`Inst (.*) \(([^ ]+) .*\)`)
var _removeRegex = regexp.MustCompile(`Remv (\S+)\s\[([^]]+)]`)

// GenOnlineUpdatePackagesByEmulateInstall option 需要带上仓库参数 // TODO 存在正则范围不够的情况，导致风险，需要替换成ListDistUpgradePackages
func GenOnlineUpdatePackagesByEmulateInstall(packages []string, option []string) (map[string]system.PackageInfo, map[string]system.PackageInfo, error) {
	allInstallPackages := make(map[string]system.PackageInfo)
	removePackages := make(map[string]system.PackageInfo)
	args := []string{
		"dist-upgrade", "-s",
		"-c", system.LastoreAptV2CommonConfPath,
		"-o", "Debug::NoLocking=1",
	}
	args = append(args, option...)
	if len(packages) > 0 {
		args = append(args, packages...)
	}
	cmd := exec.Command("apt-get", args...) // #nosec G204
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		logger.Warning(errBuf.String())
		return nil, nil, errors.New(errBuf.String())
	}
	const upgraded = "The following packages will be upgraded:"
	const newInstalled = "The following NEW packages will be installed:"
	const removed = "The following packages will be REMOVED:"
	if bytes.Contains(outBuf.Bytes(), []byte(upgraded)) ||
		bytes.Contains(outBuf.Bytes(), []byte(newInstalled)) ||
		bytes.Contains(outBuf.Bytes(), []byte(removed)) {
		// 证明平台要求包可以安装
		allLine := strings.Split(outBuf.String(), "\n")
		for _, line := range allLine {
			matches := _installRegex.FindStringSubmatch(line)
			if len(matches) < 3 {
				matches = _installRegex2.FindStringSubmatch(line)
			}
			if len(matches) >= 3 {
				allInstallPackages[matches[1]] = system.PackageInfo{
					Name:    matches[1],
					Version: matches[2],
					Need:    "skipversion",
				}
			} else {
				removeMatches := _removeRegex.FindStringSubmatch(line)
				if len(removeMatches) >= 3 {
					removePackages[removeMatches[1]] = system.PackageInfo{
						Name:    removeMatches[1],
						Version: removeMatches[2],
						Need:    "skipversion",
					}
				}
			}
		}
	}
	return allInstallPackages, removePackages, nil
}

// ListDistUpgradePackages return the pkgs from apt dist-upgrade
// NOTE: the result strim the arch suffix
func ListDistUpgradePackages(sourcePath string, option []string) ([]string, error) {
	args := []string{
		"-c", system.LastoreAptV2CommonConfPath,
		"dist-upgrade", "--assume-no",
		"-o", "Debug::NoLocking=1",
	}
	if info, err := os.Stat(sourcePath); err == nil {
		if info.IsDir() {
			args = append(args, "-o", "Dir::Etc::SourceList=/dev/null")
			args = append(args, "-o", "Dir::Etc::SourceParts="+sourcePath)
		} else {
			args = append(args, "-o", "Dir::Etc::SourceList="+sourcePath)
			args = append(args, "-o", "Dir::Etc::SourceParts=/dev/null")
		}
	} else {
		return nil, err
	}
	args = append(args, option...)
	cmd := exec.Command("apt-get", args...) // #nosec G204
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	// NOTE: 这里不能使用命令的退出码来判断，因为 --assume-no 会让命令的退出码为 1
	_ = cmd.Run()
	logger.Debug("cmd is ", cmd.String())
	const upgraded = "The following packages will be upgraded:"
	const newInstalled = "The following NEW packages will be installed:"
	if bytes.Contains(outBuf.Bytes(), []byte(upgraded)) ||
		bytes.Contains(outBuf.Bytes(), []byte(newInstalled)) {

		p := parseAptShowList(bytes.NewReader(outBuf.Bytes()), upgraded)
		p = append(p, parseAptShowList(bytes.NewReader(outBuf.Bytes()), newInstalled)...)
		return p, nil
	}

	err := parsePkgSystemError(outBuf.Bytes(), errBuf.Bytes())
	return nil, err
}

func parseAptShowList(r io.Reader, title string) []string {
	buf := bufio.NewReader(r)

	var p []string

	var line string
	in := false

	var err error
	for err == nil {
		line, err = buf.ReadString('\n')
		if strings.TrimSpace(title) == strings.TrimSpace(line) {
			in = true
			continue
		}

		if !in {
			continue
		}

		if !strings.HasPrefix(line, " ") {
			break
		}

		for _, f := range strings.Fields(line) {
			p = append(p, strings.Split(f, ":")[0])
		}
	}

	return p
}

// ImmutableCtlOutput is the json output of deepin-immutable-ctl command
type ImmutableCtlOutput struct {
	Code    uint8              `json:"code"`
	Message string             `json:"message"`
	Error   *ImmutableCtlError `json:"error"`
	Data    interface{}        `json:"data"`
}

// ImmutableCtlError is the error of deepin-immutable-ctl command
type ImmutableCtlError struct {
	Code    string   `json:"code"`
	Message []string `json:"message"`
}

func parseBackupJobError(stdErrStr string, stdOutStr string) *system.JobError {
	var output ImmutableCtlOutput
	err := json.Unmarshal([]byte(stdOutStr), &output)
	if err != nil {
		return &system.JobError{
			ErrType:   system.ErrorUnknown,
			ErrDetail: err.Error(),
			ErrorLog:  []string{stdErrStr},
		}
	}
	if output.Code == 0 {
		// success
		return nil
	}

	errDetail := fmt.Sprintf("err code: %v, err message: %+v",
		output.Error.Code, output.Error.Message)

	return &system.JobError{
		ErrType:   system.ErrorUnknown,
		ErrDetail: errDetail,
		ErrorLog:  []string{stdErrStr},
	}
}

func (p *APTSystem) OsBackup(jobId string) error {
	c := newAPTCommand(p, jobId, system.BackupJobType, p.Indicator, nil)
	c.ParseJobError = parseBackupJobError
	c.ParseProgressInfo = func(id, line string) (system.JobProgressInfo, error) {
		type info struct {
			Progress    float64 `json:"progress"`
			Description string  `json:"description"`
		}

		var progress float64
		var p info
		if err := json.Unmarshal([]byte(line), &p); err == nil {
			progress = p.Progress / 100.0
		}
		return system.JobProgressInfo{
			JobId:       jobId,
			Progress:    progress,
			Description: p.Description,
			Status:      system.RunningStatus,
			Cancelable:  false,
		}, nil
	}
	environ := map[string]string{
		"IMMUTABLE_DISABLE_REMOUNT": "false",
	}
	c.SetEnv(environ)
	c.Indicator(system.JobProgressInfo{
		JobId:         jobId,
		ResetProgress: true,
		Status:        system.RunningStatus,
		Cancelable:    false,
	})
	return c.Start()
}
