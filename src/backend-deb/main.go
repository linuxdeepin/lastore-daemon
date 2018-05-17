package main

import (
	"bufio"
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	debVersion "github.com/knqyf263/go-deb-version"
	"github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusInterface     = "com.deepin.AppStore.Backend"
	dbusServiceName   = dbusInterface + ".Deb"
	dbusPath          = "/com/deepin/AppStore/Backend"
	dbusJobPathPrefix = dbusPath + "/Job"
	dbusJobInterface  = dbusInterface + ".Job"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

type Backend struct {
	service          *dbusutil.Service
	sysSigLoop       *dbusutil.SignalLoop
	lastore          *lastore.Lastore
	lastoreJobList   []dbus.ObjectPath
	lastoreJobListMu sync.Mutex
	jobs             map[dbus.ObjectPath]*Job
	PropsMu          sync.RWMutex
	JobList          []dbus.ObjectPath
	methods          *struct {
		Install               func() `in:"jobName,id" out:"job"`
		Remove                func() `in:"jobName,id" out:"job"`
		ListInstalled         func() `out:"installedInfoList"`
		QueryVersion          func() `in:"idList" out:"versionInfoList"`
		QueryDownloadSize     func() `in:"id" out:"size"`
		QueryInstallationTime func() `in:"idList" out:"installationTimeList"`
	}
}

func newBackend(service *dbusutil.Service) (*Backend, error) {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	lastoreObj := lastore.NewLastore(systemConn)
	sysSigLoop := dbusutil.NewSignalLoop(systemConn, 50)
	return &Backend{
		service:    service,
		lastore:    lastoreObj,
		sysSigLoop: sysSigLoop,
		jobs:       make(map[dbus.ObjectPath]*Job),
	}, nil
}

func (b *Backend) updatePropJobList() {
	var jobList []dbus.ObjectPath
	for _, job := range b.jobs {
		jobList = append(jobList, job.getPath())
	}
	b.JobList = jobList
	err := b.service.EmitPropertyChanged(b, "JobList", jobList)
	if err != nil {
		log.Println("warning:", err)
	}
}

func (b *Backend) init() {
	b.sysSigLoop.Start()
	b.lastore.InitSignalExt(b.sysSigLoop, true)
	b.lastore.JobList().ConnectChanged(func(hasValue bool, value []dbus.ObjectPath) {
		if !hasValue {
			return
		}
		b.lastoreJobListMu.Lock()
		defer b.lastoreJobListMu.Unlock()

		var removedJobPaths []dbus.ObjectPath
		for _, jobPath := range b.lastoreJobList {
			if !objectPathSliceContains(value, jobPath) {
				removedJobPaths = append(removedJobPaths, jobPath)
			}
		}
		b.lastoreJobList = value
		for _, jobPath := range removedJobPaths {
			b.PropsMu.Lock()
			job, ok := b.jobs[jobPath]
			b.PropsMu.Unlock()
			if ok {
				b.PropsMu.Lock()
				delete(b.jobs, jobPath)
				b.updatePropJobList()
				b.PropsMu.Unlock()

				job.destroy()

				time.AfterFunc(1*time.Second, func() {
					log.Println("remove job", job.core.Path_())
					b.service.StopExport(job)
				})
			}
		}
	})

	b.lastoreJobListMu.Lock()
	b.lastoreJobList, _ = b.lastore.JobList().Get(0)
	b.lastoreJobListMu.Unlock()
}

func objectPathSliceContains(slice []dbus.ObjectPath, path dbus.ObjectPath) bool {
	for _, v := range slice {
		if v == path {
			return true
		}
	}
	return false
}

func (*Backend) GetInterfaceName() string {
	return dbusInterface
}

func (b *Backend) addJob(jobPath dbus.ObjectPath) (dbus.ObjectPath, error) {
	log.Println("add job", jobPath)
	job, err := newJob(b, jobPath)
	if err != nil {
		return "/", err
	}

	myJobPath := job.getPath()
	err = b.service.Export(myJobPath, job)
	if err != nil {
		log.Printf("warning: failed to export job %s: %v", job.Id, err)
		return "/", err
	}

	b.PropsMu.Lock()
	b.jobs[jobPath] = job
	b.updatePropJobList()
	b.PropsMu.Unlock()
	return myJobPath, nil
}

func (b *Backend) QueryDownloadSize(id string) (int64, *dbus.Error) {
	size, err := b.lastore.PackagesDownloadSize(0, []string{id})
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	return size, nil
}

func (b *Backend) Install(jobName, id string) (dbus.ObjectPath, *dbus.Error) {
	b.service.DelayAutoQuit()
	jobPath, err := b.lastore.InstallPackage(0, jobName, id)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}

	myJobPath, err := b.addJob(jobPath)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return myJobPath, nil
}

func (b *Backend) Remove(jobName, id string) (dbus.ObjectPath, *dbus.Error) {
	b.service.DelayAutoQuit()
	jobPath, err := b.lastore.RemovePackage(0, jobName, id)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}

	myJobPath, err := b.addJob(jobPath)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return myJobPath, nil
}

func (b *Backend) ListInstalled() (result []PackageInstalledInfo, busErr *dbus.Error) {
	b.service.DelayAutoQuit()

	cmd := exec.Command("/usr/bin/dpkg-query", "--show", "-f",
		"${binary:Package}\\t${db:Status-Abbrev}\\t${Version}\\n")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}
	err = cmd.Start()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}
	defer func() {
		err = cmd.Wait()
		if err != nil {
			result = nil
			busErr = dbusutil.ToError(err)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		parts := bytes.SplitN(scanner.Bytes(), []byte{'\t'}, 3)
		if len(parts) != 3 {
			continue
		}

		if bytes.HasPrefix(parts[1], []byte("ii")) {
			result = append(result, PackageInstalledInfo{
				ID:      string(parts[0]),
				Version: string(parts[2]),
			})
		}
	}
	err = scanner.Err()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}

	return result, nil
}

type PackageInstalledInfo struct {
	ID      string
	Version string
}

func (b *Backend) QueryVersion(idList []string) (result []PackageVersionInfo,
	busErr *dbus.Error) {
	b.service.DelayAutoQuit()

	args := append([]string{"policy", "--"}, idList...)
	cmd := exec.Command("/usr/bin/apt-cache", args...)
	cmd.Env = []string{"LC_ALL=C"}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}
	err = cmd.Start()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}

	defer func() {
		err = cmd.Wait()
		if err != nil {
			result = nil
			busErr = dbusutil.ToError(err)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if line[0] != ' ' {
			// is package name line
			id := string(bytes.TrimRight(line, ":"))

			// get local version
			const installed = "Installed: "
			localVersion, err := scanVersion(scanner, installed)
			if err != nil {
				return nil, dbusutil.ToError(err)
			}

			// get remote version
			const candidate = "Candidate: "
			remoteVersion, err := scanVersion(scanner, candidate)
			if err != nil {
				return nil, dbusutil.ToError(err)
			}

			result = append(result, PackageVersionInfo{
				ID:            id,
				LocalVersion:  localVersion,
				RemoteVersion: remoteVersion,
				Upgradable:    isUpgradable(localVersion, remoteVersion),
			})
		}
	}
	return result, nil
}

func scanVersion(scanner *bufio.Scanner, versionType string) (string, error) {
	ok := scanner.Scan()
	if !ok {
		return "", errors.New("scan failed")
	}
	line := scanner.Bytes()
	idx := bytes.Index(line, []byte(versionType))
	if idx == -1 {
		return "", errors.New("not found mark")
	}
	result := string(line[idx+len(versionType):])
	if result == "(none)" {
		result = ""
	}
	return result, nil
}

func isUpgradable(localVersion, remoteVersion string) bool {
	localVer, err := debVersion.NewVersion(localVersion)
	if err != nil {
		return false
	}

	remoteVer, err := debVersion.NewVersion(remoteVersion)
	if err != nil {
		return false
	}

	return remoteVer.GreaterThan(localVer)
}

type PackageVersionInfo struct {
	ID            string
	LocalVersion  string
	RemoteVersion string
	Upgradable    bool
}

type PackageInstallationTimeInfo struct {
	ID               string
	InstallationTime int64
}

func (b *Backend) QueryInstallationTime(idList []string) (result []PackageInstallationTimeInfo,
	busErr *dbus.Error) {
	b.service.DelayAutoQuit()

	for _, id := range idList {
		t, err := getInstallationTime(id)
		if err == nil {
			result = append(result, PackageInstallationTimeInfo{
				ID:               id,
				InstallationTime: t,
			})
		} else {
			log.Printf("warning: failed to get installation time of %q\n", id)
		}
	}
	return
}

func getInstallationTime(id string) (int64, error) {
	fileInfo, err := os.Stat("/var/lib/dpkg/info/" + id + ".md5sums")
	if err != nil {
		return 0, err
	}
	sysStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("type assert failed")
	}
	t := time.Unix(int64(sysStat.Ctim.Sec), int64(sysStat.Ctim.Nsec))
	return t.Unix(), nil
}

func main() {
	service, err := dbusutil.NewSessionService()
	if err != nil {
		log.Fatal(err)
	}

	b, err := newBackend(service)
	if err != nil {
		log.Fatal(err)
	}
	b.init()

	err = service.Export(dbusPath, b)
	if err != nil {
		log.Fatal(err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		log.Fatal(err)
	}

	service.SetAutoQuitHandler(3*time.Minute, func() bool {
		b.PropsMu.Lock()
		jobCount := len(b.jobs)
		b.PropsMu.Unlock()
		return jobCount == 0
	})
	service.Wait()
}
