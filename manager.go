package main

import (
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

func (m *Manager) update(info system.ProgressInfo) {
	j := m.findJob(info.JobId)
	if j == nil {

		return
	}

	j.updateInfo(info)
	if j.Status == system.SuccessedStatus && j.next != nil {
		j.swap(j.next)
		j.next = nil
	}
}

func (m *Manager) findJob(id string) *Job {
	for _, j := range m.JobList {
		if j.Id == id {
			return j
		}
	}
	return nil
}

func (m *Manager) addJob(j *Job) {
	m.JobList = append(m.JobList, j)
	dbus.NotifyChange(m, "JobList")
}
func (m *Manager) InstallPackages(packageId string) (*Job, error) {
	j, err := NewInstallJob(packageId)
	if err != nil {
		return nil, err
	}
	m.addJob(j)
	return j, nil
}

func (m *Manager) DownloadPackages(packageId string) (*Job, error) {
	j, err := NewDownloadJob(packageId, "/dev/shm/cache")
	if err != nil {
		return nil, err
	}
	m.addJob(j)
	return j, nil
}
func (m *Manager) RemovePackages(packageId string) (*Job, error) {
	j, err := NewRemoveJob(packageId)
	if err != nil {
		return nil, err
	}
	m.addJob(j)
	return j, nil
}

func (m *Manager) StartJob(jobId string) error {
	//TODO: handled by Job
	j := m.findJob(jobId)
	if j == nil {
		return system.NotFoundError
	}
	return j.start(m.b)
}

func (m *Manager) PauseJob2(jobId string) error {
	return m.b.Pause(jobId)
}
func (m *Manager) CleanJob2(jobId string) error {
	return system.NotImplementError
}

func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		"org.deepin.lastore",
		"/org/deepin/lastore",
		"org.deepin.lastore.Manager",
	}
}

func (m *Manager) CheckPackageExists(pid string) bool {
	return m.b.CheckInstalled(pid)
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
