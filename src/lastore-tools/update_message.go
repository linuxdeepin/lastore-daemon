package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
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

type tokenMessage struct {
	Result bool            `json:"result"`
	Code   int             `json:"code"`
	Data   json.RawMessage `json:"data"`
}
type tokenErrorMessage struct {
	Result bool   `json:"result"`
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
}

type updateMessage struct {
	SystemType string     `json:"systemType"`
	Version    Version    `json:"version"`
	Policy     Policy     `json:"policy"`
	RepoInfos  []repoInfo `json:"repoInfos"`
}

type Version struct {
	Version  string `json:"version"`
	Baseline string `json:"baseline"`
}

type Policy struct {
	Tp int `json:"tp"`

	Data interface {
	} `json:"data"`
}

type repoInfo struct {
	Uri      string `json:"uri"`
	Cdn      string `json:"cdn"`
	CodeName string `json:"codename"`
	Version  string `json:"version"`
}

type requestType struct {
	path   string
	method string
}

const (
	GetVersion = iota
	PostProcess
)

var Urls = map[uint32]requestType{
	GetVersion: {
		"/api/v1/version",
		"GET",
	},
	PostProcess: {
		"/api/v1/process",
		"POST",
	},
}

type report struct {
	url             string
	hardwareId      string
	currentBaseline string
	targetBaseline  string
	timestamp       time.Time
	sign            string
	token           string
}

// 上传更新日志
func (r *report) ReportLog(reqType uint32, body io.Reader) (data interface{}, err error) {
	// 设置请求url
	policyUrl := r.url + Urls[reqType].path
	client := &http.Client{
		Timeout: 4 * time.Second,
	}

	request, err := http.NewRequest(Urls[reqType].method, policyUrl, body)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	// 设置header
	if reqType == PostProcess {
		// 如果是更新过程日志上报，设置header
		request.Header.Set("X-MachineID", r.hardwareId)
		request.Header.Set("X-CurrentBaseline", r.currentBaseline)
		request.Header.Set("X-Baseline", r.targetBaseline)
		request.Header.Set("X-Time", fmt.Sprintf("%d", r.timestamp.Unix()))
		request.Header.Set("X-Sign", r.sign)
		logger.Debugf("report:%+v", r)
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(r.token)))
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	var respData []byte

	switch response.StatusCode {
	case http.StatusOK:
		respData, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		logger.Debugf("request for %s, respData:%s", policyUrl, string(respData))
		msg := &tokenMessage{}
		err = json.Unmarshal(respData, msg)
		if err != nil {
			return nil, err
		}
		if !msg.Result {
			errorMsg := &tokenErrorMessage{}
			err = json.Unmarshal(respData, errorMsg)
			if err != nil {
				return nil, err
			}
			err = fmt.Errorf("request for %s err:%s", policyUrl, errorMsg.Msg)
			return nil, err
		}
		switch reqType {
		case GetVersion:
			tmp := updateMessage{}
			err = json.Unmarshal(msg.Data, &tmp)
			if err != nil {
				return nil, err
			}
			data = tmp
		case PostProcess:
			return
		default:
			err = fmt.Errorf("unknown report type:%d", reqType)
			return
		}
	default:
		err = fmt.Errorf("request for %s failed, response code=%d", policyUrl, response.StatusCode)
	}
	return
}

func (r *report) tarFiles(files []string) string {
	// 临时文件在/tmp中，文件名格式为：update_«machine-id»_«uuid»_«年月日时分秒微秒»
	filename := fmt.Sprintf("/tmp/%s_%s_%s_%s.tar", "update", r.hardwareId, utils.GenUuid(), r.timestamp.Format("20231019102233444"))
	// 创建tar包文件
	tarFile, err := os.Create(filename)
	if err != nil {
		logger.Warning("create tar failed:", err)
		return ""
	}
	defer tarFile.Close()

	// 创建tar包写入器
	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()
	// 将文件添加到tar包中
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			logger.Warning("open file failed:", err)
			return ""
		}
		defer file.Close()

		// 获取文件信息
		info, err := file.Stat()
		if err != nil {
			logger.Warning("get file info err:", err)
			return ""
		}

		// 创建tar头部信息
		header := new(tar.Header)
		header.Name = filepath.Base(filePath)
		header.Size = info.Size()
		header.Mode = int64(info.Mode())
		header.ModTime = info.ModTime()

		// 写入tar头部信息
		if err := tarWriter.WriteHeader(header); err != nil {
			logger.Warning("create tar header failed:", err)
			return ""
		}

		// 写入文件内容到tar包
		if _, err := io.Copy(tarWriter, file); err != nil {
			logger.Warning("input data to tar failed:", err)
			return ""
		}
	}
	return filename
}

// 日志文件打包压缩，设置sign
func (r *report) compress(files []string) string {
	tarFilename := r.tarFiles(files)
	if tarFilename == "" {
		logger.Warning("tar files failed", files)
		return ""
	}

	tarFile, err := os.Open(tarFilename)
	if err != nil {
		logger.Warning("open file failed:", err)
		return ""
	}
	defer tarFile.Close()

	// 创建临时文件保存压缩后的数据
	xzFilename := tarFilename + ".xz"
	xzFile, err := os.Create(xzFilename)
	if err != nil {
		logger.Warning("create file failed:", err)
		return ""
	}

	// 使用外部命令执行xz压缩
	xzCmd := exec.Command("xz", "-z", "-c")
	xzCmd.Stdin = tarFile
	xzCmd.Stdout = xzFile
	if err := xzCmd.Run(); err != nil {
		xzFile.Close()
		logger.Warning("exec command err:", err)
		return ""
	}
	xzFile.Close()

	xzFile, err = os.Open(xzFilename)
	if err != nil {
		logger.Warning("open xz file failed:", err)
		return ""
	}
	defer xzFile.Close()

	hasher := sha256.New()
	buffer := bytes.NewBufferString(fmt.Sprintf("%v%v", secret, r.timestamp.Unix()))
	// logger.Info(fmt.Sprintf("%v%v", secret, r.timestamp.Unix()))

	reader := io.MultiReader(buffer, xzFile)
	if _, err := io.Copy(hasher, reader); err != nil {
		return ""
	}

	r.sign = base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(hasher.Sum(nil))))

	return xzFilename
}

// 日志收集，并上报更新平台
func (r *report) collectLogs() {
	// 更新一下时间戳
	r.timestamp = time.Now()
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
	// 压缩文件，并计算x-sign作为header
	xzFilename := r.compress(newFiles)
	if xzFilename == "" {
		logger.Warning("compress failed", newFiles)
		return
	}
	xzFile, err := os.Open(xzFilename)
	if err != nil {
		logger.Warning("open file failed:", err)
		return
	}
	defer xzFile.Close()
	r.ReportLog(PostProcess, xzFile)
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

// 检查更新时将token数据发送给更新平台，获取本次更新信息
func (r *report) getTargetBaseline() {
	data, err := r.ReportLog(GetVersion, nil)
	if err != nil {
		logger.Warning(err)
	}
	msg, ok := data.(updateMessage)
	if !ok {
		logger.Warning("bad format")
	}
	r.targetBaseline = msg.Version.Baseline
}

func UpdateMonitor() error {
	logger.Debug("UpdateMonitor")
	r := &report{
		url:             "https://update-platform-pre.uniontech.com",
		currentBaseline: getCurrentBaseline(),
		timestamp:       time.Now(),
		token:           genToken(),
	}

	// 如果是更新过程日志上报，设置header
	hardwareId, err := getHardwareId()
	if err != nil {
		return err
	}
	r.hardwareId = hardwareId

	// 先获取当前环境的token和baseline信息，放置更新过程中被篡改
	r.getTargetBaseline()
	logger.Debugf("report:%+v", r)

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
		r.collectLogs()
	}
	return nil
}
