// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"bufio"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/jouyouyun/hardware/utils"
	utils2 "github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/lastore-daemon/src/internal/system"

	hhardware "github.com/jouyouyun/hardware"
)

const (
	cpuInfoFilePath    = "/proc/cpuinfo"
	cpuKeyDelim        = ":"
	cpuKeyName         = "model name"
	cpuKeySWCPU        = "cpu"
	cpuKeyARMProcessor = "Processor"
	cpuKeyHWHardware   = "Hardware"

	lscpuKeyModelName = "Model name"
	lscpuKeyDelim     = ":"
	lscpuCmd          = "lscpu"
)

const (
	customInfoPath = "/usr/share/deepin/custom-note/info.json"
)

type SystemInfo struct {
	SystemName      string
	ProductType     string
	EditionName     string
	Version         string
	HardwareId      string
	Processor       string
	Arch            string
	Custom          string
	SN              string
	HardwareVersion string
	OEMID           string
	ProjectId       string
	Baseline        string
	SystemType      string
	MachineType     string
	Mac             string
}

const (
	OemNotCustomState = "0"
	OemCustomState    = "1"
)

func getSystemInfo(includeDiskInfo bool) SystemInfo {
	systemInfo := SystemInfo{
		Custom: OemNotCustomState,
	}

	osVersionInfoMap, err := GetOSVersionInfo(CacheVersion)
	if err != nil {
		logger.Warning("failed to get os-version:", err)
	} else {
		systemInfo.SystemName = osVersionInfoMap["SystemName"]
		systemInfo.ProductType = osVersionInfoMap["ProductType"]
		systemInfo.EditionName = osVersionInfoMap["EditionName"]
		systemInfo.Version = strings.Join([]string{
			osVersionInfoMap["MajorVersion"],
			osVersionInfoMap["MinorVersion"],
			osVersionInfoMap["OsBuild"]},
			".")
	}

	systemInfo.HardwareId = GetHardwareId(includeDiskInfo)
	if err != nil {
		logger.Warning("failed to get hardwareId:", err)
	}

	systemInfo.Processor, err = getProcessorModelName()
	if err != nil {
		logger.Warning("failed to get modelName:", err)
	} else if len(systemInfo.Processor) > 100 {
		systemInfo.Processor = systemInfo.Processor[0:100] // 按照需求,长度超过100时,只取前100个字符
	}

	systemInfo.Arch, err = GetArchInfo()
	if err != nil {
		logger.Warning("failed to get Arch:", err)
	}
	systemInfo.SN, err = getSN()
	if err != nil {
		logger.Warning("failed to get SN:", err)
	}
	isCustom, oemId, err := getCustomInfoAndOemId()
	if err != nil {
		logger.Warningf("failed to get oemId by /etc/oem-info :%v, start get oemId by /etc/.oemid", err)
		systemInfo.Custom = OemNotCustomState
		// 当从oem-info文件解析出错(通常为文件不存在的情况),需要从/etc/.oemid重新读取oemid
		oemId, err = getOEMID()
		if err != nil {
			logger.Warning(err)
		}
	} else if isCustom {
		systemInfo.Custom = OemCustomState
	}
	systemInfo.OEMID = oemId
	systemInfo.HardwareVersion, err = getHardwareVersion()
	if err != nil {
		logger.Warning("failed to get HardwareVersion:", err)
	}
	systemInfo.ProjectId, err = getProjectID(customInfoPath)
	if err != nil {
		logger.Warning("failed to get project id:", err)
	}
	systemInfo.Baseline = getCurrentBaseline()
	systemInfo.SystemType = getCurrentSystemType()
	systemInfo.MachineType = getMachineType()
	systemInfo.Mac, err = getDefaultMac()
	if err != nil {
		logger.Warning("failed to get Mac:", err)
	}
	return systemInfo
}

func loadFile(filepath string) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	var lines []string
	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	return lines, nil
}

func GetOSVersionInfo(filePath string) (map[string]string, error) {
	versionLines, err := loadFile(filePath)
	if err != nil {
		logger.Warning("failed to load os-version file:", err)
		return nil, err
	}
	osVersionInfoMap := make(map[string]string)
	for _, item := range versionLines {
		itemSlice := strings.SplitN(item, "=", 2)
		if len(itemSlice) < 2 {
			continue
		}
		key := strings.TrimSpace(itemSlice[0])
		value := strings.TrimSpace(itemSlice[1])
		osVersionInfoMap[key] = value
	}
	// 判断必要内容是否存在
	necessaryKey := []string{"SystemName", "ProductType", "EditionName", "MajorVersion", "MinorVersion", "OsBuild"}
	for _, key := range necessaryKey {
		if value := osVersionInfoMap[key]; value == "" {
			return nil, errors.New("os-version lack necessary content")
		}
	}
	return osVersionInfoMap, nil
}

func GetHardwareId(includeDiskInfo bool) string {
	hhardware.IncludeDiskInfo = includeDiskInfo
	machineID, err := hhardware.GenMachineID()
	if err != nil {
		logger.Warningf("failed to get hardware id: %v ", err.Error())
		return ""
	}
	return machineID
}

func getProcessorModelName() (string, error) {
	processor, err := getProcessorInfo(cpuInfoFilePath)
	if err != nil {
		logger.Warning("Get cpu info failed:", err)
		return "", err
	}
	if processor != "" {
		return processor, nil
	}
	res, err := runLsCpu() // 当 `/proc/cpuinfo` 中无法获取到处理器名称时，通过 `lscpu` 命令来获取
	if err != nil {
		logger.Warning("run lscpu failed:", err)
		return "", nil
	}
	return getCPUInfoFromMap(lscpuKeyModelName, res)
}

func getProcessorInfo(file string) (string, error) {
	data, err := parseInfoFile(file, cpuKeyDelim)
	if err != nil {
		return "", err
	}

	cpu, _ := getCPUInfoFromMap(cpuKeySWCPU, data)
	if len(cpu) != 0 {
		return cpu, nil
	}
	// huawei kirin
	cpu, _ = getCPUInfoFromMap(cpuKeyHWHardware, data)
	if len(cpu) != 0 {
		return cpu, nil
	}
	// arm
	cpu, _ = getCPUInfoFromMap(cpuKeyARMProcessor, data)
	if len(cpu) != 0 {
		return cpu, nil
	}

	return getCPUInfoFromMap(cpuKeyName, data)
}

func getCPUInfoFromMap(nameKey string, data map[string]string) (string, error) {
	value, ok := data[nameKey]
	if !ok {
		return "", fmt.Errorf("can not find the key %q", nameKey)
	}
	var name string
	array := strings.Split(value, " ")
	for i, v := range array {
		if len(v) == 0 {
			continue
		}
		name += v
		if i != len(array)-1 {
			name += " "
		}
	}
	return name, nil
}

func runLsCpu() (map[string]string, error) {
	cmd := exec.Command(lscpuCmd)
	cmd.Env = []string{"LC_ALL=C"}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	res := make(map[string]string, len(lines))
	for _, line := range lines {
		items := strings.Split(line, lscpuKeyDelim)
		if len(items) != 2 {
			continue
		}
		res[strings.TrimSpace(items[0])] = strings.TrimSpace(items[1])
	}

	return res, nil
}

func parseInfoFile(file, delim string) (map[string]string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var ret = make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		array := strings.Split(line, delim)
		if len(array) != 2 {
			continue
		}

		ret[strings.TrimSpace(array[0])] = strings.TrimSpace(array[1])
	}

	return ret, nil
}

func GetArchInfo() (string, error) {
	arch, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		logger.Warningf("GetSystemArchitecture failed:%v\n", arch)
		return "", err
	}
	return strings.TrimSpace(string(arch)), nil
}

// 获取激活码
func getSN() (string, error) {
	logger.Debug("start get SN")
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	object := systemBus.Object("com.deepin.license", "/com/deepin/license/Info")
	var ret dbus.Variant
	err = object.Call("org.freedesktop.DBus.Properties.Get", 0, "com.deepin.license.Info", "ActiveCode").Store(&ret)
	if err != nil {
		return "", err
	}
	v := ret.Value().(string)
	logger.Debug("end get SN")
	return v, nil
}

type oemInfo struct {
	Basic struct {
		IsoId     string `json:"iso_id"`
		TimeStamp uint64 `json:"timestamp"`
	} `json:"basic"`
	CustomInfo struct {
		CustomizedKernel bool `json:"customized_kernel"`
	} `json:"custom_info"`
}

const (
	oemInfoFile = "/etc/oem-info"
	oemSignFile = "/var/uos/.oem-shadow"
	oemPubKey   = "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwzVS35kJl3mhSJssD3S5\nEzjJbFoAD+VsMSy2nS7WQA2XH0aPAWjgCeU+1ScYdBOWz+zWsnK77fGm96HueAuT\nhQEJ9J+ISJUuYBYCc6ovc35gxnhCmP2Qof+/vw98+uKnf1aTDI1imNCWOd/shSbL\nOBn5xFXPsQld1HJqahOuQZOguNIWvrvT7RtmQb77iu576gVLc948HreXKOPD57uK\nJoA2KcoUt95hd94wYyphCuE4onjPcIlpJQfda6PP+HO2Xwze3ltIG6hJSSAEK4R9\n8GnaOTqvslWVI9QFLCIyQ63dbbnASYFTWpDXTlPJsss64vfWOuEjwIyzzQDJNOzN\nFQIDAQAB\n-----END PUBLIC KEY-----"
)

func getCustomInfoAndOemId() (bool, string, error) {
	if !utils2.IsFileExist(oemInfoFile) || !utils2.IsFileExist(oemSignFile) {
		return false, "", errors.New("oemInfoFile or oemSignFile not exist")
	}
	var info oemInfo
	err := system.DecodeJson(oemInfoFile, &info)
	if err != nil {
		return false, "", err
	}

	if !verifyOemFile(oemPubKey, oemInfoFile) {
		logger.Warning("verify oem-info failure")
		return false, "", errors.New("verify oem-info failure")
	}
	return info.CustomInfo.CustomizedKernel, info.Basic.IsoId, nil
}

// 定制标识校验
func verifyOemFile(key, file string) bool {
	// pem解码
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return false
	}
	// 解析得到一个公钥interface
	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logger.Warning(err)
		return false
	}
	// 转为rsa公钥
	publicKey := pubKeyInterface.(*rsa.PublicKey)
	// sha256计算
	hash := sha256.New()
	encContent, err := os.ReadFile(file)
	if err != nil {
		logger.Warning(err)
		return false
	}
	_, err = hash.Write(encContent)
	if err != nil {
		logger.Warning(err)
		return false
	}
	hashed := hash.Sum(nil)

	// 读取签名文件
	srBuf, err := os.ReadFile(oemSignFile)
	if err != nil {
		logger.Warning(err)
		return false
	}
	// 签名认证
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed, srBuf) == nil
}

func getHardwareVersion() (string, error) {
	res, err := exec.Command("dmidecode", "-s", "system-version").Output()
	if err != nil {
		return "", err
	}
	return string(res), nil
}

const oemFilePath = "/etc/.oemid"

func getOEMID() (string, error) {
	content, err := os.ReadFile(oemFilePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

type ProjectInfo struct {
	Id string `json:"id"`
}

func getProjectID(fileName string) (string, error) {
	content, err := os.ReadFile(fileName)
	if err != nil {
		return "", err
	}
	info := new(ProjectInfo)
	err = json.Unmarshal(content, info)
	if err != nil {
		return "", err
	}
	return info.Id, nil
}

func getMachineType() string {
	const dmiDirPrefix = "/sys/class/dmi/id"
	var files = []string{
		"product_family",
		"product_name",
		"product_sku",
	}
	var content []string
	for _, key := range files {
		value, err := utils.ReadFileContent(filepath.Join(dmiDirPrefix, key))
		if err != nil {
			continue
		}
		content = append(content, value)
	}
	return strings.Join(content, " ")
}

var _tokenUpdateMu sync.Mutex

// UpdateTokenConfigFile 更新 99lastore-token.conf 文件的内容
func UpdateTokenConfigFile(includeDiskInfo bool) string {
	logger.Debug("start updateTokenConfigFile")
	_tokenUpdateMu.Lock()
	defer _tokenUpdateMu.Unlock()
	logger.Debug("start getSystemInfo")
	systemInfo := getSystemInfo(includeDiskInfo)
	logger.Debug("end getSystemInfo")
	tokenPath := "/etc/apt/apt.conf.d/99lastore-token.conf"
	var tokenSlice []string
	tokenSlice = append(tokenSlice, "a="+systemInfo.SystemName)
	tokenSlice = append(tokenSlice, "b="+systemInfo.ProductType)
	tokenSlice = append(tokenSlice, "c="+systemInfo.EditionName)
	tokenSlice = append(tokenSlice, "v="+systemInfo.Version)
	tokenSlice = append(tokenSlice, "i="+systemInfo.HardwareId)
	tokenSlice = append(tokenSlice, "m="+systemInfo.Processor)
	tokenSlice = append(tokenSlice, "ac="+systemInfo.Arch)
	tokenSlice = append(tokenSlice, "cu="+systemInfo.Custom)
	tokenSlice = append(tokenSlice, "sn="+systemInfo.SN)
	tokenSlice = append(tokenSlice, "vs="+systemInfo.HardwareVersion)
	tokenSlice = append(tokenSlice, "oid="+systemInfo.OEMID)
	tokenSlice = append(tokenSlice, "pid="+systemInfo.ProjectId)
	tokenSlice = append(tokenSlice, "baseline="+systemInfo.Baseline)
	tokenSlice = append(tokenSlice, "st="+systemInfo.SystemType)
	tokenSlice = append(tokenSlice, "mt="+systemInfo.MachineType)
	tokenSlice = append(tokenSlice, "mac="+systemInfo.Mac)
	token := strings.Join(tokenSlice, ";")
	token = strings.Replace(token, "\n", "", -1)
	tokenContent := []byte("Acquire::SmartMirrors::Token \"" + token + "\";\n")
	err := os.WriteFile(tokenPath, tokenContent, 0644) // #nosec G306
	if err != nil {
		logger.Warning(err)
	}
	return token
}

func getDefaultMac() (string, error) {
	res, err := exec.Command("route").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(res), "\n")
	var dev string
	for _, line := range lines {
		if strings.Contains(line, "default") {
			devs := strings.Split(line, " ")
			dev = devs[len(devs)-1]
			break
		}
	}
	if len(dev) == 0 {
		return "", nil
	}

	mac, err := os.ReadFile("/sys/class/net/" + dev + "/address")
	if err != nil {
		return "", err
	}
	return string(mac), nil
}

func getClientPackageInfo(client string) string {
	o, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f", "${Version}", "--", client).Output()
	if err != nil {
		logger.Warning(err)
		return ""
	}

	return fmt.Sprintf("client=%v&version=%v", client, string(o))
}
