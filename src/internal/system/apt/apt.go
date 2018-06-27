/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package apt

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"internal/system"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	log "github.com/cihub/seelog"
)

type CommandSet interface {
	AddCMD(cmd *aptCommand)
	RemoveCMD(id string)
	FindCMD(id string) *aptCommand
}

func (p *APTSystem) AddCMD(cmd *aptCommand) {
	if _, ok := p.cmdSet[cmd.JobId]; ok {
		log.Warnf("APTSystem AddCMD: exist cmd %q\n", cmd.JobId)
		return
	}
	log.Infof("APTSystem AddCMD: %v\n", cmd)
	p.cmdSet[cmd.JobId] = cmd
}
func (p *APTSystem) RemoveCMD(id string) {
	c, ok := p.cmdSet[id]
	if !ok {
		log.Warnf("APTSystem RemoveCMD with invalid Id=%q\n", id)
		return
	}
	log.Infof("APTSystem RemoveCMD: %v (exitCode:%d)\n", c, c.exitCode)
	delete(p.cmdSet, id)
}
func (p *APTSystem) FindCMD(id string) *aptCommand {
	return p.cmdSet[id]
}

type aptCommand struct {
	JobId      string
	Cancelable bool

	cmdSet CommandSet

	apt      *exec.Cmd
	aptMu    sync.Mutex
	exitCode int

	aptPipe *os.File

	indicator system.Indicator

	logger bytes.Buffer
	stderr bytes.Buffer
}

func (c aptCommand) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.apt.Args, " "))
}

func createCommandLine(cmdType string, packages []string) *exec.Cmd {
	var args = []string{"-y"}

	options := map[string]string{
		"APT::Status-Fd": "3",
	}

	if cmdType == system.DownloadJobType {
		options["Debug::NoLocking"] = "1"
		options["Acquire::Retries"] = "1"
		args = append(args, "-m")
	}

	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}

	switch cmdType {
	case system.InstallJobType:
		args = append(args, "-c", "/var/lib/lastore/apt.conf")
		args = append(args, "-f", "install")
		args = append(args, "--")
		args = append(args, packages...)
	case system.DistUpgradeJobType:
		args = append(args, "-c", "/var/lib/lastore/apt.conf")
		args = append(args, "--allow-downgrades", "--allow-change-held-packages")
		args = append(args, "dist-upgrade")
	case system.RemoveJobType:
		args = append(args, "-c", "/var/lib/lastore/apt.conf")
		args = append(args, "-f", "autoremove")
		args = append(args, "--")
		args = append(args, packages...)
	case system.DownloadJobType:
		args = append(args, "-c", "/var/lib/lastore/apt.conf")
		args = append(args, "install", "-d", "--allow-change-held-packages")
		args = append(args, "--")
		args = append(args, packages...)
	case system.UpdateSourceJobType:
		sh := "apt-get -y -o APT::Status-Fd=3 -o Dir::Etc::sourceparts=/var/lib/lastore/source.d update && /var/lib/lastore/scripts/build_system_info -now"
		return exec.Command("/bin/sh", "-c", sh)

	case system.CleanJobType:
		return exec.Command("/usr/bin/lastore-apt-clean")
	}

	return exec.Command("apt-get", args...)
}

func newAPTCommand(cmdSet CommandSet, jobId string, cmdType string, fn system.Indicator, packages []string) *aptCommand {
	cmd := createCommandLine(cmdType, packages)

	// See aptCommand.Abort
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	r := &aptCommand{
		JobId:      jobId,
		cmdSet:     cmdSet,
		indicator:  fn,
		apt:        cmd,
		Cancelable: true,
	}
	cmd.Stdout = &r.logger
	cmd.Stderr = &r.stderr

	cmdSet.AddCMD(r)
	return r
}

func (c *aptCommand) setEnv(envVarMap map[string]string) {
	if envVarMap == nil {
		return
	}

	envVarSlice := os.Environ()
	for key, value := range envVarMap {
		envVarSlice = append(envVarSlice, key+"="+value)
	}
	c.apt.Env = envVarSlice
}

func (c *aptCommand) Start() error {
	c.logger.WriteString(fmt.Sprintf("Begin AptCommand:%v\n", c))

	rr, ww, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("aptCommand.Start pipe : %v", err)
	}

	// It must be closed after c.osCMD.Start
	defer ww.Close()

	c.apt.ExtraFiles = append(c.apt.ExtraFiles, ww)

	c.aptMu.Lock()
	err = c.apt.Start()
	c.aptMu.Unlock()
	if err != nil {
		rr.Close()
		return err
	}

	c.aptPipe = rr

	go c.updateProgress()

	go c.Wait()

	return nil
}

func (c *aptCommand) Wait() (err error) {
	err = c.apt.Wait()
	if c.exitCode != ExitPause {
		if err != nil {
			c.exitCode = ExitFailure
			log.Infof("aptCommand.Wait: %v\n", err)
		} else {
			c.exitCode = ExitSuccess
		}
	}
	c.atExit()
	return err
}

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitPause   = 2
)

func (c *aptCommand) atExit() {
	c.aptPipe.Close()

	c.logger.WriteString(fmt.Sprintf("End AptCommand: %s\n", c.JobId))
	log.Info(c.logger.String())
	log.Info(c.stderr.String())

	c.cmdSet.RemoveCMD(c.JobId)

	switch c.exitCode {
	case ExitSuccess:
		c.indicator(system.JobProgressInfo{
			JobId:      c.JobId,
			Status:     system.SucceedStatus,
			Progress:   1.0,
			Cancelable: true,
		})
	case ExitFailure:
		errStr := c.stderr.String()
		c.indicator(system.JobProgressInfo{
			JobId:       c.JobId,
			Status:      system.FailedStatus,
			Progress:    -1.0,
			Cancelable:  true,
			Description: errStr,
		})
	case ExitPause:
		c.indicator(system.JobProgressInfo{
			JobId:      c.JobId,
			Status:     system.PausedStatus,
			Progress:   -1.0,
			Cancelable: true,
		})
	}
}

func (c *aptCommand) indicateFailed(description string) {
	log.Warn("AptCommand Failed: ", description)
	progressInfo := system.JobProgressInfo{
		JobId:       c.JobId,
		Progress:    -1.0,
		Description: description,
		Status:      system.FailedStatus,
		Cancelable:  true,
	}
	c.cmdSet.RemoveCMD(c.JobId)
	c.indicator(progressInfo)
}

func (c *aptCommand) Abort() error {
	if c.Cancelable {
		c.aptMu.Lock()
		defer c.aptMu.Unlock()
		if c.apt.Process == nil {
			return errors.New("the process has not yet started")
		}

		log.Tracef("Abort Command: %v\n", c)
		c.exitCode = ExitPause
		var err error
		pgid, err := syscall.Getpgid(c.apt.Process.Pid)
		if err != nil {
			return err
		}
		return syscall.Kill(-pgid, 2)
	}
	return system.NotSupportError
}

func (c *aptCommand) updateProgress() {
	b := bufio.NewReader(c.aptPipe)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, err := ParseProgressInfo(c.JobId, line)
		if err != nil {
			log.Errorf("aptCommand.updateProgress %v -> %v\n", info, err)
			continue
		}

		c.Cancelable = info.Cancelable
		c.indicator(info)
	}
}
