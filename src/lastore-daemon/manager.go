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
	log "github.com/cihub/seelog"
	"internal/system"
	"pkg.deepin.io/lib/dbus"
	"strings"
	"sync"
)

type Manager struct {
	do     sync.Mutex
	b      system.System
	config *Config

	JobList    []*Job
	jobManager *JobManager

	SystemArchitectures []system.Architecture

	UpgradableApps []string

	SystemOnChanging bool
	inhibitFd        dbus.UnixFD

	updated bool

	//TODO: remove this. It should be record in com.deepin.Accounts
	cachedLocale map[uint64]string
}

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
	}

	m.jobManager = NewJobManager(b, m.updateJobList)

	go m.jobManager.Dispatch()

	m.updateJobList()

	// Force notify changed at the first time
	dbus.NotifyChange(m, "SystemOnChanging")
	dbus.NotifyChange(m, "JobList")
	dbus.NotifyChange(m, "UpgradableApps")

	return m
}

func (m *Manager) checkNeedUpdate() {
	m.do.Lock()
	if m.updated {
		m.do.Unlock()
		return
	}
	m.updated = true
	m.do.Unlock()

	m.UpdateSource()
}

func (m *Manager) UpdatePackage(jobName string, packages string) (*Job, error) {
	m.checkNeedUpdate()
	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob(jobName, system.UpdateJobType, strings.Fields(packages))
	if err != nil {
		log.Warnf("UpdatePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) InstallPackage(msg dbus.DMessage, jobName string, packages string) (*Job, error) {
	locale, ok := m.cachedLocale[uint64(msg.GetSenderUID())]
	if !ok {
		log.Warnf("Can't find lang information from :%v %v\n", msg)
		return m.installPackage(jobName, packages)
	}

	localePkgs := QueryEnhancedLocalePackages(system.QueryPackageInstallable, locale, strings.Fields(packages)...)
	if len(localePkgs) != 0 {
		log.Infof("Follow locale packages will be installed:%v\n", localePkgs)
	}
	pkgs := strings.Join(append(strings.Fields(packages), localePkgs...), " ")
	return m.installPackage(jobName, pkgs)
}

func (m *Manager) installPackage(jobName string, packages string) (*Job, error) {
	m.checkNeedUpdate()
	m.do.Lock()
	defer m.do.Unlock()

	pList := strings.Fields(packages)

	installedN := 0
	for _, pkg := range pList {
		if m.PackageExists(pkg) {
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
	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob(jobName, system.RemoveJobType, strings.Fields(packages))
	if err != nil {
		log.Warnf("RemovePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) UpdateSource() (*Job, error) {
	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob("", system.UpdateSourceJobType, nil)
	if err != nil {
		log.Warnf("UpdateSource error: %v\n", err)
	}
	return job, err
}

func (m *Manager) DistUpgrade() (*Job, error) {
	m.checkNeedUpdate()
	m.do.Lock()
	defer m.do.Unlock()

	m.updateJobList()
	if len(m.UpgradableApps) == 0 {
		return nil, system.NotFoundError
	}

	var updateJobIds []string
	for _, job := range m.jobManager.List() {
		if job.Type == system.DistUpgradeJobType {
			err := m.StartJob(job.Id)
			log.Warnf("Using exist DistUpgradeJob %v --> %v\n", job, err)
			return job, err
		}
		if job.Type == system.UpdateJobType && job.Status != system.RunningStatus {
			updateJobIds = append(updateJobIds, job.Id)
		}
	}

	for _, jobId := range updateJobIds {
		m.CleanJob(jobId)
	}

	job, err := m.jobManager.CreateJob("", system.DistUpgradeJobType, m.UpgradableApps)
	if err != nil {
		log.Warnf("DistUpgrade error: %v\n", err)
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

func (m *Manager) PackageInstallable(pkgId string) bool {
	return system.QueryPackageInstallable(pkgId)
}

func (m *Manager) PackageExists(pkgId string) bool {
	return system.QueryPackageInstalled(pkgId)
}

func (m *Manager) PackagesDownloadSize(packages []string) (int64, error) {
	m.checkNeedUpdate()
	m.do.Lock()
	defer m.do.Unlock()

	if len(packages) == 1 && m.PackageExists(packages[0]) {
		return system.SizeDownloaded, nil
	}

	s, err := system.QueryPackageDownloadSize(packages...)
	if err != nil {
		log.Warnf("PackagesDownloadSize(%q) %v\n", strings.Join(packages, " "), err)
	}
	return int64(s), err
}

func (m *Manager) PackageDesktopPath(pkgId string) string {
	m.do.Lock()
	defer m.do.Unlock()

	return QueryDesktopFilePath(pkgId)
}

func (m *Manager) SetRegion(region string) error {
	m.do.Lock()
	defer m.do.Unlock()

	return m.config.SetAppstoreRegion(region)
}
