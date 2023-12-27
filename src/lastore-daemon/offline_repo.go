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

type OfflineUpgradeType int

const (
	unknownType   OfflineUpgradeType = 0
	offlineSystem OfflineUpgradeType = 1
	offlineCEV    OfflineUpgradeType = 2
	toBType       OfflineUpgradeType = 3
)

func (t OfflineUpgradeType) string() string {
	switch t {
	case unknownType:
		return "Unknown"
	case offlineSystem:
		return "System"
	case offlineCEV:
		return "CVE"
	case toBType:
		return "Business"
	}
	return fmt.Sprintf("other type:%v", t)
}

type Indicator func(progress float64)
type OfflineManager struct {
	localOupRepoPaths   []string // repo.sfs挂载后的路径
	checkResult         OfflineCheckResult
	upgradeAblePackages map[string]system.PackageInfo // 离线更新可更新包
	removePackages      map[string]system.PackageInfo
}

func NewOfflineManager() *OfflineManager {
	return &OfflineManager{
		localOupRepoPaths: nil,
		// localOupCheckMap:  make(map[string]*OupResultInfo),
	}
}

type CheckState int

const (
	nocheck  CheckState = 0
	success  CheckState = 1
	unknown  CheckState = 2
	failed   CheckState = -1
	partPass CheckState = -2
)

func (t CheckState) string() string {
	switch t {
	case nocheck:
		return "nocheck"
	case success:
		return "success"
	case failed:
		return "failed"
	case partPass:
		return "partPass"
	case unknown:
		return "unknown"
	}
	return ""
}

type OfflineRepoInfo struct {
	Type    OfflineUpgradeType `json:"type"`
	Version string             `json:"version"`
	Data    struct {
		Archs string `json:"archs"`
		// Binary         string `json:"binary"`
		// CveDescription string `json:"cveDescription"`
		CveId string `json:"cveId"`
		// Description    string `json:"description"`
		// FixedVersion   string `json:"fixedVersion"`
		// PubTime        string `json:"pubTime"`
		// Score          string `json:"score"`
		// Source         string `json:"source"`
		// Status         string `json:"status"`
		SystemType string `json:"systemType"`
		// VulCategory    string `json:"vulCategory"`
		// VulLevel       string `json:"vulLevel"`
		// VulName        string `json:"vulName"`
	} `json:"data"`
	message string
}

type OupResultInfo struct {
	CveId             string             // CVE ID
	OupType           OfflineUpgradeType // 离线包类型   int 类型 0 未知  1 系统仓库  2 安全补丁
	CompletenessCheck CheckState         // 完整性检查	int 类型  0 未检查 1 检查通过 -1 检查不通过
	systemTypeCheck   CheckState         // 系统版本检查  int 类型  0 未检查 1 检查通过 -1 检查不通过
	ArchCheck         CheckState         // 架构检查		int 类型  0 未检查 1 检查通过 -1 检查不通过
	infoCheck         CheckState         // info格式检查 int 类型  0 未检查 1 检查通过 -1 检查不通过
	CheckResult       CheckState         // 该oup检查结果 int 类型  0 未检查 1 检查通过 -1 检查不通过  2 未知(检查虽然不通过，但是不影响安装，目前只有升级类型检查不通过会出现未知)
}

type OfflineCheckResult struct {
	// 检查oup包即可完成下面5项数据的补充
	OupCount        int
	OupCheckState   CheckState // 整体检查是否通过 int 类型  0 未检查 1 检查通过 -1 检查不通过 -2 部分通过
	CheckResultInfo map[string]*OupResultInfo
	DiskCheckState  CheckState // 解压空间是否满足 int 类型  0 未检查 1 检查通过 -1 检查不通过

	// 建立离线仓库检查更新后补充下面两项数据
	DebCount         int        // apt update后,获取可更新包的数量
	SystemCheckState CheckState // apt update后,通过系统更新工具做环境检查 int 类型  0 未检查 1 检查通过 -1 检查不通过
}

// PrepareUpdateOffline  离线检查更新之前触发：需要完成缓存清理、解压、验签、挂载
func (m *OfflineManager) PrepareUpdateOffline(paths []string, indicator Indicator) error {
	err := m.CleanCache()
	if err != nil {
		return err
	}
	m.checkResult = OfflineCheckResult{
		OupCount:         len(paths),
		OupCheckState:    nocheck,
		CheckResultInfo:  make(map[string]*OupResultInfo),
		DiskCheckState:   nocheck,
		DebCount:         -1,
		SystemCheckState: nocheck,
	}

	progressRange := float64(len(paths)) // 按照数量设置进度
	checkSuccessOupCount := 0
	checkUnknownOupCount := 0

	for index, path := range paths {
		m.checkResult.CheckResultInfo[filepath.Base(path)] = &OupResultInfo{}

		// 进行完整性检查、系统版本检查、架构检查
		var checkInfo OupResultInfo
		var info OfflineRepoInfo
		m.checkResult.CheckResultInfo[filepath.Base(path)] = &checkInfo
		for {
			var unzipPath string
			// 解压文件，判断错误是否为空间不足的错误
			unzipPath, err = unzip(path)
			if err != nil {
				logger.Warningf("failed to unzip %v error is:%v", path, err)
				if strings.Contains(err.Error(), "No space left on device") {
					// 空间不足解压失败
					m.checkResult.DiskCheckState = failed
					m.checkResult.OupCheckState = failed
					return err // 致命错误，整体阻塞
				}
				// 其他原因导致解压失败，按照完整性检查不通过处理
				logger.Warningf("unzip %v error: %v", unzipPath, err)
				m.checkResult.DiskCheckState = success
				checkInfo.CompletenessCheck = failed
				checkInfo.CheckResult = failed
				break
			}
			m.checkResult.DiskCheckState = success
			// 通过校验工具进行完整性检查
			err = verify(unzipPath)
			if err != nil {
				logger.Warningf("verify %v error: %v", unzipPath, err)
				checkInfo.CompletenessCheck = failed
				checkInfo.CheckResult = failed
				break
			} else {
				checkInfo.CompletenessCheck = success
			}
			// 解析info.json文件，如果解析失败，空的数据继续向后执行
			info, err = getInfo(unzipPath)
			if err != nil {
				checkInfo.infoCheck = failed
				logger.Warningf("get oup info %v error: %v", unzipPath, err)
			} else {
				checkInfo.infoCheck = success
			}

			// 该项检查结果不影响整体结果
			systemTypeErr := systemTypeCheck(info)
			if systemTypeErr != nil {
				logger.Warningf("check systemType %v error: %v", unzipPath, systemTypeErr)
				checkInfo.systemTypeCheck = failed
			} else {
				checkInfo.systemTypeCheck = success
			}

			err = archCheck(info)
			if err != nil {
				logger.Warningf("check arch %v error: %v", unzipPath, err)
				checkInfo.ArchCheck = failed
				checkInfo.CheckResult = failed
				break
			} else {
				checkInfo.ArchCheck = success
			}
			checkInfo.OupType, err = getOupType(info)
			if err != nil {
				// oup类型错误
				logger.Warningf("check OupType %v error: %v", unzipPath, err)
				checkUnknownOupCount++
				checkInfo.CheckResult = unknown
			} else {
				checkSuccessOupCount++
				checkInfo.CheckResult = success
			}
			checkInfo.CveId = info.Data.CveId

			// 挂载检查通过或者为未知的repo.sfs
			// 挂载之后检查更新,获取可更新内容
			mountDir, err := mount(unzipPath)
			if err != nil {
				logger.Warningf("failed to mount %v error: %v", unzipPath, err)
				// 挂载出错，通常为文件错误
				checkInfo.CompletenessCheck = failed
				checkInfo.CheckResult = failed
				break
			}
			m.localOupRepoPaths = append(m.localOupRepoPaths, mountDir)
			break
		}
		indicator(float64(index) / progressRange)
	}
	switch checkSuccessOupCount {
	case 0:
		if checkUnknownOupCount > 0 {
			m.checkResult.OupCheckState = partPass
		} else {
			m.checkResult.OupCheckState = failed
		}
	case m.checkResult.OupCount:
		m.checkResult.OupCheckState = success
	default:
		m.checkResult.OupCheckState = partPass
	}
	// 生成离线的list文件
	err = updateOfflineSourceFile(m.localOupRepoPaths)
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (m *OfflineManager) GetCheckInfo() OfflineCheckResult {
	return m.checkResult
}

func (m *OfflineManager) PrintCheckResult() {
	logger.Infof("oup count is %v", m.checkResult.OupCount)
	logger.Infof("all oup check state is %v", m.checkResult.OupCheckState.string())
	for name, info := range m.checkResult.CheckResultInfo {
		logger.Infof("%v check result:%v detail is cveId:%v infoCheck:%v CompletenessCheck:%v oupType:%v ArchCheck:%v systemTypeCheck:%v",
			name, info.CheckResult.string(), info.CveId,
			info.infoCheck.string(), info.CompletenessCheck.string(), info.OupType.string(), info.ArchCheck.string(), info.systemTypeCheck.string())
	}
	logger.Infof("disk check is %v", m.checkResult.DiskCheckState.string())
	logger.Infof("upgradable deb count is %v", m.checkResult.DebCount)
	logger.Infof("system check is %v", m.checkResult.SystemCheckState.string())
}

// AfterUpdateOffline 离线检查成功之后触发，汇总前端需要的数据：可升级包数量,磁盘升级空间检测结果
func (m *OfflineManager) AfterUpdateOffline() error {
	installPkgs, removePkgs, err := apt.GenOnlineUpdatePackagesByEmulateInstall(nil, []string{
		"-o", "Dir::State::lists=/var/lib/lastore/offline_list",
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%v", system.GetCategorySourceMap()[system.OfflineUpdate]),
		"-o", "Dir::Etc::SourceParts=/dev/null",
	})
	if err != nil {
		return err
	}

	m.checkResult.DebCount = len(installPkgs)
	m.upgradeAblePackages = installPkgs
	m.removePackages = removePkgs
	return nil
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
			m.offline.PrintCheckResult()
			if err != nil {
				logger.Warning(err)
				cleanErr := m.offline.CleanCache()
				if cleanErr != nil {
					logger.Warning(cleanErr)
				}
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
					m.offline.checkResult.DebCount = -1
				}
			} else {
				m.offline.checkResult.SystemCheckState = success
			}

			m.offline.PrintCheckResult()
			job.setPropProgress(1)
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.PostStatusMessage("offline update check success")
			}()

			return nil
		},
		string(system.FailedStatus): func() error {
			go func() {
				m.inhibitAutoQuitCountAdd()
				defer m.inhibitAutoQuitCountSub()
				m.updatePlatform.PostStatusMessage(fmt.Sprintf("offline update check failed detail is:%v", job.Description))
			}()
			m.offline.checkResult.SystemCheckState = failed
			m.offline.checkResult.DebCount = -1
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
		system.LocalCachePath,
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
