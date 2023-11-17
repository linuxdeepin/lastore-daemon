package main

import (
	"bytes"
	"errors"
	"fmt"
	"internal/system"
	"internal/system/apt"
	"internal/system/dut"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/godbus/dbus"
)

const (
	verifyBin   = "/usr/bin/deepin-iso-verify"
	unzipBin    = "/usr/bin/ar"
	unzipOupDir = "/var/lib/lastore/unzipcache"
	mountFsDir  = "/var/lib/lastore/mountfs"
)

type OfflineUpgradeType uint

const (
	unknown OfflineUpgradeType = iota // 同时存在两种类型时
	offlineSystem
	offlineCEV
)

type Indicator func(progress float64)
type OfflineManager struct {
	localUnzipOupDirs []string // oup文件解压后的路径集合
	localOupRepoPaths []string // repo.sfs挂载后的路径
	localOupInfoMap   map[string]OfflineRepoInfo
	// localOupCheckMap  map[string]*OupResultInfo
	checkResult         OfflineCheckResult
	upgradeAblePackages map[string]system.PackageInfo // 离线更新可更新包
	removePackages      map[string]system.PackageInfo
}

func NewOfflineManager() *OfflineManager {
	return &OfflineManager{
		localUnzipOupDirs: nil,
		localOupRepoPaths: nil,
		localOupInfoMap:   make(map[string]OfflineRepoInfo),
		// localOupCheckMap:  make(map[string]*OupResultInfo),
	}
}

type CheckState uint

const (
	nocheck CheckState = iota
	success
	failed
)

type OfflineRepoInfo struct {
	Type    OfflineUpgradeType `json:"type"`
	Version string             `json:"version"`
	Data    struct {
		Archs          string `json:"archs"`
		Binary         string `json:"binary"`
		CveDescription string `json:"cveDescription"`
		CveId          string `json:"cveId"`
		Description    string `json:"description"`
		FixedVersion   string `json:"fixedVersion"`
		PubTime        string `json:"pubTime"`
		Score          string `json:"score"`
		Source         string `json:"source"`
		Status         string `json:"status"`
		SystemType     string `json:"systemType"`
		VulCategory    string `json:"vulCategory"`
		VulLevel       string `json:"vulLevel"`
		VulName        string `json:"vulName"`
	} `json:"data"`
	message string
}

type OupResultInfo struct {
	CveId             string             // CVE ID
	oupType           OfflineUpgradeType // 离线包类型 unit类型 0 未知  1 系统仓库  2 安全补丁
	CompletenessCheck CheckState         // 完整性检查	unit类型  0 未检查 1 检查通过 2检查不通过
	SystemTypeCheck   CheckState         // 系统版本检查  unit类型  0 未检查 1 检查通过 2检查不通过
	ArchCheck         CheckState         // 架构检查		unit类型  0 未检查 1 检查通过 2检查不通过
}

type OfflineCheckResult struct {
	// 检查oup包即可完成下面5项数据的补充
	OfflineUpgradeType OfflineUpgradeType // 离线包类型 unit类型 0 未知  1 系统仓库  2 安全补丁
	OupCount           int
	OupCheckState      CheckState // 整体检查是否通过 unit类型  0 未检查 1 检查通过 2检查不通过
	CheckResultInfo    map[string]*OupResultInfo
	DiskCheckState     CheckState // 解压空间是否满足 unit类型  0 未检查 1 检查通过 2检查不通过

	// 建立离线仓库检查更新后补充下面两项数据
	DebCount         int        // apt update后,获取可更新包的数量
	SystemCheckState CheckState // apt update后,通过系统更新工具做环境检查 unit类型  0 未检查 1 检查通过 2检查不通过
}

// PrepareUpdateOffline  离线检查更新之前触发：需要完成缓存清理、解压、验签、挂载
func (m *OfflineManager) PrepareUpdateOffline(paths []string, indicator Indicator) error {
	err := m.CleanCache()
	if err != nil {
		return err
	}
	m.checkResult = OfflineCheckResult{
		OfflineUpgradeType: unknown,
		OupCount:           -1,
		OupCheckState:      nocheck,
		CheckResultInfo:    nil,
		DiskCheckState:     nocheck,
		DebCount:           -1,
		SystemCheckState:   nocheck,
	}
	m.checkResult.CheckResultInfo = make(map[string]*OupResultInfo)
	for _, path := range paths {
		m.checkResult.CheckResultInfo[filepath.Base(path)] = &OupResultInfo{}
	}
	m.checkResult.OupCount = len(paths)

	hasSystemOup := false
	hasCVEOup := false

	progressRange := float64(len(paths))
	for index, path := range paths {
		unzipPath, err := unzip(path)
		if err != nil {
			if strings.Contains(err.Error(), "No space left on device") {
				// 空间不足解压失败
				m.checkResult.DiskCheckState = failed
			}
			return err
		}
		m.checkResult.DiskCheckState = success
		info, err := getInfo(unzipPath)
		if err != nil {
			return err
		}
		m.localOupInfoMap[filepath.Base(path)] = info

		// 进行完整性检查、系统版本检查、架构检查
		var checkInfo OupResultInfo
		for {
			err = verify(unzipPath)
			if err != nil {
				checkInfo.CompletenessCheck = failed
				break
			} else {
				checkInfo.CompletenessCheck = success
			}

			if info.Type == offlineSystem {
				hasSystemOup = true
				m.checkResult.OfflineUpgradeType = offlineSystem
			}
			if info.Type == offlineCEV {
				hasCVEOup = true
				m.checkResult.OfflineUpgradeType = offlineCEV
			}
			if hasCVEOup && hasSystemOup {
				err = errors.New("multiple oup file type")
				m.checkResult.OfflineUpgradeType = unknown
				break
			}

			err = systemTypeCheck(info)
			if err != nil {
				checkInfo.SystemTypeCheck = failed
				break
			} else {
				checkInfo.SystemTypeCheck = success
			}

			err = archCheck(info)
			if err != nil {
				checkInfo.ArchCheck = failed
				break
			} else {
				checkInfo.ArchCheck = success
			}
			break
		}
		m.checkResult.CheckResultInfo[filepath.Base(path)] = &checkInfo
		if err != nil {
			m.checkResult.OupCheckState = failed
			return err
		}
		checkInfo.CveId = info.Data.CveId
		checkInfo.oupType = info.Type
		m.checkResult.OupCheckState = success
		// 挂载之后检查更新,获取可更新内容
		mountDir, err := mount(unzipPath)
		if err != nil {
			return err
		}
		m.localOupRepoPaths = append(m.localOupRepoPaths, mountDir)
		indicator(float64(index) / progressRange)
	}
	// 生成离线的list文件
	err = m.updateOfflineSourceFile()
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (m *OfflineManager) GetCheckInfo() OfflineCheckResult {
	return m.checkResult
}

// AfterUpdateOffline 离线检查成功之后触发，汇总前端需要的数据：可升级包数量,磁盘升级空间检测结果
func (m *OfflineManager) AfterUpdateOffline() error {
	packages, err := apt.ListDistUpgradePackages(system.GetCategorySourceMap()[system.OfflineUpdate], []string{
		"-o", "Dir::State::lists=/var/lib/lastore/offline_list",
	})
	if err != nil {
		return err
	}
	m.checkResult.DebCount = len(packages)
	installPkgs, removePkgs, err := apt.GenOnlineUpdatePackagesByEmulateInstall(packages, []string{
		"-o", "Dir::State::lists=/var/lib/lastore/offline_list",
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", system.GetCategorySourceMap()[system.OfflineUpdate]),
		"-o", "Dir::Etc::SourceParts=/dev/null",
	})
	if err != nil {
		return err
	}
	m.upgradeAblePackages = installPkgs
	m.removePackages = removePkgs
	return nil
}

// repos: 离线仓库地址列表 单个地址eg:deb [trusted=yes] file:///home/lee/patch/temp/ eagle main
func (m *OfflineManager) updateOfflineSourceFile() error {
	var repos []string
	for _, dir := range m.localOupRepoPaths {
		repos = append(repos, fmt.Sprintf("deb [trusted=yes] file://%v/ eagle main contrib non-free", dir))
	}
	return ioutil.WriteFile(system.GetCategorySourceMap()[system.OfflineUpdate], []byte(strings.Join(repos, "\n")), 0644)
}

func (m *OfflineManager) CleanCache() error {
	var err error
	dirInfo, err := ioutil.ReadDir(mountFsDir)
	if err == nil {
		for _, info := range dirInfo {
			mountPoint := filepath.Join(mountFsDir, info.Name())
			if info.IsDir() && isMountPoint(mountPoint) {
				cmd := exec.Command("umount", mountPoint)
				var outBuf bytes.Buffer
				cmd.Stdout = &outBuf
				var errBuf bytes.Buffer
				cmd.Stderr = &errBuf
				err = cmd.Run()
				if err != nil {
					logger.Warningf("failed to umount: %v %v", outBuf.String(), errBuf.String())
				}
			}
		}
	}
	m.localOupRepoPaths = []string{}
	m.localUnzipOupDirs = []string{}
	err = os.RemoveAll(mountFsDir)
	if err != nil {
		return err
	}
	return os.RemoveAll(unzipOupDir)
}

func (m *Manager) updateOfflineSource(sender dbus.Sender, paths []string, option string) (job *Job, err error) {
	var environ map[string]string
	if !system.IsAuthorized() || !system.IsActiveCodeExist() {
		return nil, errors.New("not authorized, don't allow to exec update")
	}
	environ, err = makeEnvironWithSender(m, sender)
	if err != nil {
		return nil, err
	}
	m.do.Lock()
	defer m.do.Unlock()
	var isExist bool
	// 如果控制中心正在检查更新，那么离线检查需要特殊处理
	isExist, job, err = m.jobManager.CreateJob("", system.OfflineUpdateJobType, paths, environ, nil)
	if err != nil {
		logger.Warningf("create offline update Job error: %v", err)
		return nil, err
	}
	if isExist {
		logger.Info(JobExistError)
		return job, nil
	}
	// 在线检查更新的索引放到/var/lib/apt/lists
	// 离线检查更新的索引放到/var/lib/lastore/offline_list
	job.option = map[string]string{
		"Dir::Etc::SourceList":  system.GetCategorySourceMap()[system.OfflineUpdate],
		"Dir::Etc::SourceParts": "/dev/null",
		"Dir::State::lists":     "/var/lib/lastore/offline_list",
	}
	job.setPreHooks(map[string]func() error{
		string(system.RunningStatus): func() error {
			err := m.offline.PrepareUpdateOffline(paths, func(progress float64) {
				job.setPropProgress(progress / float64(10))
			})
			logger.Info(m.offline.GetCheckInfo())
			if err != nil {
				logger.Warning(err)
				return &system.JobError{
					Type:   system.ErrorOfflineCheck,
					Detail: "check offline oup file error:" + err.Error(),
				}
			}
			return nil
		},
		string(system.SucceedStatus): func() error {
			err = m.offline.AfterUpdateOffline()
			if err != nil {
				logger.Warning(err)
				return &system.JobError{
					Type:   system.ErrorOfflineCheck,
					Detail: "check offline oup file error:" + err.Error(),
				}
			}
			if len(m.offline.upgradeAblePackages) > 0 {
				if m.offline.checkOfflineSystemState() {
					m.offline.checkResult.SystemCheckState = success
				} else {
					m.offline.checkResult.SystemCheckState = failed
				}
			} else {
				m.offline.checkResult.SystemCheckState = success
			}

			logger.Info(m.offline.GetCheckInfo())
			job.setPropProgress(1)
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.postStatusMessage("offline update check success")
			}()

			return nil
		},
		string(system.FailedStatus): func() error {
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.postStatusMessage(fmt.Sprintf("offline update check failed detail is:%v", job.Description))
			}()
			m.offline.checkResult.SystemCheckState = failed
			return nil
		},
	})
	if err = m.jobManager.addJob(job); err != nil {
		logger.Warning(err)
		return nil, err
	}
	return job, nil
}

func checkRootSpace() bool {
	isSatisfied := false
	addSize, err := system.QuerySourceAddSize(system.OfflineUpdate)
	if err != nil {
		logger.Warning(err)
	}
	content, err := exec.Command("/bin/sh", []string{
		"-c",
		"df -BK --output='avail' /var|awk 'NR==2'",
	}...).CombinedOutput()
	if err != nil {
		logger.Warning(string(content))
	} else {
		spaceStr := strings.Replace(string(content), "K", "", -1)
		spaceStr = strings.TrimSpace(spaceStr)
		spaceNum, err := strconv.Atoi(spaceStr)
		if err != nil {
			logger.Warning(err)
		} else {
			spaceNum = spaceNum * 1000
			isSatisfied = spaceNum > int(addSize)
		}
	}
	return isSatisfied
}

func (m *OfflineManager) checkOfflineSystemState() bool {
	_, err := dut.GenDutMetaFile(system.DutOfflineMetaConfPath,
		"/var/cache/lastore/archives",
		m.upgradeAblePackages,
		m.upgradeAblePackages, nil, m.upgradeAblePackages, m.removePackages, nil, genRepoInfo(system.OfflineUpdate, system.OfflineListPath))
	if err != nil {
		logger.Warning(err)
		return false
	}

	err = dut.CheckSystem(dut.PreCheck, true, nil)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}
