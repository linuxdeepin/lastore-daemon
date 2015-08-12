package main

import (
	"log"

	"./system"
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
	CacheDir string
	JobList  []*Job
	b        system.System
}

func NewManager(b system.System) *Manager {
	m := &Manager{
		Version:  "0.1",
		CacheDir: "/dev/shm",
		JobList:  nil,
		b:        b,
	}
	b.AttachIndicator(m.update)
	return m
}

func (m *Manager) update(jobId string, progress float64, desc string, status system.Status) {
	for _, j := range m.JobList {
		if j.Id == jobId {
			if progress != -1 {
				j.Progress = progress
			}
			j.Description = desc
			j.Status = string(status)
			log.Printf("JobId: %q(%q)  ----> progress:%f ----> msg:%q, status:%q\n", jobId, j.PackageId, progress, desc, j.Status)
			dbus.NotifyChange(j, "Progress")
			dbus.NotifyChange(j, "Description")
			dbus.NotifyChange(j, "Status")

		}
	}
}

func (m *Manager) InstallPackages(packages []string) *Job {
	j, _ := NewInstallJob(packages[0], "/dev/shm/cache")
	id := m.b.Install(packages)
	j.Id = id
	m.JobList = append(m.JobList, j)
	return j
}

func (m *Manager) DownloadPackages(packages []string) *Job {
	j, _ := NewDownloadJob(packages[0], "/dev/shm/cache")
	id := m.b.Download(packages)
	j.Id = id
	m.JobList = append(m.JobList, j)
	return j
}
func (m *Manager) RemovePackages(packages []string) *Job {
	j, _ := NewRemoveJob(packages[0])
	id := m.b.Remove(packages)
	j.Id = id
	m.JobList = append(m.JobList, j)
	return j
}

func (m *Manager) StartJob(jobId string) bool {
	r := m.b.Start(jobId)
	return r
}
func (m *Manager) PauseJob2(jobId string) bool {
	return m.b.Pause(jobId)
}
func (m *Manager) CleanJob2(jobId string) bool {
	return false
}

func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		"org.deepin.lastore",
		"/org/deepin/lastore",
		"org.deepin.lastore.Manager",
	}
}

func (m *Manager) CheckPackageExists(pid string) bool {
	return m.b.CheckPackageExists(pid)
}
func (m *Manager) GetPackageDesktopPath1(pid string) string {
	return GetPackageDesktopPath(pid)
}
func (m *Manager) GetPackageCategory1(pid string) string {
	return GetPackageCategory(pid)
}

func GetPackageDesktopPath(pid string) string {
	return "/usr/share/applications/deepin-movie.desktop"
}
func GetPackageCategory(pid string) string {
	return "others"
}
