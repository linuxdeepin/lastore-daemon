// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"bytes"
	"fmt"
	"internal/system"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type APTSystem struct {
	cmdSet    map[string]*aptCommand
	indicator system.Indicator
}

func New(systemSourceList []string, nonUnknownList []string) system.System {
	p := &APTSystem{
		cmdSet: make(map[string]*aptCommand),
	}
	WaitDpkgLockRelease()
	_ = exec.Command("/var/lib/lastore/scripts/build_safecache.sh").Run()
	p.initSource(systemSourceList, nonUnknownList)
	return p
}

func parseProgressField(v string) (float64, error) {
	progress, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return -1, fmt.Errorf("unknown progress value: %q", v)
	}
	return progress, nil
}

func ParseProgressInfo(id, line string) (system.JobProgressInfo, error) {
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
		status = system.FailedStatus

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
	p.indicator = f
}

func WaitDpkgLockRelease() {
	for {
		msg, wait := checkLock("/var/lib/dpkg/lock")
		if wait {
			logger.Warningf("Wait 5s for unlock\n\"%s\" \n at %v\n",
				msg, time.Now())
			time.Sleep(time.Second * 5)
			continue
		}

		msg, wait = checkLock("/var/lib/dpkg/lock-frontend")
		if wait {
			logger.Warningf("Wait 5s for unlock\n\"%s\" \n at %v\n",
				msg, time.Now())
			time.Sleep(time.Second * 5)
			continue
		}

		return
	}
}

func checkLock(p string) (string, bool) {
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

func ParsePkgSystemError(out, err []byte) error {
	return parsePkgSystemError(out, err)
}

func parsePkgSystemError(out, err []byte) error {
	if len(err) == 0 {
		return nil
	}
	switch {
	case bytes.Contains(err, []byte("dpkg was interrupted")):
		return &system.PkgSystemError{
			Type: system.ErrTypeDpkgInterrupted,
		}

	case bytes.Contains(err, []byte("Unmet dependencies")):
		var detail string
		idx := bytes.Index(out,
			[]byte("The following packages have unmet dependencies:"))
		if idx == -1 {
			// not found
			detail = string(out)
		} else {
			detail = string(out[idx:])
		}

		return &system.PkgSystemError{
			Type:   system.ErrTypeDependenciesBroken,
			Detail: detail,
		}

	case bytes.Contains(err, []byte("The list of sources could not be read")):
		detail := string(err)
		return &system.PkgSystemError{
			Type:   system.ErrTypeInvalidSourcesList,
			Detail: detail,
		}

	default:
		detail := string(err)
		return &system.PkgSystemError{
			Type:   system.ErrTypeUnknown,
			Detail: detail,
		}
	}
}

func checkPkgSystemError(lock bool) error {
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
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return parsePkgSystemError(outBuf.Bytes(), errBuf.Bytes())
}

func safeStart(c *aptCommand) error {
	args := c.apt.Args
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
			c.indicateFailed(jobErr.Type, jobErr.Detail, false)
			return
		}

		// cmd run ok
		// check rm dde?
		if bytes.Contains(stdout.Bytes(), []byte("Remv dde ")) {
			c.indicateFailed("removeDDE", "", true)
			return
		}

		// really perform apt-get action
		err = c.Start()
		if err != nil {
			c.indicateFailed("unknown",
				"apt-get start failed: "+err.Error(), false)
		}
	}()
	return nil
}

func (p *APTSystem) DownloadPackages(jobId string, packages []string, environ map[string]string, args []string) error {
	err := checkPkgSystemError(false)
	if err != nil {
		return err
	}
	c := newAPTCommand(p, jobId, system.DownloadJobType, p.indicator, append(packages, args...))
	c.setEnv(environ)
	return c.Start()
}

func (p *APTSystem) DownloadSource(jobId string, environ map[string]string, cmdArgs []string) error {
	// 无需检查依赖错误
	/*
		err := checkPkgSystemError(false)
		if err != nil {
			return err
		}
	*/
	c := newAPTCommand(p, jobId, system.PrepareDistUpgradeJobType, p.indicator, cmdArgs)
	c.setEnv(environ)
	return c.Start()
}

func (p *APTSystem) Remove(jobId string, packages []string, environ map[string]string) error {
	WaitDpkgLockRelease()
	err := checkPkgSystemError(true)
	if err != nil {
		return err
	}

	c := newAPTCommand(p, jobId, system.RemoveJobType, p.indicator, packages)
	c.setEnv(environ)
	return safeStart(c)
}

func (p *APTSystem) Install(jobId string, packages []string, environ map[string]string, args []string) error {
	WaitDpkgLockRelease()
	err := checkPkgSystemError(true)
	if err != nil {
		return err
	}
	c := newAPTCommand(p, jobId, system.InstallJobType, p.indicator, append(packages, args...))
	c.setEnv(environ)
	return safeStart(c)
}

func (p *APTSystem) DistUpgrade(jobId string, environ map[string]string, cmdArgs []string) error {
	WaitDpkgLockRelease()
	err := checkPkgSystemError(true)
	if err != nil {
		// 无需处理依赖错误,在获取可更新包时,使用dist-upgrade -d命令获取,就会报错了
		e, ok := err.(*system.PkgSystemError)
		if !ok || (ok && e.Type != system.ErrTypeDependenciesBroken) {
			return err
		}
	}
	c := newAPTCommand(p, jobId, system.DistUpgradeJobType, p.indicator, cmdArgs)
	c.setEnv(environ)
	return safeStart(c)
}

func (p *APTSystem) UpdateSource(jobId string, environ map[string]string) error {
	c := newAPTCommand(p, jobId, system.UpdateSourceJobType, p.indicator, nil)
	c.atExitFn = func() bool {
		if c.exitCode == ExitSuccess &&
			bytes.Contains(c.stderr.Bytes(), []byte("Some index files failed to download")) {
			if bytes.Contains(c.stderr.Bytes(), []byte("No space left on device")) {
				c.indicateFailed(string(system.ErrorInsufficientSpace), c.stderr.String(), false)
			}
			c.indicateFailed(string(system.ErrorIndexDownloadFailed), c.stderr.String(), false)
			return true
		}
		return false
	}
	c.setEnv(environ)
	return c.Start()
}

func (p *APTSystem) Clean(jobId string) error {
	c := newAPTCommand(p, jobId, system.CleanJobType, p.indicator, nil)
	return c.Start()
}

func (p *APTSystem) Abort(jobId string) error {
	if c := p.FindCMD(jobId); c != nil {
		return c.Abort()
	}
	return system.NotFoundError("abort " + jobId)
}

func (p *APTSystem) FixError(jobId string, errType string, environ map[string]string, cmdArgs []string) error {
	WaitDpkgLockRelease()
	c := newAPTCommand(p, jobId, system.FixErrorJobType, p.indicator, append([]string{errType}, cmdArgs...))
	c.setEnv(environ)
	if errType == system.ErrTypeDependenciesBroken { // 修复依赖错误的时候，会有需要卸载dde的情况，因此需要用safeStart来进行处理
		return safeStart(c)
	}
	return c.Start()
}

func (p *APTSystem) initSource(systemSourceList []string, nonUnknownList []string) {
	// apt初始化时执行一次，避免其他apt操作过程中删改软链接导致数据异常
	err := system.UpdateUnknownSourceDir(nonUnknownList)
	if err != nil {
		logger.Warning(err)
	}

	err = system.UpdateSystemSourceDir(systemSourceList)
	if err != nil {
		logger.Warning(err)
	}
}
