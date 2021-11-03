package main

import (
	"bufio"
	"errors"
	"fmt"
	"internal/system"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
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

type SystemInfo struct {
	SystemName  string
	ProductType string
	EditionName string
	Version     string
	HardwareId  string
	Processor   string
	Arch        string
	Custom      string
	SN          string
}

const (
	OemNotCustomState = "0"
	OemCustomState    = "1"
)

func getSystemInfo() (SystemInfo, error) {
	versionPath := path.Join(etcDir, osVersionFileName)
	versionLines, err := loadFile(versionPath)
	if err != nil {
		logger.Warning("failed to load os-version file:", err)
		return SystemInfo{}, err
	}
	mapOsVersion := make(map[string]string)
	for _, item := range versionLines {
		itemSlice := strings.SplitN(item, "=", 2)
		if len(itemSlice) < 2 {
			continue
		}
		key := strings.TrimSpace(itemSlice[0])
		value := strings.TrimSpace(itemSlice[1])
		mapOsVersion[key] = value
	}
	// 判断必要内容是否存在
	necessaryKey := []string{"SystemName", "ProductType", "EditionName", "MajorVersion", "MinorVersion", "OsBuild"}
	for _, key := range necessaryKey {
		if value := mapOsVersion[key]; value == "" {
			return SystemInfo{}, errors.New("os-version lack necessary content")
		}
	}
	systemInfo := SystemInfo{
		Custom: OemNotCustomState,
	}
	systemInfo.SystemName = mapOsVersion["SystemName"]
	systemInfo.ProductType = mapOsVersion["ProductType"]
	systemInfo.EditionName = mapOsVersion["EditionName"]
	systemInfo.Version = strings.Join([]string{
		mapOsVersion["MajorVersion"],
		mapOsVersion["MinorVersion"],
		mapOsVersion["OsBuild"]},
		".")
	systemInfo.HardwareId, err = getHardwareId()
	if err != nil {
		logger.Warning("failed to get hardwareId:", err)
		return SystemInfo{}, err
	}

	systemInfo.Processor, err = getProcessorModelName()
	if err != nil {
		logger.Warning("failed to get modelName:", err)
		return SystemInfo{}, err
	}
	systemInfo.Arch, err = getArchInfo()
	if err != nil {
		logger.Warning("failed to get Arch:", err)
		return SystemInfo{}, err
	}
	systemInfo.SN, err = getSN()
	if err != nil {
		logger.Warning("failed to get SN:", err)
		return SystemInfo{}, err
	}
	isCustom, err := getCustomInfo()
	if err != nil {
		return SystemInfo{}, err
	}
	if isCustom {
		systemInfo.Custom = OemCustomState
	}

	if len(systemInfo.Processor) > 100 {
		systemInfo.Processor = systemInfo.Processor[0:100] // 按照需求,长度超过100时,只取前100个字符
	}

	return systemInfo, nil
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

func getCustomInfo() (bool, error) {
	var info oemInfo
	err := system.DecodeJson("/etc/oem-info.json", &info)
	if err != nil {
		return false, err
	}
	return info.CustomInfo.CustomizedKernel, nil
}
