package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/codegangsta/cli"
	"github.com/jouyouyun/hardware/dmi"
	"github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/linuxdeepin/lastore-daemon/src/internal/updateplatform"
)

type Response struct {
	Result bool `json:"result"`
	Code   int  `json:"code"`
	Data   Data `json:"data"`
}

type Data struct {
	Fill   bool `json:"fill"`
	Custom bool `json:"custom"`
}

type DiskInfo struct {
	DiskNo        string `json:"diskNo"`
	TotalCapacity int64  `json:"totalCapacity"`
	FreeCapacity  int64  `json:"freeCapacity"`
}

type PostMemoryInfo struct {
	MemoryNo string `json:"memoryNo"`
	Capacity int64  `json:"capacity"`
}

type SystemInfo struct {
	SN     string           `json:"sn"`
	Disk   []DiskInfo       `json:"disk"`
	Memory []PostMemoryInfo `json:"memory"`
}

var CMDPostHardwareInfo = cli.Command{
	Name:   "posthardware",
	Usage:  `post hardware info`,
	Action: MainPostHardwareInfo,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type,t",
			Value: "",
			Usage: "ui|post",
		},
	},
}

var CMDGatherInfo = cli.Command{
	Name:   "gatherinfo",
	Usage:  `gather user info`,
	Action: MainGatherInfo,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type,t",
			Value: "",
			Usage: "ui|post",
		},
	},
}

type BlockDevice struct {
	Name      string      `json:"name"`
	Removable bool        `json:"rm"`
	Size      int64       `json:"size"`
	Type      string      `json:"type"`
	Fsuse     string      `json:"fsuse%"`
	Children  []Partition `json:"children"`
}

type Partition struct {
	Name      string `json:"name"`
	Removable bool   `json:"rm"`
	Size      int64  `json:"size"`
	Fsuse     string `json:"fsuse%"`
}

type BlockDevices struct {
	Blockdevices []BlockDevice `json:"blockdevices"`
}

var gbToByteNumber int64 = 1024 * 1024 * 1024

func convertToHardwareBytes(totalCapacity int64) int64 {
	if totalCapacity/gbToByteNumber < 256 {
		return 256 * gbToByteNumber
	} else if totalCapacity/gbToByteNumber >= 256 && totalCapacity/gbToByteNumber < 512 {
		return 512 * gbToByteNumber
	} else if totalCapacity/gbToByteNumber >= 512 && totalCapacity/gbToByteNumber < 1024 {
		return 1024 * gbToByteNumber
	} else if totalCapacity/gbToByteNumber >= 1024 && totalCapacity/gbToByteNumber < 2048 {
		return 2048 * gbToByteNumber
	} else if totalCapacity/gbToByteNumber >= 2048 && totalCapacity/gbToByteNumber < 4096 {
		return 4096 * gbToByteNumber
	}
	return totalCapacity
}

func getDiskSize() ([]DiskInfo, error) {
	var diskInfos []DiskInfo
	out, err := exec.Command("lsblk", "-J", "-bno", "NAME,RM,TYPE,SIZE,FSUSE%").Output()
	if err != nil {
		return diskInfos, err
	}
	var blockDevices BlockDevices
	err = json.Unmarshal(out, &blockDevices)
	if err != nil {
		return diskInfos, err
	}
	for _, blockDevice := range blockDevices.Blockdevices {
		// ignore removable disk(such as usb disk)
		if blockDevice.Removable {
			continue
		}
		if blockDevice.Type != "disk" {
			continue
		}

		var info DiskInfo
		info.DiskNo = blockDevice.Name
		info.TotalCapacity = convertToHardwareBytes(blockDevice.Size)
		if len(blockDevice.Children) == 0 {
			if len(blockDevice.Fsuse) == 0 {
				info.FreeCapacity = blockDevice.Size
			} else {
				fsuse := strings.ReplaceAll(blockDevice.Fsuse, "%", "")
				var use int64
				use, err = strconv.ParseInt(fsuse, 10, 64)
				if err != nil {
					info.FreeCapacity = blockDevice.Size
				} else {
					info.FreeCapacity = blockDevice.Size * (100 - use) / 100
				}
			}
		} else {
			var freeCapacity int64
			for _, child := range blockDevice.Children {
				if len(child.Fsuse) == 0 {
					freeCapacity += child.Size
				} else {
					fsuse := strings.ReplaceAll(child.Fsuse, "%", "")
					var use int64
					use, err = strconv.ParseInt(fsuse, 10, 64)
					if err != nil {
						continue
					} else {
						freeCapacity += child.Size * (100 - use) / 100
					}
				}
			}
			if freeCapacity == 0 {
				info.FreeCapacity = blockDevice.Size
			} else {
				info.FreeCapacity = freeCapacity
			}
		}
		diskInfos = append(diskInfos, info)
	}
	return diskInfos, nil
}

func getMemorySizeByDmi() ([]PostMemoryInfo, error) {
	cmd := exec.Command("dmidecode", "-t", "17")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute dmidecode: %w", err)
	}

	return parseMemoryModuleOutput(string(output))
}

func parseMemoryModuleOutput(output string) ([]PostMemoryInfo, error) {
	lines := strings.Split(output, "\n")
	moduleCount := 0

	var result []PostMemoryInfo
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect new memory module
		if strings.HasPrefix(trimmedLine, "Memory Device") {
			moduleCount++
			continue
		}

		// Extract key information
		if strings.HasPrefix(trimmedLine, "Size:") {
			size := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "Size:"))
			sizeValue, err := ParseSizeToBytes(size)
			if err != nil {
				continue
			}
			result = append(result, PostMemoryInfo{
				MemoryNo: fmt.Sprintf("%d", moduleCount),
				Capacity: sizeValue,
			})
		}
	}
	return result, nil
}

func ParseSizeToBytes(sizeStr string) (int64, error) {
	numEndIndex := 0
	for i := 0; i < len(sizeStr); i++ {
		if (sizeStr[i] < '0' || sizeStr[i] > '9') && sizeStr[i] != '.' {
			numEndIndex = i
			break
		}
	}

	// Parse the numeric part
	size, err := strconv.ParseFloat(strings.TrimSpace(sizeStr[:numEndIndex]), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size: %w", err)
	}

	// Convert to bytes based on the unit
	unit := strings.ToUpper(strings.TrimSpace(sizeStr[numEndIndex:]))
	var multiplier int64

	switch unit {
	case "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit: %s", unit)
	}

	// Calculate the number of bytes and return it
	return int64(size * float64(multiplier)), nil
}

func getWhetherGatherInfo(c *config.Config) (*http.Response, error) {
	url := c.PlatformUrl
	policyUrl := url + "/api/v1/terminal/info/check"
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	logger.Infof("%v", policyUrl)
	request, err := http.NewRequest("GET", policyUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", "/api/v1/terminal/info/check", err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateplatform.UpdateTokenConfigFile(c.IncludeDiskInfo, c.GetHardwareIdByHelper))))
	return client.Do(request)
}

func postHardwareInfo(c *config.Config) error {
	// get S/N
	var sn string
	dmi, err := dmi.GetDMI()
	if err != nil {
		logger.Warning("cannot get dmi")
	} else {
		sn = dmi.BoardSerial
	}

	// get disk info
	diskInfos, err := getDiskSize()
	if err != nil {
		return fmt.Errorf("cannot get disk infos: %w", err)
	}
	logger.Infof("disk info :%+v", diskInfos)
	memoryInfos, err := getMemorySizeByDmi()
	if err != nil {
		return fmt.Errorf("cannot get memory infos: %w", err)
	}

	systemInfo := SystemInfo{
		SN:     sn,
		Disk:   diskInfos,
		Memory: memoryInfos,
	}

	jsonSystemInfo, err := json.Marshal(systemInfo)
	if err != nil {
		return fmt.Errorf("marshal system info failed: %w", err)
	}

	url := c.PlatformUrl
	policyUrl := url + "/api/v1/terminal/hardware"
	client := &http.Client{
		Timeout: 4 * time.Second,
	}
	request, err := http.NewRequest("POST", policyUrl, bytes.NewBuffer(jsonSystemInfo))
	if err != nil {
		return fmt.Errorf("%v new request failed: %v ", "/api/v1/terminal/hardware", err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(updateplatform.UpdateTokenConfigFile(c.IncludeDiskInfo, c.GetHardwareIdByHelper))))

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body failed: %w", err)
	} else {
		logger.Infof("post hardware info response: status=%d, body=%s", resp.StatusCode, string(body))
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("post hardware info failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func MainPostHardwareInfo(c *cli.Context) error {
	config := config.NewConfig(path.Join("/var/lib/lastore", "config.json"))
	// run with root
	return postHardwareInfo(config)
}

func MainGatherInfo(c *cli.Context) error {
	config := config.NewConfig(path.Join("/var/lib/lastore", "config.json"))
	response, err := getWhetherGatherInfo(config)
	if err != nil {
		return fmt.Errorf("get whether gather info failed: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body failed: %w", err)
	}
	logger.Infof("gather info body:%v", string(body))
	if response.StatusCode == 200 {
		var result Response
		err = json.Unmarshal(body, &result)
		if err != nil {
			return fmt.Errorf("unmarshal response body failed: %w", err)
		}
		if result.Data.Fill && result.Data.Custom {
			cmd := exec.Command("/usr/bin/dde-gather-info")
			var outBuf bytes.Buffer
			cmd.Stdout = &outBuf
			var errBuf bytes.Buffer
			cmd.Stderr = &errBuf
			err = cmd.Run()
			if err != nil {
				logger.Infof("Error executing command: %s\n", errBuf.String())
				logger.Infof("Output: %s\n", outBuf.String())
				return nil
			}
		}
	}
	return nil
}
