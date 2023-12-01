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
	"github.com/linuxdeepin/go-lib/strv"
	utils2 "github.com/linuxdeepin/go-lib/utils"
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
				m.PropsMu.Lock()
				m.updateSourceOnce = true
				m.PropsMu.Unlock()
				if len(m.UpgradableApps) > 0 {
					go m.updatePlatform.reportLog(updateStatusReport, true, "")
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
					go m.updatePlatform.reportLog(updateStatusReport, false, "")
				}
				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.updatePlatform.postStatusMessage("check update success")
				}()
				m.savePlatformCache()
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
				go func() {
					m.inhibitAutoQuitCountAdd()
					defer m.inhibitAutoQuitCountSub()
					m.updatePlatform.reportLog(updateStatusReport, false, job.Description)
					m.updatePlatform.postStatusMessage(fmt.Sprintf("check update failed, detail is %v ", job.Description))
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
				err = m.updatePlatform.genUpdatePolicyByToken()
				if err != nil {
					job.retry = 0
					return &system.JobError{
						Type:   system.ErrorPlatformUnreachable,
						Detail: "failed to get update policy by token" + err.Error(),
					}
				}
				err = m.updatePlatform.UpdateAllPlatformDataSync()
				if err != nil {
					logger.Warning(err)
					job.retry = 0
					return &system.JobError{
						Type:   system.ErrorPlatformUnreachable,
						Detail: "failed to get update info by update platform" + err.Error(),
					}
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

const (
	coreListPath    = "/usr/share/core-list/corelist"
	coreListVarPath = "/var/lib/lastore/corelist"
	coreListPkgName = "deepin-package-list"
)

// 下载并解压coreList
func downloadAndDecompressCoreList() (string, error) {
	downloadPackages := []string{coreListPkgName}
	options := map[string]string{
		"Dir::Etc::SourceList":  system.GetCategorySourceMap()[system.SystemUpdate],
		"Dir::Etc::SourceParts": "/dev/null",
	}
	downloadPkg, err := apt.DownloadPackages(downloadPackages, nil, options)
	if err != nil {
		// 下载失败则直接去本地目录查找
		logger.Warningf("download %v failed:%v", downloadPackages, err)
		return coreListPath, nil
	}
	// 去下载路径查找
	files, err := ioutil.ReadDir(downloadPkg)
	if err != nil {
		return "", err
	}
	var debFile string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), coreListPkgName) && strings.HasSuffix(file.Name(), ".deb") {
			debFile = filepath.Join(downloadPkg, file.Name())
			break
		}
	}
	if debFile != "" {
		tmpDir, err := ioutil.TempDir("/tmp", coreListPkgName+".XXXXXX")
		if err != nil {
			return "", err
		}
		cmd := exec.Command("dpkg-deb", "-x", debFile, tmpDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if err != nil {
			return "", err
		}
		return filepath.Join(tmpDir, coreListPath), nil
	} else {
		return "", fmt.Errorf("coreList deb not found")
	}
}

type Package struct {
	PkgName string `json:"PkgName"`
	Version string `json:"Version"`
}

type PackageList struct {
	PkgList []Package `json:"PkgList"`
	Version string    `json:"Version"`
}

func parseCoreList() ([]string, error) {
	// 1. download coreList to /var/cache/lastore/archives/
	// 2. 使用dpkg-deb解压deb得到coreList文件
	corefile, err := downloadAndDecompressCoreList()
	if err != nil {
		return nil, err
	}
	// 将coreList 备份到/var/lib/lastore/中
	if err != utils2.CopyFile(corefile, coreListVarPath) {
		logger.Warning("backup coreList failed:", err)
	}
	// 3. 解析文件获取coreList必装列表
	data, err := ioutil.ReadFile(corefile)
	if err != nil {
		return nil, err
	}
	var pkgList PackageList
	err = json.Unmarshal(data, &pkgList)
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, pkg := range pkgList.PkgList {
		pkgs = append(pkgs, pkg.PkgName)
	}
	return pkgs, nil
}

// 生成系统更新内容和安全更新内容
func (m *Manager) generateUpdateInfo() (error, error) {
	propPkgMap := make(map[string][]string) // updater的ClassifiedUpdatablePackages用
	var systemErr error = nil
	var securityErr error = nil
	var systemInstallPkgList map[string]system.PackageInfo
	var systemRemovePkgList map[string]system.PackageInfo
	var securityInstallPkgList map[string]system.PackageInfo
	var securityRemovePkgList map[string]system.PackageInfo
	m.allUpgradableInfo = make(map[system.UpdateType]map[string]system.PackageInfo)
	m.allRemovePkgInfo = make(map[system.UpdateType]map[string]system.PackageInfo)

	var wg sync.WaitGroup
	// 检查更新前，先下载解析coreList，获取必装清单
	coreList, err := parseCoreList()

	if err != nil {
		systemErr = err
	} else {
		if !strv.Strv(coreList).Contains(coreListPkgName) {
			coreList = append(coreList, coreListPkgName)
		}
		m.coreList = coreList
		logger.Debug("generateUpdateInfo get coreList:", coreList)
		wg.Add(1)
		go func() {
			systemInstallPkgList, systemRemovePkgList, systemErr = getSystemUpdatePackageList(coreList)
			wg.Done()
		}()
	}
	wg.Add(1)
	go func() {
		securityInstallPkgList, securityRemovePkgList, securityErr = getSecurityUpdatePackageList()
		wg.Done()
	}()
	wg.Wait()
	if systemErr == nil && systemInstallPkgList != nil {
		// 如果卸载列表中有coreList，则系统更新列表置空，上报日志
		var removeCoreList []string
		for _, pkgName := range coreList {
			if _, ok := systemRemovePkgList[pkgName]; ok {
				removeCoreList = append(removeCoreList, pkgName)
			}
		}
		if len(removeCoreList) > 0 {
			// 上报日志
			m.updatePlatform.postStatusMessage(fmt.Sprintf("there was coreList remove, detail is %v:", removeCoreList))
			// 清空系统可升级包列表
			systemInstallPkgList = nil
			systemRemovePkgList = nil
		}
		var packageList []string
		for k, v := range systemInstallPkgList {
			packageList = append(packageList, fmt.Sprintf("%v=%v", k, v.Version))
		}
		propPkgMap[system.SystemUpdate.JobType()] = packageList
		m.allUpgradableInfo[system.SystemUpdate] = systemInstallPkgList
	}
	if securityErr == nil && securityInstallPkgList != nil {
		var packageList []string
		for k, v := range securityInstallPkgList {
			packageList = append(packageList, fmt.Sprintf("%v=%v", k, v.Version))
		}
		propPkgMap[system.SecurityUpdate.JobType()] = packageList
		m.allUpgradableInfo[system.SecurityUpdate] = securityInstallPkgList
	}

	if systemErr == nil && systemRemovePkgList != nil {
		m.allRemovePkgInfo[system.SystemUpdate] = systemRemovePkgList
	}
	if securityErr == nil && securityRemovePkgList != nil {
		m.allRemovePkgInfo[system.SecurityUpdate] = securityRemovePkgList
	}
	m.updater.setClassifiedUpdatablePackages(propPkgMap)
	return systemErr, securityErr
}

func getSystemUpdatePackageList(coreList []string) (map[string]system.PackageInfo, map[string]system.PackageInfo, error) {
	var err error
	// var localCache map[string]statusVersion
	var emulateInstallPkgList map[string]system.PackageInfo
	var emulateRemovePkgList map[string]system.PackageInfo
	// 获取本地deb信息
	// localCache, err = loadPkgStatusVersion()
	// if err != nil {
	// 	logger.Warning(err)
	// 	return nil, nil, err
	// }

	// 模拟安装更新平台下发所有包(不携带版本号)，获取可升级包的版本
	emulateInstallPkgList, emulateRemovePkgList, err = apt.GenOnlineUpdatePackagesByEmulateInstall(coreList, []string{
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", system.GetCategorySourceMap()[system.SystemUpdate]),
		"-o", "Dir::Etc::SourceParts=/dev/null",
		"-o", "Dir::Etc::preferences=/dev/null", // 系统更新仓库来自更新平台，为了不收本地优先级配置影响，覆盖本地优先级配置
		"-o", "Dir::Etc::PreferencesParts=/dev/null",
	})
	if err != nil {
		return nil, nil, err
	}
	// for _, platformPkgInfo := range platFormPackageMap {
	// 	repoPkgInfo, ok := emulateInstallPkgList[platformPkgInfo.Name]
	// 	if ok {
	// 		// 该包可升级，但是可升级版本小于更新平台下发版本，此时将不允许升级
	// 		if !compareVersionsGe(repoPkgInfo.Version, platformPkgInfo.Version) {
	// 			return nil, nil, fmt.Errorf("%v can not install to version %v", platformPkgInfo.Name, platformPkgInfo.Version)
	// 		}
	// 	} else {
	// 		// 该包不能升级，需要判断是否在本地存在高版本包
	// 		localPkgInfo, ok := localCache[platformPkgInfo.Name]
	// 		if ok {
	// 			// 本地有该包，但是版本小于更新平台版本
	// 			if !compareVersionsGe(localPkgInfo.version, repoPkgInfo.Version) {
	// 				return nil, nil, fmt.Errorf("local exist low version package and %v can not install to version：%v in repo", repoPkgInfo.Name, repoPkgInfo.Version)
	// 			}
	// 		} else {
	// 			// 本地无该包
	// 			return nil, nil, fmt.Errorf("local and repo not exist %v", platformPkgInfo.Name)
	// 		}
	// 	}
	// }
	return emulateInstallPkgList, emulateRemovePkgList, nil
}

func getSecurityUpdatePackageList() (map[string]system.PackageInfo, map[string]system.PackageInfo, error) {
	return apt.GenOnlineUpdatePackagesByEmulateInstall(nil, []string{
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

func (m *Manager) refreshUpdateInfos(sync bool) {
	// 检查更新时,同步修改canUpgrade状态;检查更新时需要同步操作
	if sync {
		systemErr, securityErr := m.generateUpdateInfo()
		if systemErr != nil {
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.postStatusMessage(fmt.Sprintf("generate system package list error, detail is %v:", systemErr))
			}()
			logger.Warning(systemErr)
		}
		if securityErr != nil {
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.postStatusMessage(fmt.Sprintf("generate security package list error, detail is %v:", securityErr))
			}()
			logger.Warning(securityErr)
		}
		m.statusManager.UpdateModeAllStatusBySize(m.updater.ClassifiedUpdatablePackages)
		m.statusManager.UpdateCheckCanUpgradeByEachStatus()
	} else {
		go func() {
			m.statusManager.UpdateModeAllStatusBySize(m.updater.ClassifiedUpdatablePackages)
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
