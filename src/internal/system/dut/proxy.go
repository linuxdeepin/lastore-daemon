package dut

import (
	"internal/system"
	"internal/system/apt"
)

type DutSystem struct {
	system.System
	cmdSet    map[string]*dutCommand
	indicator system.Indicator
}

func New() system.System {
	aptImpl := apt.New(nil, nil, nil)
	return &DutSystem{
		System: aptImpl,
	}
}

func (p *DutSystem) OptionToArgs(options map[string]string) []string {
	return p.System.OptionToArgs(options)
}

func (p *DutSystem) DownloadPackages(jobId string, packages []string, environ map[string]string, cmdArgs []string) error {
	return p.System.DownloadPackages(jobId, packages, environ, cmdArgs)
}

func (p *DutSystem) DownloadSource(jobId string, environ map[string]string, cmdArgs []string) error {
	return p.System.DownloadSource(jobId, environ, cmdArgs)
}

func (p *DutSystem) Install(jobId string, packages []string, environ map[string]string, cmdArgs []string) error {
	return p.System.Install(jobId, packages, environ, cmdArgs)
}

func (p *DutSystem) Remove(jobId string, packages []string, environ map[string]string) error {
	return p.System.Remove(jobId, packages, environ)
}

func (p *DutSystem) DistUpgrade(jobId string, environ map[string]string, cmdArgs []string) error {
	return p.System.DistUpgrade(jobId, environ, cmdArgs)
}

func (p *DutSystem) UpdateSource(jobId string, environ map[string]string, cmdArgs []string) error {
	return p.System.UpdateSource(jobId, environ, cmdArgs)
}

func (p *DutSystem) Clean(jobId string) error {
	return p.System.Clean(jobId)
}

func (p *DutSystem) Abort(jobId string) error {
	return p.System.Abort(jobId)
}

func (p *DutSystem) AbortWithFailed(jobId string) error {
	return p.System.AbortWithFailed(jobId)
}

func (p *DutSystem) AttachIndicator(indicator system.Indicator) {
	p.System.AttachIndicator(indicator)
	p.indicator = indicator
}

func (p *DutSystem) FixError(jobId string, errType string, environ map[string]string, cmdArgs []string) error {
	return p.System.FixError(jobId, errType, environ, cmdArgs)
}

func (p *DutSystem) CheckSystem(jobId string, checkType string) error {
	return nil
}
