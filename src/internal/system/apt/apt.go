package apt

import (
	"bufio"
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func init() {
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")
}

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

	output io.WriteCloser
}

func (c aptCommand) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.apt.Args, " "))
}

func createCommandLine(cmdType string, packageId string) *exec.Cmd {
	var args []string = []string{"-y", "-c", "/var/lib/lastore/apt.conf"}

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
		args = append(args, "-f", "install", packageId)
	case system.RemoveJobType:
		args = append(args, "-f", "remove", packageId)
	case system.DownloadJobType:
		args = append(args, "install", "-d", packageId)
	case system.DistUpgradeJobType:
		args = append(args, "-f", "dist-upgrade", "--force-yes")
	case system.UpdateSourceJobType:
		args = append(args, "update")
	}

	return exec.Command("apt-get", args...)
}

func newAPTCommand(cmdSet CommandSet, jobId string, cmdType string, fn system.Indicator, packageId string) *aptCommand {
	cmd := createCommandLine(cmdType, packageId)

	// See aptCommand.Abort
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	output := system.CreateLogOutput(cmdType, packageId)
	cmd.Stdout = output
	cmd.Stderr = output

	r := &aptCommand{
		JobId:      jobId,
		cmdSet:     cmdSet,
		indicator:  fn,
		apt:        cmd,
		output:     output,
		Cancelable: true,
	}

	cmdSet.AddCMD(r)
	return r
}

func (c *aptCommand) Start() error {
	log.Infof("AptCommand.Start:%v\n", c)

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
	if c.output != nil {
		c.output.Close()
	}

	c.aptPipe.Close()

	c.cmdSet.RemoveCMD(c.JobId)

	var line string
	switch c.exitCode {
	case ExitSuccess:
		line = "dstatus:" + system.SucceedStatus + ":" + "succeed"
	case ExitFailure:
		line = "dstatus:" + system.FailedStatus + ":" + "failed"
	case ExitPause:
		line = "dstatus:" + system.PausedStatus + ":" + "paused"
	}
	info, err := ParseProgressInfo(c.JobId, line)
	if err != nil {
		log.Warnf("aptCommand.Wait.ParseProgressInfo (%q): %v\n", line, err)
	}

	c.indicator(info)
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
			log.Errorf("aptCommand.updateProgress %v\n", info)
			continue
		}

		if strings.Contains(line, "rename failed") {
			// ignore rename failed
			continue
		}

		c.Cancelable = info.Cancelable
		c.indicator(info)
	}
}

func getSystemArchitectures() []system.Architecture {
	bs, err := ioutil.ReadFile("/var/lib/dpkg/arch")
	if err != nil {
		log.Error("Can't detect system architectures:", err)
		return nil
	}
	var r []system.Architecture
	for _, arch := range strings.Split(string(bs), "\n") {
		i := strings.TrimSpace(arch)
		if i == "" {
			continue
		}
		r = append(r, system.Architecture(i))
	}
	return r
}
