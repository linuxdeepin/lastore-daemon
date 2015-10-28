package main

import (
	"fmt"
	"internal/system"
	"log"
	"pkg.deepin.io/lib/dbus"
)

const (
	DownloadJobType = "download"
	InstallJobType  = "install"
	RemoveJobType   = "remove"
)

type CMD string

const (
	StopCMD  CMD = "stop"
	StartCMD     = "start"
	PauseCMD     = "pause"
)

type Manager struct {
	Version  string
	cacheDir string
	JobList  JobList
	b        system.System

	SystemArchitectures []system.Architecture

	UpgradableApps  []string
	upgradableInfos []system.UpgradeInfo
}

func NewManager(b system.System) *Manager {
	m := &Manager{
		Version:             "0.1",
		cacheDir:            "/dev/shm",
		b:                   b,
		SystemArchitectures: b.SystemArchitectures(),
	}
	b.AttachIndicator(m.update)
	m.refreshUpgradableApps()
	return m
}

func (m *Manager) update(info system.ProgressInfo) {
	j, err := m.JobList.Find(info.JobId)
	if err != nil {
		return
	}

	j.updateInfo(info)
	if j.Status == system.SuccessedStatus && j.next != nil {
		j.swap(j.next)
		j.next = nil
		m.StartJob(j.Id)

	}
	if j.Status != system.ReadyStatus && j.Status != system.RunningStatus {
		m.refreshUpgradableApps()
	}
}

func (m *Manager) do(jobType string, packageId string, region string) (*Job, error) {
	var j *Job
	switch jobType {
	case DownloadJobType:
		j = NewDownloadJob(packageId, region)
	case InstallJobType:
		j = NewInstallJob(packageId, region)
	case RemoveJobType:
		j = NewRemoveJob(packageId)
	}
	err := m.addJob(j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (m *Manager) InstallPackage(packageId string, region string) (*Job, error) {
	return m.do(InstallJobType, packageId, region)
}

func (m *Manager) DownloadPackage(packageId string, region string) (*Job, error) {
	return m.do(DownloadJobType, packageId, region)
}

func (m *Manager) RemovePackage(packageId string) (*Job, error) {
	return m.do(RemoveJobType, packageId, "")
}

func (m *Manager) StartJob(jobId string) error {
	j, err := m.JobList.Find(jobId)
	if err != nil {
		return err
	}
	return j.start(m.b)
}

func (m *Manager) removeJob(id string) error {
	j, err := m.JobList.Find(id)
	if err != nil {
		return err
	}
	dbus.UnInstallObject(j)

	l, err := m.JobList.Remove(id)
	if err != nil {
		return err
	}
	m.JobList = l

	dbus.NotifyChange(m, "JobList")
	return nil
}

func (m *Manager) addJob(j *Job) error {
	l, err := m.JobList.Add(j)
	if err != nil {
		return err
	}
	m.JobList = l

	dbus.NotifyChange(m, "JobList")
	return nil
}

func (m *Manager) PauseJob2(jobId string) error {
	return m.b.Pause(jobId)
}

func (m *Manager) CleanJob(jobId string) error {
	j, err := m.JobList.Find(jobId)
	if err != nil {
		return err
	}

	if j.Status != system.FailedStatus && j.Status != system.SuccessedStatus {
		return fmt.Errorf("The status of job %q is not cleanable", jobId)
	}

	err = m.removeJob(jobId)
	if err != nil {
		return fmt.Errorf("Internal error find the job %q, but can't remove it. (%v)", jobId, err)
	}
	return nil
}

func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "org.deepin.lastore",
		ObjectPath: "/org/deepin/lastore",
		Interface:  "org.deepin.lastore.Manager",
	}
}

func (m *Manager) PackageExists(packageId string) bool {
	log.Println("Checking package exists...", packageId)
	return m.b.CheckInstalled(packageId)
}

func (m *Manager) PackageDownloadSize(packageId string) int64 {
	return int64(GuestPackageDownloadSize(packageId))
}

func (m *Manager) PackagesDownloadSize(packages []string) int64 {
	return int64(GuestPackageDownloadSize(packages...))
}

func (m *Manager) PackageDesktopPath(packageId string) string {
	r, _ := QueryDesktopPath(packageId)
	return r
}

func (m *Manager) PackageCategory1(packageId string) string {
	return GetPackageCategory(packageId)
}

func GetPackageCategory(packageId string) string {
	return "others"
}
