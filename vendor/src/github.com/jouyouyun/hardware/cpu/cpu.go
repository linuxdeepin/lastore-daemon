package cpu

import (
	"strconv"

	"github.com/jouyouyun/hardware/utils"
)

const (
	cpuKeyName      = "model name"
	cpuKeyCores     = "cpu cores"
	cpuKeyModel     = "cpu model"     // for sw and loonson
	cpuKeyCPUs      = "cpus detected" // for sw
	cpuKeyProcessor = "processor"     // for loonson and arm
	cpuKeyNameARM   = "Processor"     // for arm

	cpuKeyDelim = ":"
	cpuFilename = "/proc/cpuinfo"
)

var (
	_cpu *CPU
)

// CPU store cpu info
type CPU struct {
	Name       string
	Processors int
}

// NewCPU return cpu name and processor number
func NewCPU() (*CPU, error) {
	if _cpu != nil {
		return _cpu, nil
	}

	cpu, err := newCPU(cpuFilename)
	if err == nil {
		_cpu = cpu
		return cpu, nil
	}
	cpu, err = newCPUForSW(cpuFilename)
	if err == nil {
		_cpu = cpu
		return cpu, nil
	}
	cpu, err = newCPUForLoonson(cpuFilename)
	if err == nil {
		_cpu = cpu
		return cpu, nil
	}
	cpu, err = newCPUForARM(cpuFilename)
	if err == nil {
		_cpu = cpu
		return cpu, nil
	}
	return nil, err
}

func newCPU(filename string) (*CPU, error) {
	return doNewCPU(filename, []string{
		cpuKeyName,
		cpuKeyCores,
	}, false, false)
}

func newCPUForSW(filename string) (*CPU, error) {
	return doNewCPU(filename, []string{
		cpuKeyModel,
		cpuKeyCPUs,
	}, false, false)
}

func newCPUForLoonson(filename string) (*CPU, error) {
	return doNewCPU(filename, []string{
		cpuKeyModel,
		cpuKeyProcessor,
	}, true, true)
}

func newCPUForARM(filename string) (*CPU, error) {
	return doNewCPU(filename, []string{
		cpuKeyNameARM,
		cpuKeyProcessor,
	}, true, true)
}

func doNewCPU(filename string, keys []string, fall, numIncr bool) (*CPU, error) {
	var keySet = make(map[string]string)
	for _, key := range keys {
		keySet[key] = ""
	}
	err := utils.ProcGetByKey(filename, cpuKeyDelim, keySet, fall)
	if err != nil {
		return nil, err
	}
	num, err := strconv.Atoi(keySet[keys[1]])
	if err != nil {
		return nil, err
	}
	if numIncr {
		num++
	}
	return &CPU{
		Name:       keySet[keys[0]],
		Processors: num,
	}, nil
}
