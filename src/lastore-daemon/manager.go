package main

import (
	"fmt"
	"internal/system"
	"pkg.deepin.io/lib/dbus"
	"sort"
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

	// the main architecture
	SystemArchitectures []system.Architecture
}

func NewManager(b system.System) *Manager {
	m := &Manager{
		Version:             "0.1",
		cacheDir:            "/dev/shm",
		b:                   b,
		SystemArchitectures: b.SystemArchitectures(),
	}
	b.AttachIndicator(m.update)
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
	}
}

func (m *Manager) InstallPackage(packageId string, region string) (*Job, error) {
	j, err := NewInstallJob(packageId, region)
	if err != nil {
		return nil, err
	}
	err = m.addJob(j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (m *Manager) DownloadPackage(packageId string, region string) (*Job, error) {
	j, err := NewDownloadJob(packageId, region)
	if err != nil {
		return nil, err
	}
	err = m.addJob(j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (m *Manager) RemovePackage(packageId string) (*Job, error) {
	j, err := NewRemoveJob(packageId)
	if err != nil {
		return nil, err
	}
	err = m.addJob(j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (m *Manager) StartJob(jobId string) error {
	//TODO: handled by Job
	j, err := m.JobList.Find(jobId)
	if err != nil {
		return err
	}
	return j.start(m.b)
}

func (m *Manager) PauseJob2(jobId string) error {
	return m.b.Pause(jobId)
}

func (m *Manager) CleanJob(jobId string) error {
	j, err := m.JobList.Find(jobId)
	if err != nil {
		return err
	}
	if j.Status == system.FailedStatus || j.Status == system.SuccessedStatus {
		if m.removeJobById(jobId) {
			return nil
		}
		return fmt.Errorf("Couldn't found the job by %q", jobId)
	}
	return fmt.Errorf("Failed CleanJob.")
}

func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		"org.deepin.lastore",
		"/org/deepin/lastore",
		"org.deepin.lastore.Manager",
	}
}

func (m *Manager) PackageExists(pid string) bool {
	return m.b.CheckInstalled(pid)
}

func (m *Manager) PackageDownloadSize(pid string) int64 {
	return int64(GuestPackageDownloadSize(pid))
}

func (m *Manager) PackageDesktopPath1(pid string) string {
	return GetPackageDesktopPath(pid)
}

func (m *Manager) PackageCategory1(pid string) string {
	return GetPackageCategory(pid)
}

func GetPackageDesktopPath(pid string) string {
	return "/usr/share/applications/deepin-movie.desktop"
}
func GetPackageCategory(pid string) string {
	return "others"
}

func (m *Manager) removeJobById(id string) bool {
	j, err := m.JobList.Find(id)
	if err != nil {
		return false
	}
	dbus.UnInstallObject(j)

	l, err := m.JobList.Remove(id)
	if err != nil {
		return false
	}
	m.JobList = l

	sort.Sort(m.JobList)
	dbus.NotifyChange(m, "JobList")
	return true
}

func (m *Manager) addJob(j *Job) error {
	l, err := m.JobList.Add(j)
	if err != nil {
		return err
	}
	m.JobList = l

	sort.Sort(m.JobList)
	dbus.NotifyChange(m, "JobList")
	return nil
}
