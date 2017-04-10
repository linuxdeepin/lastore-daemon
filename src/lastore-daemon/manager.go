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
	"fmt"
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

	SystemOnChanging  bool
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

func (m *Manager) PackageDesktopPath(pkgId string) string {
	return QueryDesktopFilePath(pkgId)
}

func (m *Manager) SetRegion(region string) error {
	return m.config.SetAppstoreRegion(region)
}
