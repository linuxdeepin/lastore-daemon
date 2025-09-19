package check

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	runcmd "github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/cmd"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

var ProgramCheckMap = map[string][]string{
	"stage1": []string{
		"/usr/sbin/lightdm",
	},
	"stage2": []string{
		"/usr/sbin/lightdm",
	},
}

func CheckImportantProgress(stage string) (int64, error) {
	if programCheckList, ok := ProgramCheckMap[stage]; ok {
		for _, program := range programCheckList {
			pidCheckCmd := "pidof " + program
			programPid, err := runcmd.RunnerOutput(10, "bash", "-c", pidCheckCmd)
			if err != nil {
				return ecode.CHK_PROGRAM_ERROR, err
			}
			if len(programPid) == 0 {
				return ecode.CHK_IMPORTANT_PROGRESS_NOT_RUNNING, fmt.Errorf("%s not running", program)
			}
		}
	} else {
		return ecode.CHK_PROGRAM_ERROR, fmt.Errorf("%s is error postcheck stage parameter", stage)
	}
	return ecode.CHK_PROGRAM_SUCCESS, nil

}

func LogRemoveSensitiveInformation(logPath string) (int64, error) {

	// 读取 dpkg.log 文件
	content, err := ioutil.ReadFile(logPath)
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, err
	}
	// 获取用户名
	usrName, err := runcmd.RunnerOutput(10, "bash", "-c", "who|awk '{print $1}'")
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, err
	}
	// 将用户名替换为 "user-name"
	newContent := strings.ReplaceAll(string(content), usrName, "user-name")

	// 将替换后的内容写入 dpkg-archive.log 文件
	newLogPath := strings.Replace(logPath, ".log", "-archive.log", 1)
	err = ioutil.WriteFile(newLogPath, []byte(newContent), 0755)
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, err
	}
	return ecode.CHK_PROGRAM_SUCCESS, nil
}

func ArchiveLogAndCache(uuid string) (int64, error) {
	uuidDir := CheckBaseDir + uuid
	archivePath := uuidDir + "-archive.tar.gz"
	cachePath := uuidDir + "/" + "cache"
	cachesFile := CheckBaseDir + "caches.yaml"
	TarArgs := []string{"-czvf", archivePath}

	if err := fs.CheckFileExistState(archivePath); err == nil {
		log.Debugf("The archive file %s has exists and will not archive generate filed", archivePath)
		return ecode.CHK_PROGRAM_SUCCESS, nil
	}
	if err := fs.CheckFileExistState(uuidDir); err != nil {
		log.Warn(err)
		return ecode.CHK_UUID_DIR_NOT_EXIST, err
	}

	if err := fs.CheckFileExistState(cachesFile); err == nil {
		TarArgs = append(TarArgs, cachesFile)
	} else {
		log.Debug(err)
	}

	if err := fs.CheckFileExistState(cachePath); err == nil {
		TarArgs = append(TarArgs, cachePath)
	} else {
		log.Debug(err)
	}

	// 检查目录下是否存在.log文件
	uuidLogs, err := filepath.Glob(filepath.Join(uuidDir, "*.log"))
	if err != nil {
		return ecode.CHK_PROGRAM_ERROR, err
	}

	if len(uuidLogs) == 0 {
		log.Debugf("%s log file not exist.", uuidDir)
	} else {
		for _, uuidLog := range uuidLogs {
			if _, err := LogRemoveSensitiveInformation(uuidLog); err != nil {
				return ecode.CHK_LOG_RM_SENSITIVE_INFO_FAILED, err
			}
		}
		archiveLogs, err := filepath.Glob(filepath.Join(uuidDir, "*-archive.log"))
		if err != nil {
			return ecode.CHK_PROGRAM_ERROR, err
		}
		if len(uuidLogs) == len(archiveLogs) {
			TarArgs = append(TarArgs, archiveLogs...)
		}
	}

	if len(TarArgs) > 2 {
		if _, err := runcmd.RunnerOutput(10, "tar", TarArgs...); err != nil {
			return ecode.CHK_PROGRAM_ERROR, err
		}
	} else {
		log.Infof("No file to archive.")
	}

	return ecode.CHK_PROGRAM_SUCCESS, nil
}

func DeleteUpgradeCacheFile(uuid string) (int64, error) {
	uuidDir := CheckBaseDir + uuid
	archivePath := uuidDir + "-archive.tar.gz"
	if err := fs.CheckFileExistState(archivePath); err == nil {
		if err := os.RemoveAll(uuidDir); err != nil {
			return ecode.CHK_PROGRAM_ERROR, err
		} else {
			return ecode.CHK_PROGRAM_SUCCESS, nil
		}
	} else {
		log.Infof("The %s archive file does not exist, and the uuid directory will not be cleaned", archivePath)
		return ecode.CHK_PROGRAM_SUCCESS, nil
	}

}
