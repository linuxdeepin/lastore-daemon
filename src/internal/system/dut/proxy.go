package dut

import (
	"internal/system"
	"internal/system/apt"
)

type DutSystem struct {
	apt.APTSystem
}

func NewSystem(systemSourceList []string, nonUnknownList []string, otherList []string) system.System {
	aptImpl := apt.New(systemSourceList, nonUnknownList, otherList)
	return &DutSystem{
		APTSystem: aptImpl,
	}
}

func (p *DutSystem) OptionToArgs(options map[string]string) []string {
	var args []string
	for key, value := range options { // dut 命令执行参数
		args = append(args, key)
		args = append(args, value)
	}
	return args
}

func (p *DutSystem) DistUpgrade(jobId string, environ map[string]string, cmdArgs []string) error {
	c := newDUTCommand(p, jobId, system.DistUpgradeJobType, p.Indicator, cmdArgs)
	c.SetEnv(environ)
	return c.Start()
}

func (p *DutSystem) FixError(jobId string, errType string, environ map[string]string, cmdArgs []string) error {
	c := newDUTCommand(p, jobId, system.FixErrorJobType, p.Indicator, append([]string{errType}, cmdArgs...))
	c.SetEnv(environ)
	return c.Start()
}

func (p *DutSystem) CheckSystem(jobId string, checkType string, environ map[string]string, cmdArgs []string) error {
	c := newDUTCommand(p, jobId, system.CheckSystemJobType, p.Indicator, cmdArgs)
	c.SetEnv(environ)
	return c.Start()
}

func (p *DutSystem) CheckDepends(jobId string, checkType string, environ map[string]string, cmdArgs []string) error {
	c := newDUTCommand(p, jobId, system.CheckDependsJobType, p.Indicator, nil)
	c.SetEnv(environ)
	return c.Start()
}
