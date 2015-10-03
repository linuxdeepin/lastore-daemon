package apt

import (
	"bufio"
	"fmt"
	"internal/system"
	"log"
	"os"
	"os/exec"
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

	log *log.Logger
}

func newAPTCommand(
	cmdSet CommandSet,
	jobId string,
	cmdType string, fn system.Indicator, packageId string, region string) *aptCommand {
	options := map[string]string{
		"APT::Status-Fd": "3",
	}
	if region != "" {
		options["Acquire::SmartMirrors::Region"] = region
	}

	polices := []string{"-y"}
	var args []string
	switch cmdType {
	case "install":
		args = append(args, "install", packageId)
	case "remove":
		args = append(args, "remove", packageId)
	case "download":
		options["Debug::NoLocking"] = "1"
		args = append(args, "install", "-d", packageId)
	}

	for k, v := range options {
		args = append(args, "-o", k+"="+v)
	}
	args = append(args, polices...)

	cmd := exec.Command("apt-get", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	ShowFd(os.Getpid())

	r := &aptCommand{
		OwnerId:   jobId,
		cmdSet:    cmdSet,
		indicator: fn,
		osCMD:     cmd,
	}
	cmdSet.AddCMD(r)
	return r
}

func (c aptCommand) Start() {
	rr, ww, err := os.Pipe()
	defer ww.Close()
	if err != nil {
		log.Println("os.Pipe error:", err)
	}
	log.Println("Actual FD:", rr.Fd(), ww.Fd(), err)
	log.Println("Start...", c)
	c.osCMD.ExtraFiles = append(c.osCMD.ExtraFiles, ww)
	c.aptPipe = rr

	c.osCMD.Start()
	ww.Close()

	go c.updateProgress()
	go c.Wait()
}

func (c aptCommand) Wait() error {
	c.cmdSet.RemoveCMD(c.OwnerId)

	err := c.osCMD.Wait()
	ShowFd(os.Getpid())
	fmt.Println("----------------Waited..", c.osCMD.Args, err)

	var line string
	if err != nil {
		line = "dstatus:" + system.FailedStatus + ":" + err.Error()
	} else {
		line = "dstatus:" + system.SuccessedStatus + ":successed"
	}
	info, err := ParseProgressInfo(c.OwnerId, line)
	c.indicator(info)
	return nil
}

func (c aptCommand) updateProgress() {
	b := bufio.NewReader(c.aptPipe)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			return
		}

		info, _ := ParseProgressInfo(c.OwnerId, line)
		c.indicator(info)
	}
}

func (c aptCommand) Abort(jobId string) error {
	return system.NotImplementError
}

func ShowFd(pid int) {
	return
	log.Println("PID:", pid)
	out, err := exec.Command("/bin/ls", "-lh", fmt.Sprintf("/proc/%d/fd", pid)).Output()
	log.Println(string(out), err)
	log.Println("__ENDPID__________")
}
