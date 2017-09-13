/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"internal/system"
	"internal/utils"

	log "github.com/cihub/seelog"
	"pkg.deepin.io/lib/dbus"
)

type Manager struct {
	do     sync.Mutex
	b      system.System
	config *Config

	JobList    []*Job
	jobManager *JobManager

	SystemArchitectures []system.Architecture

	UpgradableApps []string

	SystemOnChanging   bool
	AutoClean          bool
	autoCleanCfgChange chan struct{}

	inhibitFd         dbus.UnixFD
	sourceUpdatedOnce bool
	//TODO: remove this. It should be record in com.deepin.Accounts
	cachedLocale map[uint64]string
}

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func NewManager(b system.System, c *Config) *Manager {
	archs, err := system.SystemArchitectures()
	if err != nil {
		log.Errorf("Can't detect system supported architectures %v\n", err)
		return nil
	}

	m := &Manager{
		config:              c,
		b:                   b,
		SystemArchitectures: archs,
		cachedLocale:        make(map[uint64]string),
		inhibitFd:           -1,
		AutoClean:           c.AutoClean,
	}

	m.jobManager = NewJobManager(b, m.updateJobList)

	go m.jobManager.Dispatch()

	m.updateJobList()

	// Force notify changed at the first time
	dbus.NotifyChange(m, "SystemOnChanging")
	dbus.NotifyChange(m, "JobList")
	dbus.NotifyChange(m, "UpgradableApps")

	go m.loopCheck()
	return m
}

func NormalizePackageNames(s string) ([]string, error) {
	r := strings.Fields(s)
	if s == "" || len(r) == 0 {
		return nil, fmt.Errorf("Empty value")
	}
	return r, nil
}

func (m *Manager) UpdatePackage(jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("Invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob(jobName, system.UpdateJobType, pkgs)
	if err != nil {
		log.Warnf("UpdatePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) InstallPackage(msg dbus.DMessage, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("Invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	locale, ok := m.cachedLocale[uint64(msg.GetSenderUID())]
	if !ok {
		log.Warnf("Can't find lang information from :%v %v\n", msg)
		return m.installPackage(jobName, packages)
	}

	localePkgs := QueryEnhancedLocalePackages(system.QueryPackageInstallable, locale, pkgs...)
	if len(localePkgs) != 0 {
		log.Infof("Follow locale packages will be installed:%v\n", localePkgs)
	}

	return m.installPackage(jobName, strings.Join(append(strings.Fields(packages), localePkgs...), " "))
}

func (m *Manager) installPackage(jobName string, packages string) (*Job, error) {
	pList := strings.Fields(packages)

	installedN := 0
	for _, pkg := range pList {
		if system.QueryPackageInstalled(pkg) {
			installedN++
		}
	}
	if installedN == len(pList) {
		return nil, system.ResourceExitError
	}

	job, err := m.jobManager.CreateJob(jobName, system.InstallJobType, pList)
	if err != nil {
		log.Warnf("InstallPackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) RemovePackage(jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("Invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob(jobName, system.RemoveJobType, pkgs)
	if err != nil {
		log.Warnf("RemovePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) ensureUpdateSourceOnce() {
	if !m.sourceUpdatedOnce {
		m.UpdateSource()
	}
}

func (m *Manager) UpdateSource() (*Job, error) {
	m.do.Lock()
	defer m.do.Unlock()
	m.sourceUpdatedOnce = true

	job, err := m.jobManager.CreateJob("", system.UpdateSourceJobType, nil)
	if err != nil {
		log.Warnf("UpdateSource error: %v\n", err)
	}
	return job, err
}

func (m *Manager) cancelAllJob() error {
	var updateJobIds []string
	for _, job := range m.jobManager.List() {
		if job.Type == system.UpdateJobType && job.Status != system.RunningStatus {
			updateJobIds = append(updateJobIds, job.Id)
		}
	}

	for _, jobId := range updateJobIds {
		err := m.jobManager.CleanJob(jobId)
		if err != nil {
			log.Warnf("CleanJob %q error: %v\n", jobId, err)
		}
		return err
	}
	return nil
}

func (m *Manager) DistUpgrade() (*Job, error) {
	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	m.updateJobList()
	if len(m.UpgradableApps) == 0 {
		return nil, system.NotFoundError
	}

	job, err := m.jobManager.CreateJob("", system.DistUpgradeJobType, m.UpgradableApps)
	if err != nil {
		log.Warnf("DistUpgrade error: %v\n", err)
		return nil, err
	}

	m.cancelAllJob()
	return job, err
}

func (m *Manager) PrepareDistUpgrade() (*Job, error) {
	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	m.updateJobList()

	if len(m.UpgradableApps) == 0 {
		return nil, system.NotFoundError
	}
	if s, err := system.QueryPackageDownloadSize(m.UpgradableApps...); err == nil && s == 0 {
		return nil, system.NotFoundError
	}

	job, err := m.jobManager.CreateJob("", system.PrepareDistUpgradeJobType, m.UpgradableApps)
	if err != nil {
		log.Warnf("PrepareDistUpgrade error: %v\n", err)
		return nil, err
	}
	return job, err
}

func (m *Manager) StartJob(jobId string) error {
	m.do.Lock()
	defer m.do.Unlock()

	err := m.jobManager.MarkStart(jobId)
	if err != nil {
		log.Warnf("StartJob %q error: %v\n", jobId, err)
	}
	return err
}
func (m *Manager) PauseJob(jobId string) error {
	m.do.Lock()
	defer m.do.Unlock()

	err := m.jobManager.PauseJob(jobId)
	if err != nil {
		log.Warnf("PauseJob %q error: %v\n", jobId, err)
	}
	return err
}
func (m *Manager) CleanJob(jobId string) error {
	m.do.Lock()
	defer m.do.Unlock()

	err := m.jobManager.CleanJob(jobId)
	if err != nil {
		log.Warnf("CleanJob %q error: %v\n", jobId, err)
	}
	return err
}

func (m *Manager) PackagesDownloadSize(packages []string) (int64, error) {
	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	s, err := system.QueryPackageDownloadSize(packages...)
	if err != nil || s == system.SizeUnknown {
		log.Warnf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), s, err)
	}
	return int64(s), err
}

func (m *Manager) PackageInstallable(pkgId string) bool {
	return system.QueryPackageInstallable(pkgId)
}

func (m *Manager) PackageExists(pkgId string) bool {
	return system.QueryPackageInstalled(pkgId)
}

// TODO: Remove this API
func (m *Manager) PackageDesktopPath(pkgId string) string {
	p, err := utils.RunCommand("/usr/bin/lastore-tools", "querydesktop", pkgId)
	if err != nil {
		log.Warnf("QueryDesktopPath failed: %q\n", err)
		return ""
	}
	return p
}

func (m *Manager) SetRegion(region string) error {
	return m.config.SetAppstoreRegion(region)
}

func (m *Manager) SetAutoClean(enable bool) error {
	if m.AutoClean == enable {
		return nil
	}

	// save the config to disk
	err := m.config.SetAutoClean(enable)
	if err != nil {
		return err
	}

	m.AutoClean = enable
	m.autoCleanCfgChange <- struct{}{}
	dbus.NotifyChange(m, "AutoClean")
	return nil
}

var errAptRunning = errors.New("apt or apt-get is running")

func (m *Manager) CleanArchives() (*Job, error) {
	m.do.Lock()
	defer m.do.Unlock()

	aptRunning, err := isAptRunning()
	if err != nil {
		return nil, err
	}
	log.Debug("apt running: ", aptRunning)

	if aptRunning {
		return nil, errAptRunning
	}

	job, err := m.jobManager.CreateJob("", system.CleanJobType, nil)
	if err != nil {
		log.Warnf("CleanArchives error: %v", err)
	}
	return job, err
}

func isAptRunning() (bool, error) {
	cmd := exec.Command("pgrep", "-u", "root", "-x", "apt|apt-get")
	err := cmd.Run()
	if err != nil {
		log.Debugf("isAptRunning err: %#v", err)
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m *Manager) loopCheck() {
	m.autoCleanCfgChange = make(chan struct{})
	const checkInterval = time.Second * 100

	doClean := func() {
		log.Debug("call doClean")

		_, err := m.CleanArchives()
		if err == errAptRunning {
			log.Info("apt is running, waiting for the next chance")
			return
		} else if err != nil {
			log.Warnf("CleanArchives failed: %v", err)
		}
		m.config.UpdateLastCleanTime()
	}

	calcDelay := func() time.Duration {
		elapsed := time.Now().Sub(m.config.LastCleanTime)
		return m.config.CleanInterval - elapsed
	}

	for {
		select {
		case <-m.autoCleanCfgChange:
			// auto clean config changed
			log.Debug("autoclean config changed")
			continue
		case <-time.After(checkInterval):
			log.Debug("tick")
			if m.AutoClean {
				remaind := calcDelay()
				log.Debugf("autoclean remaind %v", remaind)
				if remaind < 0 {
					doClean()
				}
			} else {
				log.Debug("autoclean disabled")
			}
		}
	}
}
