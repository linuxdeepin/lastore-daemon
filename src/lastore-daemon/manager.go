package main

import (
	"fmt"
	"internal/system"
	"log"
	"pkg.deepin.io/lib/dbus"
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

	UpgradableApps []string
	updater        *Updater
}

func NewManager(b system.System) *Manager {
	m := &Manager{
		Version:             "0.1",
		cacheDir:            "/dev/shm",
		b:                   b,
		SystemArchitectures: b.SystemArchitectures(),
	}
	b.AttachIndicator(m.update)
	m.updater = NewUpdater(b)
	m.updatableApps()
	return m
}

func (m *Manager) updatableApps() {
	apps := UpdatableNames(m.b.UpgradeInfo())
	changed := len(apps) != len(m.UpgradableApps)
	if !changed {
		for i, app := range apps {
			if m.UpgradableApps[i] != app {
				changed = true
				break
			}
		}
	}
	if changed {
		m.UpgradableApps = apps
		dbus.NotifyChange(m, "UpgradableApps")
	}
}

func (m *Manager) update(info system.ProgressInfo) {
	j, err := m.JobList.Find(info.JobId)
	if err != nil {
		return
	}

	j.updateInfo(info)
	if j.Status == system.SucceedStatus && j.next != nil {
		j.swap(j.next)
		j.next = nil
		m.StartJob(j.Id)
	}
	if j.Status != system.ReadyStatus && j.Status != system.RunningStatus {
		go m.updatableApps()
	}
}

func (m *Manager) do(jobType string, packageId string, region string) (*Job, error) {
	var j *Job
	switch jobType {
	case system.DownloadJobType:
		j = NewDownloadJob(packageId, region)
	case system.InstallJobType:
		j = NewInstallJob(packageId, region)
	case system.RemoveJobType:
		j = NewRemoveJob(packageId)
	case system.DistUpgradeJobType:
		j = NewDistUpgradeJob()
	}
	err := m.addJob(j)
	if err != nil {
		return nil, err
	}
	fmt.Println("JobID:", j.Id)
	return j, nil
}

func (m *Manager) InstallPackage(packageId string, region string) (*Job, error) {
	return m.do(system.InstallJobType, packageId, region)
}

func (m *Manager) DownloadPackage(packageId string, region string) (*Job, error) {
	return m.do(system.DownloadJobType, packageId, region)
}

func (m *Manager) RemovePackage(packageId string) (*Job, error) {
	return m.do(system.RemoveJobType, packageId, "")
}

func (m *Manager) DistUpgrade3() (*Job, error) {
	return m.do(system.DistUpgradeJobType, "", "")
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

	if j.Status == system.RunningStatus {
		return fmt.Errorf("The job %q is running, it can't be cleaned.", jobId)
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
