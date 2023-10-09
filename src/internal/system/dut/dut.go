package dut

import (
	"bytes"
	"fmt"
	"internal/system"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/linuxdeepin/go-lib/log"
)

type CommandSet interface {
	AddCMD(cmd *dutCommand)
	RemoveCMD(id string)
	FindCMD(id string) *dutCommand
}

var logger = log.NewLogger("lastore/dut")

func (p *DutSystem) AddCMD(cmd *dutCommand) {
	if _, ok := p.cmdSet[cmd.JobId]; ok {
		logger.Warningf("DutSystem AddCMD: exist cmd %q\n", cmd.JobId)
		return
	}
	logger.Infof("DutSystem AddCMD: %v\n", cmd)
	p.cmdSet[cmd.JobId] = cmd
}

func (p *DutSystem) RemoveCMD(id string) {
	c, ok := p.cmdSet[id]
	if !ok {
		logger.Warningf("DutSystem RemoveCMD with invalid Id=%q\n", id)
		return
	}
	logger.Infof("DutSystem RemoveCMD: %v (exitCode:%d)\n", c, c.exitCode)
	delete(p.cmdSet, id)
}

func (p *DutSystem) FindCMD(id string) *dutCommand {
	return p.cmdSet[id]
}

type dutCommand struct {
	JobId      string
	Cancelable bool

	cmdSet CommandSet

	apt      *exec.Cmd
	aptMu    sync.Mutex
	exitCode int

	aptPipe *os.File

	indicator system.Indicator

	stdout   bytes.Buffer
	stderr   bytes.Buffer
	atExitFn func() bool
}

func (c *dutCommand) String() string {
	return fmt.Sprintf("DutCommand{id:%q, Cancelable:%v, CMD:%q}",
		c.JobId, c.Cancelable, strings.Join(c.apt.Args, " "))
}
