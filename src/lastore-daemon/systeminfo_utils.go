// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

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
	"internal/system"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/godbus/dbus"

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
}

const (
	OemNotCustomState = "0"
	OemCustomState    = "1"
)

func getSystemInfo() SystemInfo {
	systemInfo := SystemInfo{
		Custom: OemNotCustomState,
	}

	osVersionInfoMap, err := getOSVersionInfo(cacheVersion)
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

	systemInfo.HardwareId, err = getHardwareId()
	if err != nil {
		logger.Warning("failed to get hardwareId:", err)
	}

	systemInfo.Processor, err = getProcessorModelName()
	if err != nil {
		logger.Warning("failed to get modelName:", err)
	} else if len(systemInfo.Processor) > 100 {
		systemInfo.Processor = systemInfo.Processor[0:100] // 按照需求,长度超过100时,只取前100个字符
	}

	systemInfo.Arch, err = getArchInfo()
	if err != nil {
		logger.Warning("failed to get Arch:", err)
	}
	systemInfo.SN, err = getSN()
	if err != nil {
		logger.Warning("failed to get SN:", err)
	}
	isCustom, oemId, err := getCustomInfoAndOemId()
	if err != nil {
		systemInfo.Custom = OemNotCustomState
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
	systemInfo.SystemName = getCurrentSystemType()
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

func getOSVersionInfo(filePath string) (map[string]string, error) {
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

func getHardwareId() (string, error) {
	hhardware.IncludeDiskInfo = true
	machineID, err := hhardware.GenMachineID()
	if err != nil {
		return "", err
	}
	return machineID, nil
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
	content, err := ioutil.ReadFile(file)
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

func getArchInfo() (string, error) {
	arch, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		logger.Warningf("GetSystemArchitecture failed:%v\n", arch)
		return "", err
	}
	return string(arch), nil
}

// 获取激活码
func getSN() (string, error) {
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
	if !verifyOemFile() {
		logger.Warning("verify oem-info failure")
		return false, "", nil
	}
	var info oemInfo
	err := system.DecodeJson(oemInfoFile, &info)
	if err != nil {
		return false, "", err
	}
	return info.CustomInfo.CustomizedKernel, info.Basic.IsoId, nil
}

// 定制标识校验
func verifyOemFile() bool {
	// pem解码
	block, _ := pem.Decode([]byte(oemPubKey))
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
	encContent, err := ioutil.ReadFile(oemInfoFile)
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
	srBuf, err := ioutil.ReadFile(oemSignFile)
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
	content, err := ioutil.ReadFile(oemFilePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

type ProjectInfo struct {
	Id string `json:"id"`
}

func getProjectID(fileName string) (string, error) {
	content, err := ioutil.ReadFile(fileName)
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
