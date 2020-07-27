package dmi

import (
	"fmt"
	"path/filepath"

	"encoding/json"

	"github.com/jouyouyun/hardware/utils"
)

// DMI store bios, board, product info
type DMI struct {
	BiosVendor     string `json:"bios_vendor"`
	BiosVersion    string `json:"bios_version"`
	BiosDate       string `json:"bios_date"`
	BoardName      string `json:"board_name"`
	BoardSerial    string `json:"board_serial"`
	BoardVendor    string `json:"board_vendor"`
	BoardVersion   string `json:"board_version"`
	ProductName    string `json:"product_name"`
	ProductFamily  string `json:"product_family"`
	ProductSerial  string `json:"product_serial"`
	ProductUUID    string `json:"product_uuid"`
	ProductVersion string `json:"product_version"`
}

const (
	dmiDirPrefix = "/sys/class/dmi/id"
)

var (
	_dmi *DMI
)

// GetDMI return bios, board, product info
func GetDMI() (*DMI, error) {
	if _dmi == nil {
		dmi, err := doGetDMI(dmiDirPrefix)
		if err != nil {
			return nil, err
		}
		_dmi = dmi
	}
	return _dmi, nil
}

func doGetDMI(dir string) (*DMI, error) {
	var files = []string{
		"bios_vendor",
		"bios_version",
		"bios_date",
		"board_name",
		"board_serial",
		"board_version",
		"board_vendor",
		"product_name",
		"product_serial",
		"product_family",
		"product_uuid",
		"product_version",
	}
	var set = make(map[string]string)
	for _, key := range files {
		value, err := utils.ReadFileContent(filepath.Join(dmiDirPrefix, key))
		if err != nil {
			continue
		}
		set[key] = value
	}

	if len(set) == 0 {
		return nil, fmt.Errorf("get dmi failure")
	}

	data, err := json.Marshal(set)
	if err != nil {
		return nil, err
	}

	var dmi DMI
	err = json.Unmarshal(data, &dmi)
	if err != nil {
		return nil, err
	}
	return &dmi, nil
}
