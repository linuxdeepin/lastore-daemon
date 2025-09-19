package check

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jouyouyun/hardware/utils"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/sysinfo"
)

const CheckBaseDir = "/var/lib/lastore/check/"

var sysRealArch string

func init() {
	cmd := exec.Command("/usr/bin/dpkg", "--print-architecture")

	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err == nil {
		sysRealArch = strings.TrimSpace(output.String())
	}
}

// DONE(heysion): 修改错误返回
func PreCheckLoadSysPkgInfo(pkgs map[string]*cache.AppTinyInfo) (int64, error) {
	if err := sysinfo.GetCurrInstPkgStat(pkgs); err != nil {
		return ecode.CHK_SYS_PKG_INFO_LOAD_ERROR, err
	}
	return ecode.CHK_PROGRAM_SUCCESS, nil
}

// TODO:（DingHao）sysinfo.GetSysPkgStateAndVersion替换成袁老师的查询函数，添加日志打印
func CheckAPTAndDPKGState() (int64, error) {

	// check dpkg and apt is
	if flags, _ := sysinfo.CheckAppIsExist("/usr/bin/apt"); !flags {
		return ecode.CHK_TOOLS_DEPEND_ERROR, fmt.Errorf("/usr/bin/apt not found")
	}
	if flags, _ := sysinfo.CheckAppIsExist("/usr/bin/dpkg"); !flags {
		return ecode.CHK_TOOLS_DEPEND_ERROR, fmt.Errorf("/usr/bin/dpkg not found")
	}
	aptState, _, err := sysinfo.GetSysPkgStateAndVersion("apt")
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("apt err: %v", err)
	}
	if aptState != "ii" {
		return ecode.CHK_APT_STATE_ERROR, fmt.Errorf("apt state err: %s", aptState)
	}

	dpkgState, _, err := sysinfo.GetSysPkgStateAndVersion("dpkg")
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("dpkg err: %v", err)
	}
	if dpkgState != "ii" {
		return ecode.CHK_DPKG_STATE_ERROR, fmt.Errorf("dpkg state err: %s", dpkgState)
	}

	return ecode.CHK_PROGRAM_SUCCESS, nil
}

// dyn hook
func CheckDynHook(cfg *cache.CacheInfo, checkType int8) (int64, error) {
	execHooks := func(hookDir string) error {
		hookFiles, err := utils.ScanDir(hookDir, func(name string) bool {
			return !strings.HasSuffix(name, "sh")
		})

		if err != nil {
			return fmt.Errorf("scan hook dir error: %v", err)
		}

		//遍历执行脚本
		for _, hookFile := range hookFiles {
			hookPath := filepath.Join(hookDir, hookFile)
			log.Infof("Executing hook: %s", hookPath)
			output, err := runcmd.RunnerOutput(60, "bash", hookPath)
			if err != nil {
				return fmt.Errorf("hook execution failed: %s\nOutput:\n%s", hookPath, output)
			}
			log.Infof("Hook executed successfully: %s\nOutput:\n%s", hookPath, output)
		}
		return nil
	}

	var err error
	switch checkType {
	case cache.PreUpdate:
		err = execHooks(filepath.Join(CheckBaseDir, "pre_check"))
	case cache.UpdateCheck:
		err = execHooks(filepath.Join(CheckBaseDir, "mid_check"))
	case cache.PostCheck:
		err = execHooks(filepath.Join(CheckBaseDir, "post_check"))
	default:
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("check type error")
	}

	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("check hook error: %v", err)
	}

	return ecode.CHK_PROGRAM_SUCCESS, nil
}

// check root disk free space more need space
func CheckRootDiskFreeSpace(needSpace uint64) (int64, error) {
	diskFree, err := sysinfo.GetRootDiskFreeSpace()
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("check disk free space err: %v", err)
	}
	if diskFree < needSpace {
		log.Warnf("root disk free space is less %dM, is %dM", needSpace/1024, diskFree/1024)
		return ecode.CHK_SYS_DISK_OUT_SPACE, fmt.Errorf("root disk free space is less than %dM, is %dM", needSpace/1024, diskFree/1024)
	}
	log.Debugf("root disk free space is greater than or equal %dM", needSpace/1024)
	return ecode.CHK_PROGRAM_SUCCESS, nil
}

// check data disk free space more need space
func CheckDataDiskFreeSpace(needSpace uint64) (int64, error) {
	diskFree, err := sysinfo.GetDataDiskFreeSpace()
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("check disk free space err: %v", err)
	}
	if diskFree < needSpace {
		log.Warnf("data disk free space is less %dM, is %dM", diskFree/1024, needSpace/1024)
		return ecode.CHK_SYS_DISK_OUT_SPACE, fmt.Errorf("data disk free space is less than %dM, is %dM", diskFree/1024, needSpace/1024)
	}
	log.Infof("data free space is greater than or equal %dM", needSpace/1024)
	return ecode.CHK_PROGRAM_SUCCESS, nil
}

func AdjustPkgArchWithName(cache *cache.CacheInfo) {
	// reset arch with package name
	for idx, pkginfo := range cache.UpdateMetaInfo.PkgList {
		archIdex := strings.Index(pkginfo.Name, ":")
		if archIdex > 0 {
			cache.UpdateMetaInfo.PkgList[idx].Name = pkginfo.Name[:archIdex]
			cache.UpdateMetaInfo.PkgList[idx].Arch = strings.TrimSpace(pkginfo.Name[archIdex+1:])
		}
	}
}

func CheckPurgeList(cache *cache.CacheInfo, syspkgs map[string]*cache.AppTinyInfo) error {

	for _, pkginfo := range cache.UpdateMetaInfo.PurgeList {
		if syspkginfo, ok := syspkgs[pkginfo.Name]; ok {
			//log.Debugf("log:%v", syspkginfo)
			switch pkginfo.Need {
			case "exist":
			case "skipversion":
				continue
			case "":
			case "skipstate":
			case "strict":
				if pkginfo.Version == syspkginfo.Version {
					continue
				} else {
					log.Infof("purge package info %v != %v", pkginfo, syspkginfo)
					return fmt.Errorf("purge package version not match %s", pkginfo.Name)
				}
			default:
				continue
			}
		} else {
			if cache.InternalState.IsPurgeState.IsFirstRun() {
				return fmt.Errorf("purge package not found :%s", pkginfo.Name)
			} else {
				log.Warnf("purge package skip:%s", pkginfo.Name)
				continue
			}
		}
	}

	return nil
}
