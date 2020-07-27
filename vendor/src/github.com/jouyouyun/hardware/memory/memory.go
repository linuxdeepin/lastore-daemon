package memory

import (
	"strconv"
	"strings"

	"github.com/jouyouyun/hardware/utils"
)

// Memory store memory info
type Memory struct {
	Name         string // TODO(jouyouyun): implement
	Manufacturer string // TODO(jouyouyun): implement
	Capacity     int64  // kb
}

// MemoryList memory list
type MemoryList []*Memory

const (
	memKeyTotal = "MemTotal"
	memKeyDelim = ":"
	memFilename = "/proc/meminfo"
)

var (
	_memList MemoryList
)

func GetMemoryList() (MemoryList, error) {
	if len(_memList) == 0 {
		mem, err := getMemory(memFilename)
		if err != nil {
			return nil, err
		}
		_memList = MemoryList{mem}
	}
	return _memList, nil
}

func getMemory(filename string) (*Memory, error) {
	var keySet = map[string]string{memKeyTotal: ""}
	err := utils.ProcGetByKey(filename, memKeyDelim,
		keySet, false)
	if err != nil {
		return nil, err
	}
	items := strings.SplitN(keySet[memKeyTotal], " ", 2)
	total, err := strconv.ParseInt(items[0], 10, 64)
	if err != nil {
		return nil, err
	}
	return &Memory{Capacity: total}, nil
}
