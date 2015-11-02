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
		//TODO: log
		return
	}
	p.cmdSet[cmd.OwnerId] = cmd
}
func (p *APTSystem) RemoveCMD(id string) {
	delete(p.cmdSet, id)
}
func (p *APTSystem) FindCMD(id string) *aptCommand {
	return p.cmdSet[id]
}

type aptCommand struct {
	OwnerId string
	Type    string

	cmdSet CommandSet

	osCMD *exec.Cmd

	aptPipe *os.File

	indicator system.Indicator

	output io.WriteCloser
	logger *log.Logger
}

func newAPTCommand(
	cmdSet CommandSet,
	jobId string,
	cmdType string, fn system.Indicator, packageId string) *aptCommand {
	options := map[string]string{
		"APT::Status-Fd": "3",
	}

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
		OwnerId:   jobId,
		cmdSet:    cmdSet,
		indicator: fn,
		osCMD:     cmd,
		output:    output,
		logger:    log.New(output, "", log.LstdFlags|log.Lshortfile),
	}

	r.logger.Printf("add cmd(%q) from cmdset\n", r.OwnerId)
	cmdSet.AddCMD(r)
	return r
}

func (c aptCommand) Start() error {
	c.logger.Println("Starting with ", c.osCMD.Args)
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

func (c aptCommand) Wait() error {
	c.logger.Printf("remove cmd(%q) from cmdset\n", c.OwnerId)
	c.cmdSet.RemoveCMD(c.OwnerId)

	defer func() {
		if c.output != nil {
			c.output.Close()
		}
	}()
	err := c.osCMD.Wait()
	if err != nil {
		c.logger.Println("osCMD.Wait():", err)
	}

	var line string
	if err != nil {
		line = "dstatus:" + system.FailedStatus + ":" + err.Error()
	} else {
		line = "dstatus:" + system.SucceedStatus + ":succeed"
	}
	info, err := ParseProgressInfo(c.OwnerId, line)
	if err != nil {
		c.logger.Println("ParseProgressInfo:", err)
	}

	c.logger.Printf("End indicator(%v)\n", info)
	c.indicator(info)

	return nil
}

func (c aptCommand) Abort() error {
	return c.osCMD.Process.Kill()
}

func (c aptCommand) updateProgress() {
	b := bufio.NewReader(c.aptPipe)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, _ := ParseProgressInfo(c.OwnerId, line)
		c.logger.Printf("indicator(%v)\n", info)
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
