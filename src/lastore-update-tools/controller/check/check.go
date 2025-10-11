package check

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jouyouyun/hardware/utils"
	"github.com/linuxdeepin/go-lib/log"
	utils2 "github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/sysinfo"
)

const CheckBaseDir = "/var/lib/lastore/check/"

var logger = log.NewLogger("lastore/update-tools/check")

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
func LoadSysPkgInfo(pkgs map[string]*cache.AppTinyInfo) error {
	if err := sysinfo.GetCurrInstPkgStat(pkgs); err != nil {
		return &system.JobError{
			ErrType:      system.ErrorSysPkgInfoLoad,
			ErrDetail:    fmt.Sprintf("load system package info error: %v", err),
			IsCheckError: true,
		}
	}
	return nil
}

// TODO:（DingHao）sysinfo.GetSysPkgStateAndVersion替换成袁老师的查询函数，添加日志打印
func CheckAPTAndDPKGState() error {

	// check dpkg and apt is
	if flags, _ := sysinfo.CheckAppIsExist("/usr/bin/apt"); !flags {
		return &system.JobError{
			ErrType:      system.ErrorCheckToolsDependFailed,
			ErrDetail:    fmt.Sprintf("/usr/bin/apt not found"),
			IsCheckError: true,
		}
	}
	if flags, _ := sysinfo.CheckAppIsExist("/usr/bin/dpkg"); !flags {
		return &system.JobError{
			ErrType:      system.ErrorCheckToolsDependFailed,
			ErrDetail:    fmt.Sprintf("/usr/bin/dpkg not found"),
			IsCheckError: true,
		}
	}
	aptState, _, err := sysinfo.GetSysPkgStateAndVersion("apt")
	if err != nil {
		return &system.JobError{
			ErrType:      system.ErrorCheckToolsDependFailed,
			ErrDetail:    fmt.Sprintf("apt err: %v", err),
			IsCheckError: true,
		}
	}
	if aptState != "ii" {
		return &system.JobError{
			ErrType:      system.ErrorCheckToolsDependFailed,
			ErrDetail:    fmt.Sprintf("apt state err: %s", aptState),
			IsCheckError: true,
		}
	}

	dpkgState, _, err := sysinfo.GetSysPkgStateAndVersion("dpkg")
	if err != nil {
		return &system.JobError{
			ErrType:      system.ErrorCheckToolsDependFailed,
			ErrDetail:    fmt.Sprintf("dpkg err: %v", err),
			IsCheckError: true,
		}
	}
	if dpkgState != "ii" {
		return &system.JobError{
			ErrType:      system.ErrorCheckToolsDependFailed,
			ErrDetail:    fmt.Sprintf("dpkg state err: %s", dpkgState),
			IsCheckError: true,
		}
	}

	return nil
}

// dyn hook
func CheckDynHook(cfg *cache.CacheInfo, checkType int8) error {
	execHooks := func(hookDir string) error {
		// 检查hook目录是否存在
		if !utils2.IsFileExist(hookDir) {
			logger.Warningf("hook dir %s not exist", hookDir)
			return nil
		}

		hookFiles, err := utils.ScanDir(hookDir, func(name string) bool {
			return !strings.HasSuffix(name, "sh")
		})
		//logger.Infof("hookFiles: %v", hookFiles)
		if err != nil {
			return fmt.Errorf("scan hook dir error: %v", err)
		}

		//遍历执行脚本
		for _, hookFile := range hookFiles {
			hookPath := filepath.Join(hookDir, hookFile)
			logger.Infof("Executing hook: %s", hookPath)
			output, err := runcmd.RunnerOutput(60, "bash", hookPath)
			if err != nil {
				return fmt.Errorf("hook execution failed: %s\nOutput:\n%s\nError:%s", hookPath, output, err.Error())
			}
			logger.Infof("Hook executed successfully: %s\nOutput:\n%s", hookPath, output)
		}
		return nil
	}

	var err error
	switch checkType {
	case cache.PreUpdate:
		err = execHooks(filepath.Join(CheckBaseDir, "pre_check"))
	case cache.MidCheck:
		err = execHooks(filepath.Join(CheckBaseDir, "mid_check"))
	case cache.PostCheck:
		err = execHooks(filepath.Join(CheckBaseDir, "post_check"))
	default:
		return fmt.Errorf("check type error")
	}

	if err != nil {
		return fmt.Errorf("check hook error: %v", err)
	}

	return nil
}

// check root disk free space more need space
func CheckRootDiskFreeSpace(needSpace uint64) error {
	diskFree, err := sysinfo.GetRootDiskFreeSpace()
	if err != nil {
		return &system.JobError{
			ErrType:      system.ErrorCheckProgramFailed,
			ErrDetail:    fmt.Sprintf("check disk free space err: %v", err),
			IsCheckError: true,
		}
	}
	if diskFree < needSpace {
		logger.Warningf("root disk free space is less %dM, is %dM", needSpace/1024, diskFree/1024)
		return &system.JobError{
			ErrType:      system.ErrorCheckSysDiskOutSpace,
			ErrDetail:    fmt.Sprintf("root disk free space is less than %dM, is %dM", needSpace/1024, diskFree/1024),
			IsCheckError: true,
		}
	}
	logger.Debugf("root disk free space is greater than or equal %dM", needSpace/1024)
	return nil
}

// check data disk free space more need space
func CheckDataDiskFreeSpace(needSpace uint64) error {
	diskFree, err := sysinfo.GetDataDiskFreeSpace()
	if err != nil {
		return &system.JobError{
			ErrType:      system.ErrorCheckProgramFailed,
			ErrDetail:    fmt.Sprintf("check disk free space err: %v", err),
			IsCheckError: true,
		}
	}
	if diskFree < needSpace {
		logger.Warningf("data disk free space is less %dM, is %dM", diskFree/1024, needSpace/1024)
		return &system.JobError{
			ErrType:      system.ErrorCheckSysDiskOutSpace,
			ErrDetail:    fmt.Sprintf("data disk free space is less than %dM, is %dM", diskFree/1024, needSpace/1024),
			IsCheckError: true,
		}
	}
	logger.Infof("data free space is greater than or equal %dM", needSpace/1024)
	return nil
}
