package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"internal/system"
	"internal/system/apt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/gettext"
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
		infos, err := ioutil.ReadDir(partialFilePath)
		if err != nil {
			logger.Warning(err)
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
func (m *Manager) updateSource(sender dbus.Sender, needNotify bool) (*Job, error) {
	var err error
	var environ map[string]string
	if !system.IsAuthorized() || !system.IsActiveCodeExist() {
		return nil, errors.New("not authorized, don't allow to exec update")
	}
	defer func() {
		if err == nil {
			err1 := m.config.UpdateLastCheckTime()
			if err1 != nil {
				logger.Warning(err1)
			}
			err1 = m.updateAutoCheckSystemUnit()
			if err != nil {
				logger.Warning(err)
			}
		}
	}()
	environ, err = makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	prepareUpdateSource()
	m.updatePlatform.token = updateTokenConfigFile()
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
				job.setPropProgress(0.90)
				m.PropsMu.Lock()
				m.updateSourceOnce = true
				m.PropsMu.Unlock()
				if len(m.UpgradableApps) > 0 {
					m.updatePlatform.reportLog(updateStatusReport, true, "")
					m.updatePlatform.PostStatusMessage("")
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
					m.updatePlatform.reportLog(updateStatusReport, false, "")
					m.updatePlatform.PostStatusMessage("")
				}
				job.setPropProgress(1.0)
				return nil
			},
			string(system.FailedStatus): func() error {
				// 网络问题检查更新失败和空间不足下载索引失败,需要发通知
				var errorContent = struct {
					ErrType   string
					ErrDetail string
				}{}
				err = json.Unmarshal([]byte(job.Description), &errorContent)
				if err == nil {
					if strings.Contains(errorContent.ErrType, string(system.ErrorFetchFailed)) || strings.Contains(errorContent.ErrType, string(system.ErrorIndexDownloadFailed)) {
						msg := gettext.Tr("Failed to check for updates. Please check your network.")
						action := []string{"view", gettext.Tr("View")}
						hints := map[string]dbus.Variant{"x-deepin-action-view": dbus.MakeVariant("dde-control-center,-m,network")}
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, action, hints, system.NotifyExpireTimeoutDefault)
					}
					if strings.Contains(errorContent.ErrType, string(system.ErrorInsufficientSpace)) {
						msg := gettext.Tr("Failed to check for updates. Please clean up your disk first.")
						go m.sendNotify(updateNotifyShowOptional, 0, "preferences-system", "", msg, nil, nil, system.NotifyExpireTimeoutDefault)
					}
				}
				// 发通知 end
				m.updatePlatform.reportLog(updateStatusReport, false, job.Description)
				m.updatePlatform.PostStatusMessage("")
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
				err = m.updatePlatform.genUpdatePolicyByToken()
				if err != nil {
					job.retry = 0
					m.updatePlatform.PostStatusMessage(err.Error())
					return errors.New("failed to get update policy by token")
				}
				err = m.updatePlatform.UpdateAllPlatformDataSync()
				if err != nil {
					logger.Warning(err)
					job.retry = 0
					m.updatePlatform.PostStatusMessage(err.Error())
					return fmt.Errorf("failed to get update info by update platform: %v", err)
				}
				m.updater.setPropUpdateTarget(m.updatePlatform.getUpdateTarget()) // 更新目标 历史版本控制中心获取UpdateTarget,获取更新日志

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

// 生成系统更新内容和安全更新内容
func (m *Manager) generateUpdateInfo(platFormPackageList map[string]system.PackageInfo) (error, error) {
	propPkgMap := make(map[string][]string) // updater的ClassifiedUpdatablePackages用
	var systemErr error = nil
	var securityErr error = nil
	var systemPackageList map[string]system.PackageInfo
	var securityPackageList map[string]system.PackageInfo
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		systemPackageList, systemErr = getSystemUpdatePackageList(platFormPackageList)
		if systemErr == nil && systemPackageList != nil {
			var packageList []string
			for k, v := range systemPackageList {
				packageList = append(packageList, fmt.Sprintf("%v=%v", k, v.Version))
			}
			propPkgMap[system.SystemUpdate.JobType()] = packageList
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		securityPackageList, securityErr = getSecurityUpdatePackageList()
		if securityErr == nil && securityPackageList != nil {
			var packageList []string
			for k, v := range securityPackageList {
				packageList = append(packageList, fmt.Sprintf("%v=%v", k, v.Version))
			}
			propPkgMap[system.SecurityUpdate.JobType()] = packageList
		}
		wg.Done()
	}()
	wg.Wait()
	m.updater.setClassifiedUpdatablePackages(propPkgMap)
	if systemErr == nil && systemPackageList != nil {
		m.allUpgradableInfo[system.SystemUpdate] = systemPackageList
	}
	if securityErr == nil && securityPackageList != nil {
		m.allUpgradableInfo[system.SecurityUpdate] = securityPackageList
	}
	return systemErr, securityErr
}

func getSystemUpdatePackageList(platFormPackageMap map[string]system.PackageInfo) (map[string]system.PackageInfo, error) {
	var err error
	var localCache map[string]statusVersion
	var repoUpgradableList []string
	var emulatePkgList map[string]system.PackageInfo
	// 获取本地deb信息
	localCache, err = loadPkgStatusVersion()
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	for name, _ := range platFormPackageMap {
		repoUpgradableList = append(repoUpgradableList, name)
	}
	// 模拟安装更新平台下发所有包(不携带版本号)，获取可升级包的版本
	emulatePkgList, err = apt.GenOnlineUpdatePackagesByEmulateInstall(repoUpgradableList, []string{
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", system.GetCategorySourceMap()[system.SystemUpdate]),
		"-o", "Dir::Etc::SourceParts=/dev/null",
		"-o", "Dir::Etc::preferences=/dev/null", // 系统更新仓库来自更新平台，为了不收本地优先级配置影响，覆盖本地优先级配置
		"-o", "Dir::Etc::PreferencesParts=/dev/null",
	})
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	for _, platformPkgInfo := range platFormPackageMap {
		repoPkgInfo, ok := emulatePkgList[platformPkgInfo.Name]
		if ok {
			// 该包可升级，但是可升级版本小于更新平台下发版本，此时将不允许升级
			if !compareVersionsGe(platformPkgInfo.Version, repoPkgInfo.Version) {
				return nil, fmt.Errorf("%v can not install to version %v", platformPkgInfo.Name, platformPkgInfo.Version)
			}
		} else {
			// 该包不能升级，需要判断是否在本地存在高版本包
			localPkgInfo, ok := localCache[platformPkgInfo.Name]
			if ok {
				// 本地有该包，但是版本小于更新平台版本
				if !compareVersionsGe(localPkgInfo.version, repoPkgInfo.Version) {
					return nil, fmt.Errorf("local exist low version package and %v can not install to version：%v in repo", repoPkgInfo.Name, repoPkgInfo.Version)
				}
			} else {
				// 本地无该包
				return nil, fmt.Errorf("local and repo not exist %v", platformPkgInfo.Name)
			}
		}
	}
	return emulatePkgList, nil
}

func getSecurityUpdatePackageList() (map[string]system.PackageInfo, error) {
	pkgList, err := listDistUpgradePackages(system.SecurityUpdate) // 仓库检查出所有可以升级的包
	if err != nil {
		if os.IsNotExist(err) { // 该类型源文件不存在时
			logger.Info(err)
			return nil, nil // 该错误无需返回
		} else {
			logger.Warningf("failed to list %v upgradable package %v", system.SystemUpdate.JobType(), err)
			return nil, err
		}
	}
	return apt.GenOnlineUpdatePackagesByEmulateInstall(pkgList, []string{
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", system.GetCategorySourceMap()[system.SecurityUpdate]),
		"-o", "Dir::Etc::SourceParts=/dev/null",
	})
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

func compareVersionsGe(ver1, ver2 string) bool {
	gt, err := compareVersionsGeFast(ver1, ver2)
	if err != nil {
		return compareVersionsGeDpkg(ver1, ver2)
	}
	return gt
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

func listDistUpgradePackages(updateType system.UpdateType) ([]string, error) {
	sourcePath := system.GetCategorySourceMap()[updateType]
	return apt.ListDistUpgradePackages(sourcePath, nil)
}

func (m *Manager) refreshUpdateInfos(sync bool) {
	// 检查更新时,同步修改canUpgrade状态;检查更新时需要同步操作
	if sync {
		systemErr, securityErr := m.generateUpdateInfo(m.updatePlatform.GetSystemMeta())
		if systemErr != nil {
			m.updatePlatform.PostStatusMessage("")
			logger.Warning(systemErr)
		}
		if securityErr != nil {
			m.updatePlatform.PostStatusMessage("")
			logger.Warning(securityErr)
		}
		m.statusManager.UpdateModeAllStatusBySize()
		m.statusManager.UpdateCheckCanUpgradeByEachStatus()
	} else {
		go func() {
			m.statusManager.UpdateModeAllStatusBySize()
			m.statusManager.UpdateCheckCanUpgradeByEachStatus()
		}()
	}
	m.updateUpdatableProp(m.updater.ClassifiedUpdatablePackages)
	if m.updater.AutoDownloadUpdates && len(m.updater.UpdatablePackages) > 0 && sync && !m.updater.getIdleDownloadEnabled() {
		logger.Info("auto download updates")
		go func() {
			m.inhibitAutoQuitCountAdd()
			_, err := m.PrepareDistUpgrade(dbus.Sender(m.service.Conn().Names()[0]))
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

	_, err := m.updateSource(dbus.Sender(m.service.Conn().Names()[0]), false)
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
		1: system.AllInstallUpdate,
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
