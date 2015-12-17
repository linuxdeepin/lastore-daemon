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

	updated bool
}

func NewManager(b system.System, c *Config) *Manager {
	m := &Manager{
		config:              c,
		b:                   b,
		SystemArchitectures: b.SystemArchitectures(),
	}
	m.jobManager = NewJobManager(b, m.updateJobList)

	go m.jobManager.Dispatch()

	m.updatableApps()
	m.updateJobList()

	// Force notify changed at the first time
	dbus.NotifyChange(m, "SystemOnChanging")
	dbus.NotifyChange(m, "JobList")
	dbus.NotifyChange(m, "UpgradableApps")

	m.loopUpdate()
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

func (m *Manager) InstallPackage(jobName string, packages string) (*Job, error) {
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

	go Touch(string(m.SystemArchitectures[0]), m.config.AppstoreRegion, pList...)
	job, err := m.jobManager.CreateJob(jobName, system.InstallJobType, pList)
	if err != nil {
		log.Warnf("InstallPackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) DownloadPackage(jobName string, packages string) (*Job, error) {
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
	job, err := m.jobManager.CreateJob(jobName, system.DownloadJobType, pList)
	if err != nil {
		log.Warnf("DownloadPackage %q error: %v\n", packages, err)
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
	if len(m.UpgradableApps) == 0 {
		return nil, system.ResourceExitError
	}

	m.checkNeedUpdate()
	m.do.Lock()
	defer m.do.Unlock()

	var updateJobIds []string
	for _, job := range m.JobList {
		if job.Type == system.DistUpgradeJobType {
			return nil, system.ResourceExitError
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
	return m.b.CheckInstallable(pkgId)
}

func (m *Manager) PackageExists(pkgId string) bool {
	return m.b.CheckInstalled(pkgId)
}

func (m *Manager) PackagesDownloadSize(packages []string) int64 {
	m.checkNeedUpdate()
	m.do.Lock()
	defer m.do.Unlock()

	if len(packages) == 1 && m.PackageExists(packages[0]) {
		return SizeDownloaded
	}
	return int64(QueryPackageDownloadSize(packages...))
}

func (m *Manager) PackageDesktopPath(pkgId string) string {
	m.do.Lock()
	defer m.do.Unlock()

	r := QueryDesktopPath(pkgId)
	if r != "" {
		return r
	}
	return QueryDesktopPath(QueryPackageSameNameDepends(pkgId)...)
}

func (m *Manager) SetRegion(region string) error {
	m.do.Lock()
	defer m.do.Unlock()

	return m.config.SetAppstoreRegion(region)
}
