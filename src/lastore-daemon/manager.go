/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/procfs"

	log "github.com/cihub/seelog"
)

type Manager struct {
	service *dbusutil.Service
	do      sync.Mutex
	b       system.System
	config  *Config

	PropsMu sync.RWMutex
	// dbusutil-gen: equal=nil
	JobList    []dbus.ObjectPath
	jobList    []*Job
	jobManager *JobManager

	// dbusutil-gen: ignore
	SystemArchitectures []system.Architecture

	// dbusutil-gen: equal=nil
	UpgradableApps []string

	SystemOnChanging   bool
	AutoClean          bool
	autoCleanCfgChange chan struct{}

	inhibitFd         dbus.UnixFD
	sourceUpdatedOnce bool

	methods *struct {
		FixError             func() `in:"errType" out:"job"`
		CleanArchives        func() `out:"job"`
		CleanJob             func() `in:"jobId"`
		StartJob             func() `in:"jobId"`
		PauseJob             func() `in:"jobId"`
		InstallPackage       func() `in:"jobName,packages" out:"job"`
		RemovePackage        func() `in:"jobName,packages" out:"job"`
		UpdatePackage        func() `in:"jobName,packages" out:"job"`
		UpdateSource         func() `out:"job"`
		DistUpgrade          func() `out:"job"`
		PrepareDistUpgrade   func() `out:"job"`
		PackageDesktopPath   func() `in:"pkgId" out:"desktopPath"`
		PackagesDownloadSize func() `in:"packages" out:"size"`
		PackageExists        func() `in:"pkgId" out:"exist"`
		PackageInstallable   func() `in:"pkgId" out:"installable"`
		SetAutoClean         func() `in:"enable"`
		SetRegion            func() `in:"region"`
		SetLogger            func() `in:"levels,format,output"`
	}
}

/*
NOTE: Most of export function of Manager will hold the lock,
so don't invoke they in inner functions
*/

func NewManager(service *dbusutil.Service, b system.System, c *Config) *Manager {
	archs, err := system.SystemArchitectures()
	if err != nil {
		log.Errorf("Can't detect system supported architectures %v\n", err)
		return nil
	}

	m := &Manager{
		service:             service,
		config:              c,
		b:                   b,
		SystemArchitectures: archs,
		inhibitFd:           -1,
		AutoClean:           c.AutoClean,
	}

	m.jobManager = NewJobManager(service, b, m.updateJobList)
	go m.jobManager.Dispatch()

	m.updateJobList()

	// Force notify changed at the first time
	m.emitPropChangedSystemOnChanging(m.SystemOnChanging)
	m.emitPropChangedJobList(m.JobList)
	m.emitPropChangedUpgradableApps(m.UpgradableApps)

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

func makeEnvironWithSender(service *dbusutil.Service, sender dbus.Sender) (map[string]string, error) {
	environ := make(map[string]string)

	pid, err := service.GetConnPID(string(sender))
	if err != nil {
		return nil, err
	}

	p := procfs.Process(pid)
	envVars, err := p.Environ()
	if err != nil {
		log.Warnf("failed to get process %d environ: %v", p, err)
	} else {
		environ["DISPLAY"] = envVars.Get("DISPLAY")
		environ["XAUTHORITY"] = envVars.Get("XAUTHORITY")
		environ["DEEPIN_LASTORE_LANG"] = getLang(envVars)
	}
	return environ, nil
}

func getUsedLang(environ map[string]string) string {
	return environ["DEEPIN_LASTORE_LANG"]
}

func getLang(envVars procfs.EnvVars) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGE", "LANG"} {
		value := envVars.Get(name)
		if value != "" {
			return value
		}
	}
	return ""
}

func (m *Manager) updatePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob(jobName, system.UpdateJobType, pkgs, environ)
	if err != nil {
		log.Warnf("UpdatePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) UpdatePackage(sender dbus.Sender, jobName string, packages string) (dbus.ObjectPath,
	*dbus.Error) {
	job, err := m.updatePackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) installPackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()

	lang := getUsedLang(environ)
	if lang == "" {
		log.Warn("failed to get lang")
		return m.installPkg(jobName, packages, environ)
	}

	localePkgs := QueryEnhancedLocalePackages(system.QueryPackageInstallable, lang, pkgs...)
	if len(localePkgs) != 0 {
		log.Infof("Follow locale packages will be installed:%v\n", localePkgs)
	}

	return m.installPkg(jobName, strings.Join(append(strings.Fields(packages), localePkgs...), " "), environ)
}

func (m *Manager) InstallPackage(sender dbus.Sender, jobName string, packages string) (dbus.ObjectPath,
	*dbus.Error) {
	job, err := m.installPackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) installPkg(jobName, packages string, environ map[string]string) (*Job, error) {
	pList := strings.Fields(packages)
	job, err := m.jobManager.CreateJob(jobName, system.InstallJobType, pList, environ)
	if err != nil {
		log.Warnf("installPackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) removePackage(sender dbus.Sender, jobName string, packages string) (*Job, error) {
	pkgs, err := NormalizePackageNames(packages)
	if err != nil {
		return nil, fmt.Errorf("invalid packages arguments %q : %v", packages, err)
	}

	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()

	job, err := m.jobManager.CreateJob(jobName, system.RemoveJobType, pkgs, environ)
	if err != nil {
		log.Warnf("removePackage %q error: %v\n", packages, err)
	}
	return job, err
}

func (m *Manager) RemovePackage(sender dbus.Sender, jobName string, packages string) (dbus.ObjectPath,
	*dbus.Error) {
	job, err := m.removePackage(sender, jobName, packages)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) ensureUpdateSourceOnce() {
	if !m.sourceUpdatedOnce {
		m.UpdateSource()
	}
}

func (m *Manager) updateSource() (*Job, error) {
	m.do.Lock()
	defer m.do.Unlock()
	m.sourceUpdatedOnce = true

	job, err := m.jobManager.CreateJob("", system.UpdateSourceJobType, nil, nil)
	if err != nil {
		log.Warnf("UpdateSource error: %v\n", err)
	}
	return job, err
}

func (m *Manager) UpdateSource() (dbus.ObjectPath, *dbus.Error) {
	job, err := m.updateSource()
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
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

func (m *Manager) DistUpgrade(sender dbus.Sender) (dbus.ObjectPath, *dbus.Error) {
	job, err := m.distUpgrade(sender)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) distUpgrade(sender dbus.Sender) (*Job, error) {
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()

	m.updateJobList()
	if len(m.UpgradableApps) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}

	job, err := m.jobManager.CreateJob("", system.DistUpgradeJobType, m.UpgradableApps, environ)
	if err != nil {
		log.Warnf("DistUpgrade error: %v\n", err)
		return nil, err
	}

	m.cancelAllJob()
	return job, err
}

func (m *Manager) PrepareDistUpgrade() (dbus.ObjectPath, *dbus.Error) {
	job, err := m.prepareDistUpgrade()
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) prepareDistUpgrade() (*Job, error) {
	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	m.updateJobList()

	if len(m.UpgradableApps) == 0 {
		return nil, system.NotFoundError("empty UpgradableApps")
	}
	if s, err := system.QueryPackageDownloadSize(m.UpgradableApps...); err == nil && s == 0 {
		return nil, system.NotFoundError("no need download")
	}

	job, err := m.jobManager.CreateJob("", system.PrepareDistUpgradeJobType, m.UpgradableApps, nil)
	if err != nil {
		log.Warnf("PrepareDistUpgrade error: %v\n", err)
		return nil, err
	}
	return job, err
}

func (m *Manager) StartJob(jobId string) *dbus.Error {
	m.do.Lock()
	defer m.do.Unlock()

	err := m.jobManager.MarkStart(jobId)
	if err != nil {
		log.Warnf("StartJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) PauseJob(jobId string) *dbus.Error {
	m.do.Lock()
	defer m.do.Unlock()

	err := m.jobManager.PauseJob(jobId)
	if err != nil {
		log.Warnf("PauseJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) CleanJob(jobId string) *dbus.Error {
	m.do.Lock()
	defer m.do.Unlock()

	err := m.jobManager.CleanJob(jobId)
	if err != nil {
		log.Warnf("CleanJob %q error: %v\n", jobId, err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) PackagesDownloadSize(packages []string) (int64, *dbus.Error) {
	m.ensureUpdateSourceOnce()

	m.do.Lock()
	defer m.do.Unlock()

	s, err := system.QueryPackageDownloadSize(packages...)
	if err != nil || s == system.SizeUnknown {
		log.Warnf("PackagesDownloadSize(%q)=%0.2f %v\n", strings.Join(packages, " "), s, err)
	}
	return int64(s), dbusutil.ToError(err)
}

func (m *Manager) PackageInstallable(pkgId string) (bool, *dbus.Error) {
	return system.QueryPackageInstallable(pkgId), nil
}

func (m *Manager) PackageExists(pkgId string) (bool, *dbus.Error) {
	return system.QueryPackageInstalled(pkgId), nil
}

// TODO: Remove this API
func (m *Manager) PackageDesktopPath(pkgId string) (string, *dbus.Error) {
	p, err := utils.RunCommand("/usr/bin/lastore-tools", "querydesktop", pkgId)
	if err != nil {
		log.Warnf("QueryDesktopPath failed: %q\n", err)
		return "", dbusutil.ToError(err)
	}
	return p, nil
}

func (m *Manager) SetRegion(region string) *dbus.Error {
	err := m.config.SetAppstoreRegion(region)
	return dbusutil.ToError(err)
}

func (m *Manager) SetAutoClean(enable bool) *dbus.Error {
	if m.AutoClean == enable {
		return nil
	}

	// save the config to disk
	err := m.config.SetAutoClean(enable)
	if err != nil {
		return dbusutil.ToError(err)
	}

	m.AutoClean = enable
	m.autoCleanCfgChange <- struct{}{}
	m.emitPropChangedAutoClean(enable)
	return nil
}

var errAptRunning = errors.New("apt or apt-get is running")

func (m *Manager) CleanArchives() (dbus.ObjectPath, *dbus.Error) {
	job, err := m.cleanArchives(false)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) cleanArchives(needNotify bool) (*Job, error) {
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

	var jobName string
	if needNotify {
		jobName = "+notify"
	}

	job, err := m.jobManager.CreateJob(jobName, system.CleanJobType, nil, nil)
	if err != nil {
		log.Warnf("CleanArchives error: %v", err)
		return nil, err
	}
	m.config.UpdateLastCleanTime()
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

		_, err := m.cleanArchives(true)
		if err == errAptRunning {
			log.Info("apt is running, waiting for the next chance")
			return
		} else if err != nil {
			log.Warnf("CleanArchives failed: %v", err)
		}
	}

	calcRemainingDuration := func() time.Duration {
		elapsed := time.Since(m.config.LastCleanTime)
		if elapsed < 0 {
			// now time < last clean time : last clean time (from config) is invalid
			return -1
		}
		return m.config.CleanInterval - elapsed
	}

	for {
		select {
		case <-m.autoCleanCfgChange:
			log.Debug("auto clean config changed")
			continue
		case <-time.After(checkInterval):
			if m.AutoClean {
				remaining := calcRemainingDuration()
				log.Debugf("auto clean remaining duration: %v", remaining)
				if remaining < 0 {
					doClean()
				}
			} else {
				log.Debug("auto clean disabled")
			}
		}
	}
}

func (m *Manager) FixError(sender dbus.Sender, errType string) (dbus.ObjectPath, *dbus.Error) {
	job, err := m.fixError(sender, errType)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return job.getPath(), nil
}

func (m *Manager) fixError(sender dbus.Sender, errType string) (*Job, error) {
	m.ensureUpdateSourceOnce()
	environ, err := makeEnvironWithSender(m.service, sender)
	if err != nil {
		return nil, err
	}

	m.do.Lock()
	defer m.do.Unlock()

	switch errType {
	case system.ErrTypeDpkgInterrupted, system.ErrTypeDependenciesBroken:
		// good error type
	default:
		return nil, errors.New("invalid error type")
	}

	job, err := m.jobManager.CreateJob("", system.FixErrorJobType,
		[]string{errType}, environ)
	if err != nil {
		log.Warnf("fixError error: %v", err)
		return nil, err
	}
	return job, err
}
