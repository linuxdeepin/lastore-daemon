package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"

	. "internal/config"

	"internal/system"
	"internal/updateplatform"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	JobStatusSucceed = "succeed"
	JobStatusFailed  = "failed"
	JobStatusEnd     = "end"

	lastoreDBusDest = "com.deepin.lastore"

	aptHistoryLog = "/var/log/apt/history.log"
	aptTermLog    = "/var/log/apt/term.log"

	secret = "DflXyFwTmaoGmbDkVj8uD62XGb01pkJn"
)

var monitorPath = []string{
	"/com/deepin/lastore/Jobsystem_upgrade",
	"/com/deepin/lastore/Jobdist_upgrade",
}

var logFiles = []string{
	// "/var/log/dpkg.log",
	aptHistoryLog,
	aptTermLog,
}

// TODO: 根据具体情况再补充脱敏信息
func desensitize(input string) string {
	userReg := regexp.MustCompile(`Requested-By: (.+?) \((.+?)\)`) // 用户信息(用户名，uid)
	input = userReg.ReplaceAllString(input, "Requested-By: *** (***)")
	return input
}

// 日志脱敏
func maskLogfile(file string) (string, error) {
	inputFile, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer inputFile.Close()

	outputFilePath := "/tmp/" + filepath.Base(file)
	// 创建新文件
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return "", err
	}
	defer outputFile.Close()

	switch file {
	case aptHistoryLog:
		// 使用scanner的话，一行太长会报错
		reader := bufio.NewReader(inputFile)
		for {
			// 使用ReadString方法读取一行内容，直到遇到换行符\n为止
			line, err := reader.ReadString('\n')

			// 日志内容脱敏
			line = desensitize(line)

			// 写入新文件
			if err != nil {
				io.WriteString(outputFile, line)
				break
			} else {
				_, err = io.WriteString(outputFile, line)
				if err != nil {
					break
				}
			}
		}
	default:
		_, err := io.Copy(outputFile, inputFile)
		if err != nil {
			return "", err
		}
	}

	return outputFilePath, nil
}

// 日志收集，并上报更新平台
func collectLogs() {
	newFiles := make([]string, 0)
	for _, logFile := range logFiles {
		logger.Debug("collectLogs", logFile)
		newFile, err := maskLogfile(logFile)
		if err != nil {
			logger.Warning("mask log file failed", logFile, err)
			continue
		}
		logger.Debug("maskLogfile", newFile)
		newFiles = append(newFiles, newFile)
	}
	updatePlatform.PostUpdateLogFiles(newFiles)
}

func getUpdateJosStatusProperty(conn *dbus.Conn, jobPath string) string {
	var variant dbus.Variant
	err := conn.Object(lastoreDBusDest, dbus.ObjectPath(jobPath)).Call(
		"org.freedesktop.DBus.Properties.Get", 0, "com.deepin.lastore.Job", "Status").Store(&variant)
	if err != nil {
		logger.Warning(err, jobPath)
		return ""
	}
	ret := variant.Value().(string)
	return ret
}

// 监听job状态
func monitorJobStatusChange(jobPath string) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	// 检查一下Job的状态。如果failed，直接退出上报日志
	if getUpdateJosStatusProperty(sysBus, jobPath) == JobStatusFailed {
		return errors.New("job Failed")
	}

	rule := dbusutil.NewMatchRuleBuilder().ExtPropertiesChanged(jobPath,
		"com.deepin.lastore.Job").Sender(lastoreDBusDest).Build()
	err = rule.AddTo(sysBus)
	if err != nil {
		return err
	}

	ch := make(chan *dbus.Signal, 10)
	sysBus.Signal(ch)

	defer func() {
		sysBus.RemoveSignal(ch)
		err := rule.RemoveFrom(sysBus)
		if err != nil {
			logger.Warning("RemoveMatch failed:", err)
		}
		logger.Info("monitorJobStatusChange return", jobPath)
	}()

	for v := range ch {
		if len(v.Body) != 3 {
			continue
		}

		props, _ := v.Body[1].(map[string]dbus.Variant)
		status, ok := props["Status"]
		if !ok {
			continue
		}
		statusStr, _ := status.Value().(string)
		logger.Info("job status changed", jobPath, statusStr)
		switch statusStr {
		case JobStatusSucceed:
			return nil
		case JobStatusFailed:
			return errors.New("job Failed")
			// case JobStatusEnd: // 只关注成功和失败的结果，end不作为更新结束
			// 	return nil
		}
	}
	return nil
}

var updatePlatform *updateplatform.UpdatePlatformManager

func UpdateMonitor() error {
	config := NewConfig(path.Join(system.VarLibDir, "config.json"))
	updatePlatform = updateplatform.NewUpdatePlatformManager(config, true)
	err := updatePlatform.GenUpdatePolicyByToken(false)
	if err != nil {
		logger.Warning("gen update info failed:", err)
		return err
	}
	needReport := make(chan bool)
	for _, path := range monitorPath {
		go func(path string) {
			// 两个path只会执行一个,但是得同时监听两个
			err := monitorJobStatusChange(path)
			if err != nil {
				// 一旦有任务失败，则上传更新日志
				needReport <- true
			} else {
				needReport <- false
			}
		}(path)
	}
	res := <-needReport
	if res {
		collectLogs()
	}
	return nil
}
