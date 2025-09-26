// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system/apt"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
	debVersion "pault.ag/go/debian/version"
)

func prepareUpdateSource() {
	partialFilePaths := []string{
		"/var/lib/apt/lists/partial",
		"/var/lib/lastore/lists/partial",
		"/var/cache/apt/archives/partial",
		"/var/cache/lastore/archives/partial",
	}
	for _, partialFilePath := range partialFilePaths {
		infos, err := os.ReadDir(partialFilePath)
		if err != nil {
			continue
		}
		for _, info := range infos {
			err = os.RemoveAll(filepath.Join(partialFilePath, info.Name()))
			if err != nil {
				logger.Warning(err)
			}
		}
	}

}

// updateSource 检查更新主要步骤:1.从更新平台获取数据并解析;2.apt update;3.最终可更新内容确定(模拟安装的方式);4.数据上报;
// 任务进度划分: 0-10%-80%-90%-100%
func (m *Manager) updateSource(sender dbus.Sender) (*Job, error) {
	var err error
	var environ map[string]string
	if !system.IsAuthorized() {
		return nil, errors.New("not authorized, don't allow to exec update")
	}

	if m.ImmutableAutoRecovery {
		logger.Info("immutable auto recovery is enabled, don't allow to exec update")
		return nil, errors.New("immutable auto recovery is enabled, don't allow to exec update")
	}

	defer func() {
		if err == nil {
			err1 := m.config.UpdateLastCheckTime()
			if err1 != nil {
				logger.Warning(err1)
			}
			err1 = m.updateAutoCheckSystemUnit()
			if err1 != nil {
				logger.Warning(err1)
			}
		}
	}()
	environ, err = makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	prepareUpdateSource()
	m.reloadOemConfig(true)
	m.updatePlatform.Token = updateplatform.UpdateTokenConfigFile(m.config.IncludeDiskInfo)
	m.jobManager.dispatch() // 解决 bug 59351问题（防止CreatJob获取到状态为end但是未被删除的job）
	var job *Job
	var isExist bool
	err = system.CustomSourceWrapper(system.AllCheckUpdate, func(path string, unref func()) error {
		m.do.Lock()
		defer m.do.Unlock()
		isExist, job, err = m.jobManager.CreateJob("", system.UpdateSourceJobType, nil, environ, nil)
		if err != nil {
			logger.Warningf("UpdateSource error: %v\n", err)
			if unref != nil {
				unref()
			}
			return err
		}
		if isExist {
			if unref != nil {
				unref()
			}
			logger.Info(JobExistError)
			return JobExistError
		}
		// 设置apt命令参数
		info, err := os.Stat(path)
		if err != nil {
			if unref != nil {
				unref()
			}
			return err
		}
		if info.IsDir() {
			job.option = map[string]string{
				"Dir::Etc::SourceList":  "/dev/null",
				"Dir::Etc::SourceParts": path,
			}
		} else {
			job.option = map[string]string{
				"Dir::Etc::SourceList":  path,
				"Dir::Etc::SourceParts": "/dev/null",
			}
		}
		job.subRetryHookFn = func(j *Job) {
			handleUpdateSourceFailed(j)
		}
		job.setPreHooks(map[string]func() error{
			string(system.RunningStatus): func() error {
				// 检查更新需要重置备份状态,主要是处理备份失败后再检查更新,会直接显示失败的场景
				m.statusManager.SetABStatus(system.AllCheckUpdate, system.NotBackup, system.NoABError)
				return nil
			},
			string(system.SucceedStatus): func() error {
				m.refreshUpdateInfos(true)
				m.PropsMu.Lock()
				m.updateSourceOnce = true
				m.PropsMu.Unlock()
				if len(m.UpgradableApps) > 0 {
					go m.reportLog(updateStatusReport, true, "")
					// 开启自动下载时触发自动下载,发自动下载通知,不发送可更新通知;
					// 关闭自动下载时,发可更新的通知;
					if !m.updater.AutoDownloadUpdates {
						// msg := gettext.Tr("New system edition available")
						msg := gettext.Tr("New version available!")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,update")}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
				} else {
					go m.reportLog(updateStatusReport, false, "")
				}
				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:   "info",
						Detail: "update source success",
					})
				}()
				m.updatePlatform.SaveCache(m.config)
				job.setPropProgress(1.0)
				return nil
			},
			string(system.FailedStatus): func() error {
				// 网络问题检查更新失败和空间不足下载索引失败,需要发通知
				var errorContent system.JobError
				err = json.Unmarshal([]byte(job.Description), &errorContent)
				if err == nil {
					if strings.Contains(errorContent.ErrType.String(), system.ErrorFetchFailed.String()) || strings.Contains(errorContent.ErrType.String(), system.ErrorIndexDownloadFailed.String()) {
						msg := gettext.Tr("Failed to check for updates. Please check your network.")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,network")}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
					if strings.Contains(errorContent.ErrType.String(), system.ErrorInsufficientSpace.String()) {
						msg := gettext.Tr("Failed to check for updates. Please clean up your disk first.")
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
					}
				}
				// 发通知 end
				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.reportLog(updateStatusReport, false, job.Description)

					m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
						Type:           "error",
						JobDescription: job.Description,
						Detail:         fmt.Sprintf("apt-get update failed, detail is %v , option is %+v", job.Description, job.option),
					})
				}()
				return nil
			},
			string(system.EndStatus): func() error {
				// wrapper的资源释放
				if unref != nil {
					unref()
				}
				return nil
			},
		})
		job.setAfterHooks(map[string]func() error{
			string(system.RunningStatus): func() error {
				job.setPropProgress(0.01)
				_ = os.Setenv("http_proxy", environ["http_proxy"])
				_ = os.Setenv("https_proxy", environ["https_proxy"])
				// 检查任务开始后,从更新平台获取仓库、更新注记等信息
				// 从更新平台获取数据:系统更新和安全更新流程都包含
				err = m.updatePlatform.GenUpdatePolicyByToken(true)
				if err != nil {
					if m.config.PlatformUpdate {
						job.retry = 0
						return &system.JobError{
							ErrType:   system.ErrorPlatformUnreachable,
							ErrDetail: "failed to get update policy by token" + err.Error(),
						}
					} else {
						logger.Warning("updatePlatform gen token failed", err)
						return nil
					}
				}

				err = m.updatePlatform.UpdateAllPlatformDataSync()
				if err != nil {
					logger.Warning(err)
					if m.config.PlatformUpdate {
						job.retry = 0
						return &system.JobError{
							ErrType:   system.ErrorPlatformUnreachable,
							ErrDetail: "failed to get update info by update platform" + err.Error(),
						}
					} else {
						return nil
					}
				}
				m.updater.setPropUpdateTarget(m.updatePlatform.GetUpdateTarget()) // 更新目标 历史版本控制中心获取UpdateTarget,获取更新日志

				// 从更新平台获取数据并处理完成后,进度更新到10%
				job.setPropProgress(0.10)
				return nil
			},
		})

		if err = m.jobManager.addJob(job); err != nil {
			logger.Warning(err)
			if unref != nil {
				unref()
			}
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, JobExistError) { // exist的err无需返回
		logger.Warning(err)
		return nil, err
	}
	return job, nil
}

// 暂时废弃获取可更新列表的详细信息
var getUpgradablePackageListMap = map[system.UpdateType]func([]string) (map[string]system.PackageInfo, map[string]system.PackageInfo, error){
	system.SystemUpdate:   getSystemUpgradablePackagesMap,
	system.SecurityUpdate: getSecurityUpgradablePackagesMap,
	system.UnknownUpdate:  getUnknownUpgradablePackagesMap,
}

// 生成系统更新内容和安全更新内容
func (m *Manager) generateUpdateInfo() (errList []error) {
	propPkgMap := make(map[string][]string) // updater的ClassifiedUpdatablePackages用
	var propPkgMapMu sync.Mutex
	var errListMu sync.Mutex
	appendErrorSafe := func(err error) {
		errListMu.Lock()
		errList = append(errList, err)
		errListMu.Unlock()
	}
	updatePropPkgMapSafe := func(t string, packageList []string) {
		propPkgMapMu.Lock()
		propPkgMap[t] = packageList
		propPkgMapMu.Unlock()
	}

	var wg sync.WaitGroup
	for updateType, getFn := range getUpgradablePackageList {
		wg.Add(1)
		fn := getFn
		t := updateType
		go func() {
			logger.Infof("start get %v upgradable package", t.JobType())
			installList, err := fn(m.coreList)
			if err != nil {
				appendErrorSafe(err)
			} else {
				updatePropPkgMapSafe(t.JobType(), installList)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	m.updater.setClassifiedUpdatablePackages(propPkgMap)
	return
}

func getSystemUpgradablePackagesMap(coreList []string) (map[string]system.PackageInfo, map[string]system.PackageInfo, error) {
	if len(coreList) == 0 {
		return nil, nil, errors.New("coreList is nil,can not get system update package list")
	}
	var err error

	var emulateInstallPkgList map[string]system.PackageInfo
	var emulateRemovePkgList map[string]system.PackageInfo

	// 模拟安装更新平台下发所有包(不携带版本号)，获取可升级包的版本
	systemSource := system.GetCategorySourceMap()[system.SystemUpdate]
	var options []string
	if info, err := os.Stat(systemSource); err == nil {
		if info.IsDir() {
			options = []string{
				"-o", "Dir::Etc::sourcelist=/dev/null",
				"-o", fmt.Sprintf("Dir::Etc::SourceParts=%v", systemSource),
			}
		} else {
			options = []string{
				"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", systemSource),
				"-o", "Dir::Etc::SourceParts=/dev/null",
				"-o", "Dir::Etc::preferences=/dev/null", // 系统更新仓库来自更新平台，为了不收本地优先级配置影响，覆盖本地优先级配置
				"-o", "Dir::Etc::PreferencesParts=/dev/null",
			}
		}
	}

	emulateInstallPkgList, emulateRemovePkgList, err = apt.GenOnlineUpdatePackagesByEmulateInstall(coreList, options)
	if err != nil {
		return nil, nil, err
	}
	return emulateInstallPkgList, emulateRemovePkgList, nil
}

func getSecurityUpgradablePackagesMap(coreList []string) (map[string]system.PackageInfo, map[string]system.PackageInfo, error) {
	return apt.GenOnlineUpdatePackagesByEmulateInstall(nil, []string{
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", system.GetCategorySourceMap()[system.SecurityUpdate]),
		"-o", "Dir::Etc::SourceParts=/dev/null",
	})
}

func getUnknownUpgradablePackagesMap(coreList []string) (map[string]system.PackageInfo, map[string]system.PackageInfo, error) {
	return apt.GenOnlineUpdatePackagesByEmulateInstall(nil, []string{
		"-o", fmt.Sprintf("Dir::Etc::SourceParts=%v", system.GetCategorySourceMap()[system.UnknownUpdate]),
		"-o", "Dir::Etc::sourcelist=/dev/null",
	})
}

var getUpgradablePackageList = map[system.UpdateType]func([]string) ([]string, error){
	system.SystemUpdate:   getSystemUpgradablePackageList,
	system.SecurityUpdate: getSecurityUpgradablePackageList,
	system.UnknownUpdate:  getUnknownUpgradablePackageList,
}

func getSystemUpgradablePackageList(coreList []string) ([]string, error) {
	return apt.ListDistUpgradePackages(system.GetCategorySourceMap()[system.SystemUpdate], coreList)
}

func getSecurityUpgradablePackageList(coreList []string) ([]string, error) {
	return apt.ListDistUpgradePackages(system.GetCategorySourceMap()[system.SecurityUpdate], coreList)
}

func getUnknownUpgradablePackageList(coreList []string) ([]string, error) {
	return apt.ListDistUpgradePackages(system.GetCategorySourceMap()[system.UnknownUpdate], coreList)
}

// 判断包对应版本是否存在
func checkDebExistWithVersion(pkgList []string) bool {
	args := strings.Join(pkgList, " ")
	args = "apt show " + args
	cmd := exec.Command("/bin/sh", "-c", args)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		logger.Warning(outBuf.String())
		logger.Warning(errBuf.String())
	}
	return err == nil
}

type statusVersion struct {
	status  string
	version string
}

func loadPkgStatusVersion() (map[string]statusVersion, error) {
	out, err := exec.Command("dpkg-query", "-f", "${Package} ${db:Status-Abbrev} ${Version}\n", "-W").Output() // #nosec G204
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(out)
	scanner := bufio.NewScanner(reader)
	result := make(map[string]statusVersion)
	for scanner.Scan() {
		line := scanner.Bytes()
		fields := bytes.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pkg := string(fields[0])
		status := string(fields[1])
		version := string(fields[2])
		result[pkg] = statusVersion{
			status:  status,
			version: version,
		}
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ver1 >= ver2
func compareVersionsGe(ver1, ver2 string) bool {
	gt, err := compareVersionsGeFast(ver1, ver2)
	if err != nil {
		return compareVersionsGeDpkg(ver1, ver2)
	}
	return gt
}

// ver1 < ver2
func compareVersionLt(ver1, ver2 string) bool {
	return compareVersionsLtDpkg(ver1, ver2)
}

func compareVersionsGeFast(ver1, ver2 string) (bool, error) {
	v1, err := debVersion.Parse(ver1)
	if err != nil {
		return false, err
	}
	v2, err := debVersion.Parse(ver2)
	if err != nil {
		return false, err
	}
	return debVersion.Compare(v1, v2) >= 0, nil
}

func compareVersionsGeDpkg(ver1, ver2 string) bool {
	err := exec.Command("dpkg", "--compare-versions", "--", ver1, "ge", ver2).Run() // #nosec G204
	return err == nil
}

func compareVersionsLtDpkg(ver1, ver2 string) bool {
	err := exec.Command("dpkg", "--compare-versions", "--", ver1, "lt", ver2).Run() // #nosec G204
	return err == nil
}

func listDistUpgradePackages(updateType system.UpdateType) ([]string, error) {
	sourcePath := system.GetCategorySourceMap()[updateType]
	return apt.ListDistUpgradePackages(sourcePath, nil)
}

func (m *Manager) getCoreList(online bool) []string {
	if !m.config.EnableCoreList {
		return nil
	}
	if online {
		return getCoreListOnline()
	}
	return getCoreListFromCache()
}

const TimeOnly = "15:04:05"

func (m *Manager) refreshUpdateInfos(sync bool) {
	// 检查更新时,同步修改canUpgrade状态;检查更新时需要同步操作
	if sync {
		// 检查更新后，先下载解析coreList，获取必装清单
		m.coreList = m.getCoreList(true)
		logger.Debug("generateUpdateInfo get coreList:", m.coreList)
		for _, e := range m.generateUpdateInfo() {
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
					Type:   "error",
					Detail: fmt.Sprintf("generate package list error, detail is %v:", e),
				})
			}()
			logger.Warning(e)
		}
		m.statusManager.updateSourceOnce = true
		m.statusManager.UpdateModeAllStatusBySize(m.coreList)
		m.statusManager.UpdateCheckCanUpgradeByEachStatus()
	} else {
		m.coreList = m.getCoreList(false)
		go func() {
			// 刷新一下包信息
			if isFirstBoot() {
				for _, e := range m.generateUpdateInfo() {
					go func() {
						m.inhibitAutoQuitCountAdd()
						defer m.inhibitAutoQuitCountSub()

						m.updatePlatform.PostStatusMessage(updateplatform.StatusMessage{
							Type:   "error",
							Detail: fmt.Sprintf("generate package list error, detail is %v:", e),
						})

					}()
					logger.Warning(e)
				}
			}
			m.statusManager.UpdateModeAllStatusBySize(m.coreList)
			m.statusManager.UpdateCheckCanUpgradeByEachStatus()
		}()
	}
	m.updateUpdatableProp(m.updater.ClassifiedUpdatablePackages)
	m.statusManager.SetFrontForceUpdate(m.updatePlatform.Tp == updateplatform.UpdateShutdown)
	if updateplatform.IsForceUpdate(m.updatePlatform.Tp) {
		go func() {
			m.inhibitAutoQuitCountAdd()
			logger.Info("auto download force updates")
			_, err := m.prepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]), system.SystemUpdate, initiatorAuto) // TODO system.SystemUpdate
			if err != nil {
				logger.Error("failed to prepare dist-upgrade:", err)
			}
			m.inhibitAutoQuitCountSub()
		}()
		if !m.updatePlatform.UpdateNowForce && !m.updatePlatform.UpdateTime.IsZero() {
			timeStr := m.updatePlatform.UpdateTime.Format(TimeOnly)
			if timeStr != m.updateTime {
				m.updateTime = timeStr
				msg := fmt.Sprintf(gettext.Tr("The computer will be updated at %s"), m.updateTime)
				go m.sendNotify(updateNotifyShow, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
			}
		}
		if m.updatePlatform.Tp == updateplatform.UpdateRegularly {
			_ = m.updateTimerUnit(lastoreRegularlyUpdate)
		}
		// 强制更新开启后，以强制更新下载策略优先
		return
	}
	if m.updater.AutoDownloadUpdates && len(m.updater.UpdatablePackages) > 0 && sync && !m.updater.getIdleDownloadEnabled() {
		logger.Info("auto download updates")
		go func() {
			m.inhibitAutoQuitCountAdd()
			_, err := m.prepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]), m.CheckUpdateMode, initiatorAuto)
			if err != nil {
				logger.Error("failed to prepare dist-upgrade:", err)
			}
			m.inhibitAutoQuitCountSub()
		}()
	}
}

func (m *Manager) updateUpdatableProp(infosMap map[string][]string) {
	m.PropsMu.RLock()
	updateType := m.UpdateMode
	m.PropsMu.RUnlock()
	filterInfos := getFilterPackages(infosMap, updateType)
	m.updatableApps(filterInfos) // Manager的UpgradableApps实际为可更新的包,而非应用;
	m.updater.setUpdatablePackages(filterInfos)
	m.updater.updateUpdatableApps()
}

func (m *Manager) ensureUpdateSourceOnce() {
	m.PropsMu.Lock()
	updateOnce := m.updateSourceOnce
	m.PropsMu.Unlock()

	if updateOnce {
		return
	}

	_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]))
	if err != nil {
		logger.Warning(err)
		return
	}
}

// retry次数为1
// 默认检查为 AllCheckUpdate
// 重试检查为 AllInstallUpdate
func handleUpdateSourceFailed(j *Job) {
	retry := j.retry
	if retry > 1 {
		retry = 1
		j.retry = retry
	}
	retryMap := map[int]system.UpdateType{
		1: system.SystemUpdate | system.SecurityUpdate | system.AppendUpdate,
	}
	updateType := retryMap[retry]
	err := system.CustomSourceWrapper(updateType, func(path string, unref func()) error {
		// 重新设置apt命令参数
		info, err := os.Stat(path)
		if err != nil {
			if unref != nil {
				unref()
			}
			return err
		}
		if info.IsDir() {
			j.option = map[string]string{
				"Dir::Etc::SourceList":  "/dev/null",
				"Dir::Etc::SourceParts": path,
			}
		} else {
			j.option = map[string]string{
				"Dir::Etc::SourceList":  path,
				"Dir::Etc::SourceParts": "/dev/null",
			}
		}
		j.wrapPreHooks(map[string]func() error{
			string(system.EndStatus): func() error {
				if unref != nil {
					unref()
				}
				return nil
			},
		})
		return nil
	})
	if err != nil {
		logger.Warning(err)
	}
}
