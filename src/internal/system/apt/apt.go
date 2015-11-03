package apt

import (
	"bufio"
	"fmt"
	"internal/system"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

type CommandSet interface {
	AddCMD(cmd *aptCommand)
	RemoveCMD(id string)
	FindCMD(id string) *aptCommand
}

func (p *APTSystem) AddCMD(cmd *aptCommand) {
	if _, ok := p.cmdSet[cmd.OwnerId]; ok {
		log.Printf("APTSystem AddCMD: exist cmd %q", cmd.OwnerId)
		return
	}
	log.Printf("APTSystem AddCMD: %v\n", cmd)
	p.cmdSet[cmd.OwnerId] = cmd
}
func (p *APTSystem) RemoveCMD(id string) {
	c, ok := p.cmdSet[id]
	if !ok {
		log.Printf("APTSystem RemoveCMD with invalid Id=%q\n", id)
		return
	}
	log.Printf("APTSystem RemoveCMD: %v (exitCode:%d)", c, c.exitCode)
	delete(p.cmdSet, id)
}
func (p *APTSystem) FindCMD(id string) *aptCommand {
	return p.cmdSet[id]
}

type aptCommand struct {
	OwnerId    string
	Cancelable bool

	cmdSet CommandSet

	osCMD    *exec.Cmd
	exitCode int

	aptPipe *os.File

	indicator system.Indicator

	output io.WriteCloser
}

func (c aptCommand) String() string {
	return fmt.Sprintf("AptCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.OwnerId, c.Cancelable, strings.Join(c.osCMD.Args, " "))
}

func newAPTCommand(cmdSet CommandSet, jobId string, cmdType string, fn system.Indicator, packageId string) *aptCommand {
	options := map[string]string{
		"APT::Status-Fd": "3",
	}

	cancelable := false

	polices := []string{"-y"}
	var args []string
	switch cmdType {
	case system.InstallJobType:
		args = append(args, "install", packageId)
	case system.RemoveJobType:
		args = append(args, "remove", packageId)
	case system.DownloadJobType:
		options["Debug::NoLocking"] = "1"
		args = append(args, "install", "-d", packageId)
		cancelable = true
	case system.DistUpgradeJobType:
		args = append(args, "dist-upgrade", "--force-yes")
	}

	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}
	args = append(args, polices...)

	cmd := exec.Command("apt-get", args...)
	output := system.CreateLogOutput(cmdType, packageId)
	cmd.Stdout = output
	cmd.Stderr = output

	r := &aptCommand{
		OwnerId:    jobId,
		Cancelable: cancelable,
		cmdSet:     cmdSet,
		indicator:  fn,
		osCMD:      cmd,
		output:     output,
	}

	cmdSet.AddCMD(r)
	return r
}

func (c *aptCommand) Start() error {
	log.Printf("AptCommand.Start:%v\n", c)
	rr, ww, err := os.Pipe()
	defer ww.Close()
	if err != nil {
		return fmt.Errorf("aptCommand.Start pipe : %v", err)
	}
	c.osCMD.ExtraFiles = append(c.osCMD.ExtraFiles, ww)
	c.aptPipe = rr

	err = c.osCMD.Start()
	if err != nil {
		return err
	}

	go c.updateProgress()
	go c.Wait()
	return nil
}

func (c *aptCommand) Wait() (err error) {
	err = c.osCMD.Wait()
	if c.exitCode != ExitPause {
		fmt.Println("NNNNNNNNNNNNNNNNNNNNNNNNNNNNNN.......", c.exitCode)
		if err != nil {
			c.exitCode = ExitFailure
			log.Printf("aptCommand.Wait: %v\n", err)
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

	c.cmdSet.RemoveCMD(c.OwnerId)

	var line string
	switch c.exitCode {
	case ExitSuccess:
		line = "dstatus:" + system.SucceedStatus + ":" + "succeed"
	case ExitFailure:
		line = "dstatus:" + system.FailedStatus + ":" + "failed"
	case ExitPause:
		line = "dstatus:" + system.PausedStatus + ":" + "paused"
	}
	info, err := ParseProgressInfo(c.OwnerId, line)
	if err != nil {
		log.Printf("aptCommand.Wait.ParseProgressInfo (%q): %v", line, err)
	}

	c.indicator(info)
}

func (c *aptCommand) Abort() error {
	if c.Cancelable {
		c.exitCode = ExitPause
		return c.osCMD.Process.Kill()
	}
	return system.NotSupportError
}

func (c aptCommand) updateProgress() {
	b := bufio.NewReader(c.aptPipe)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, _ := ParseProgressInfo(c.OwnerId, line)
		log.Printf("aptCommand.updateProgress %v\n", info)
		c.indicator(info)
	}
}

func getSystemArchitectures() []system.Architecture {
	bs, err := ioutil.ReadFile("/var/lib/dpkg/arch")
	if err != nil {
		log.Fatalln("Can't detect system architectures:", err)
		os.Exit(1)
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
