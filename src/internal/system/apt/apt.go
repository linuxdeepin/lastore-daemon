/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package apt

import (
	"bufio"
	"bytes"
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"os"
	"os/exec"
	"strings"
	"syscall"
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
	exitCode int

	aptPipe *os.File

	indicator system.Indicator

	logger bytes.Buffer
}

func (c aptCommand) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.apt.Args, " "))
}

func createCommandLine(cmdType string, packages []string) *exec.Cmd {
	var args []string = []string{"-y"}

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
		args = append(args, "-f", "remove")
		args = append(args, "--")
		args = append(args, packages...)
	case system.DownloadJobType:
		args = append(args, "-c", "/var/lib/lastore/apt.conf")
		args = append(args, "install", "-d")
		args = append(args, "--")
		args = append(args, packages...)
	case system.UpdateSourceJobType:
		sh := "apt-get -y -o APT::Status-Fd=3 -o Dir::Etc::sourceparts=/var/lib/lastore/source.d update && /var/lib/lastore/scripts/build_system_info -now"
		return exec.Command("/bin/sh", "-c", sh)

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
	cmd.Stderr = &r.logger

	cmdSet.AddCMD(r)
	return r
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

	err = c.apt.Start()
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
	log.Infof(c.logger.String())

	c.cmdSet.RemoveCMD(c.JobId)

	var line string
	var fmtStr = "dummy:%s:%f:%s"

	switch c.exitCode {
	case ExitSuccess:
		line = fmt.Sprintf(fmtStr, system.SucceedStatus, 1.0, c.JobId)
	case ExitFailure:
		line = fmt.Sprintf(fmtStr, system.FailedStatus, -1.0, c.JobId)
	case ExitPause:
		line = fmt.Sprintf(fmtStr, system.PausedStatus, -1.0, c.JobId)
	}
	info, err := ParseProgressInfo(c.JobId, line)
	if err != nil {
		log.Warnf("aptCommand.Wait.ParseProgressInfo (%q): %v\n", line, err)
	}

	c.indicator(info)
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
